package session

import (
	"context"
	"fmt"
	"strings"
)

func (r *SessionRuntime) configureBot() {
	r.confirmDone = make(chan struct{})
	confirmCh := make(chan ConfirmRequest, 1)
	go r.confirmChLoop(confirmCh)

	r.runner.ConfigureRuntime(ControlCallbacks{
		Confirm: func(req ConfirmRequest) ConfirmReply {
			runID := r.CurrentRunID()
			r.setStatus(RunWaitingApproval)
			reply := r.approvals.Request(req)
			if r.shouldResumeRun(runID) {
				r.setStatus(RunRunning)
			}
			return reply
		},
		ConfirmCh: confirmCh,
		Phase: func(phase string) {
			r.events.Publish(Event{
				RunID:   r.CurrentRunID(),
				Type:    EventPhaseChanged,
				Source:  SourceRef{Kind: "bot"},
				Payload: PhasePayload{Phase: phase},
			})
		},
		Todo: func(items []TodoItem) {
			r.events.Publish(Event{
				RunID:   r.CurrentRunID(),
				Type:    EventTodosUpdated,
				Source:  SourceRef{Kind: "bot"},
				Payload: items,
			})
		},
		Notify: func(msg string) {
			r.events.Publish(Event{
				RunID:   r.CurrentRunID(),
				Type:    EventSystemMessage,
				Source:  SourceRef{Kind: "bot"},
				Payload: MessagePayload{Role: "system", Content: msg, Source: SourceRef{Kind: "bot"}},
			})
		},
		Question: func(req QuestionRequest) QuestionReply {
			runID := r.CurrentRunID()
			r.setStatus(RunWaitingQuestion)
			reply := r.questions.Request(req)
			if r.shouldResumeRun(runID) {
				r.setStatus(RunRunning)
			}
			return reply
		},
	})
}

func (r *SessionRuntime) confirmChLoop(confirmCh chan ConfirmRequest) {
	for {
		select {
		case <-r.confirmDone:
			return
		case req, ok := <-confirmCh:
			if !ok {
				return
			}
			if req.Response == nil {
				// Unblock signal sent by the bot (e.g. install cancelled).
				// Reject any pending approval so the synchronous Confirm caller returns.
				r.approvals.RejectAll()
				continue
			}
			runID := r.CurrentRunID()
			r.setStatus(RunWaitingApproval)
			reply := r.approvals.Request(req)
			if r.shouldResumeRun(runID) {
				r.setStatus(RunRunning)
			}
			select {
			case req.Response <- reply:
			default:
			}
		}
	}
}

func (r *SessionRuntime) run(ctx context.Context, runID RunID, input Input) {
	defer func() {
		if rec := recover(); rec != nil {
			r.finishRun(runID, "", fmt.Errorf("runtime: run %s panicked: %v", runID, rec))
		}
	}()

	r.events.Publish(Event{RunID: runID, Type: EventRunStarted, Source: SourceRef{Kind: "runtime"}})

	if handled := r.handleRuntimeCommand(ctx, runID, input); handled {
		return
	}

	if shouldRunAgent := r.handleBotCommand(runID, input); !shouldRunAgent {
		return
	}

	result, err := r.runner.Run(input.Text, RunCallbacks{
		Text: func(delta string) {
			r.events.Publish(Event{
				RunID:   runID,
				Type:    EventAssistantDelta,
				Source:  SourceRef{Kind: "bot"},
				Payload: DeltaPayload{Delta: delta},
			})
		},
		Reason: func(delta string) {
			r.events.Publish(Event{
				RunID:   runID,
				Type:    EventReasoningDelta,
				Source:  SourceRef{Kind: "bot"},
				Payload: DeltaPayload{Delta: delta},
			})
		},
		Step: func(ev StepEvent) {
			r.publishStep(runID, ev)
		},
	})
	r.finishRun(runID, result, err)

	select {
	case <-ctx.Done():
	default:
	}
}

func (r *SessionRuntime) handleBotCommand(runID RunID, input Input) bool {
	resp, cr := r.commands.ExecuteCommand(input.Text)
	if cr == CmdNone {
		return true
	}
	if resp != "" {
		r.events.Publish(Event{
			RunID:   runID,
			Type:    EventSystemMessage,
			Source:  SourceRef{Kind: "bot"},
			Payload: MessagePayload{Role: "system", Content: resp, Source: SourceRef{Kind: "bot"}},
		})
	}
	if cr == CmdSessionResumed {
		r.events.Publish(Event{
			RunID:  runID,
			Type:   EventSessionResumed,
			Source: SourceRef{Kind: "bot"},
		})
	}
	if hint, wantsAgent := r.skills.SkillHint(); wantsAgent {
		r.events.Publish(Event{
			RunID:   runID,
			Type:    EventSystemMessage,
			Source:  SourceRef{Kind: "runtime"},
			Payload: MessagePayload{Role: "system", Content: "Loaded skill: " + hint, Source: SourceRef{Kind: "runtime"}},
		})
		return true
	}
	r.finishRun(runID, "", nil)
	return false
}

func (r *SessionRuntime) handleRuntimeCommand(ctx context.Context, runID RunID, input Input) bool {
	fields := strings.Fields(strings.TrimSpace(input.Text))
	if len(fields) == 0 {
		return false
	}
	head := strings.ToLower(fields[0])
	if !strings.HasPrefix(head, "/") {
		return false
	}
	name := strings.TrimPrefix(head, "/")
	handler, ok := r.runtimeCommands[name]
	if !ok {
		return false
	}
	resp, err := handler(ctx, r, fields[1:])
	if err != nil {
		resp = err.Error()
	}
	if strings.TrimSpace(resp) != "" {
		r.events.Publish(Event{
			RunID:   runID,
			Type:    EventSystemMessage,
			Source:  SourceRef{Kind: "runtime"},
			Payload: MessagePayload{Role: "system", Content: resp, Source: SourceRef{Kind: "runtime"}},
		})
	}
	r.finishRun(runID, "", nil)
	return true
}

func (r *SessionRuntime) finishRun(runID RunID, output string, err error) {
	if r.isRunAborted(runID) {
		return
	}
	payload := DonePayload{Output: output}
	typ := EventRunDone
	if err != nil {
		typ = EventRunFailed
		payload.Error = err.Error()
		r.setStatus(RunFailed)
	} else {
		r.setStatus(RunDone)
	}
	r.events.Publish(Event{
		RunID:   runID,
		Type:    typ,
		Source:  SourceRef{Kind: "bot"},
		Payload: payload,
	})
	r.setStatus(RunIdle)
}

func (r *SessionRuntime) publishStep(runID RunID, ev StepEvent) {
	var typ EventType
	payload := ToolPayload{ToolName: ev.ToolName, CallID: ev.CallID, Args: ev.ToolArgs, Output: ev.Output, IsError: ev.IsError}
	switch ev.Action {
	case StepActionToolStart, StepActionSubToolStart:
		typ = EventToolStarted
		payload.Preview = ev.Output
	case StepActionToolBlocked:
		typ = EventToolBlocked
	case StepActionToolPreview:
		typ = EventToolPreview
		payload.Preview = ev.Output
	case StepActionExecuteTool, StepActionSubExecuteTool:
		typ = EventToolCompleted
	case StepActionSubAgentStart:
		typ = EventSubAgentStarted
	case StepActionSubAgentEnd:
		typ = EventSubAgentEnded
	case StepActionChat:
		r.events.Publish(Event{
			RunID:   runID,
			Type:    EventAssistantDelta,
			Source:  SourceRef{Kind: "bot"},
			Payload: DeltaPayload{Delta: ev.Output},
		})
		return
	case StepActionThink:
		return
	default:
		return
	}
	r.events.Publish(Event{
		RunID:   runID,
		Type:    typ,
		Source:  SourceRef{Kind: "bot"},
		Payload: payload,
	})
}
