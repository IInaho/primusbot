package runtime

import (
	"context"
	"fmt"
	"strings"
)

func (r *Manager) configureBackend() {
	r.backend.ConfigureRuntime(ControlCallbacks{
		Confirm: func(req ConfirmRequest) ConfirmReply {
			runID := r.currentRunID()
			if !r.beginWaiting(runID, RunWaitingApproval) {
				return ConfirmReply{Allowed: false}
			}
			reply := r.approvals.Request(req)
			r.resumeRun(runID, RunWaitingApproval)
			return reply
		},
		CommandDone: func() {
			runID := r.currentRunID()
			if runID != "" {
				r.finishRun(runID, "", nil)
			}
		},
		Phase: func(phase string) {
			r.events.Publish(Event{
				RunID:   r.currentRunID(),
				Type:    EventPhaseChanged,
				Source:  SourceRef{Kind: "bot"},
				Payload: PhasePayload{Phase: phase},
			})
		},
		Todo: func(items []TodoItem) {
			r.events.Publish(Event{
				RunID:   r.currentRunID(),
				Type:    EventTodosUpdated,
				Source:  SourceRef{Kind: "bot"},
				Payload: items,
			})
		},
		Notify: func(msg string) {
			r.events.Publish(Event{
				RunID:   r.currentRunID(),
				Type:    EventSystemMessage,
				Source:  SourceRef{Kind: "bot"},
				Payload: MessagePayload{Role: "system", Content: msg, Source: SourceRef{Kind: "bot"}},
			})
		},
		Question: func(req QuestionRequest) QuestionReply {
			runID := r.currentRunID()
			if !r.beginWaiting(runID, RunWaitingQuestion) {
				return QuestionReply{Rejected: true}
			}
			reply := r.questions.Request(req)
			r.resumeRun(runID, RunWaitingQuestion)
			return reply
		},
	})
}

func (r *Manager) run(ctx context.Context, runID RunID, input Input) {
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

	result, err := r.backend.Run(input.Text, RunCallbacks{
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

func (r *Manager) handleBotCommand(runID RunID, input Input) bool {
	resp, cr := r.backend.ExecuteCommand(input.Text)
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
	if hint, wantsAgent := r.backend.SkillHint(); wantsAgent {
		r.events.Publish(Event{
			RunID:   runID,
			Type:    EventSystemMessage,
			Source:  SourceRef{Kind: "runtime"},
			Payload: MessagePayload{Role: "system", Content: "Loaded skill: " + hint, Source: SourceRef{Kind: "runtime"}},
		})
		return true
	}
	if cr == CmdConfirming {
		return false
	}
	r.finishRun(runID, "", nil)
	return false
}

func (r *Manager) handleRuntimeCommand(ctx context.Context, runID RunID, input Input) bool {
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
	resp, err := handler(ctx, fields[1:])
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

func (r *Manager) finishRun(runID RunID, output string, err error) {
	payload := DonePayload{Output: output}
	typ := EventRunDone
	status := RunDone
	if err != nil {
		typ = EventRunFailed
		status = RunFailed
		payload.Error = err.Error()
	}

	r.mu.Lock()
	if r.closed || r.currentRun != runID {
		r.mu.Unlock()
		return
	}
	if _, aborted := r.aborted[runID]; aborted {
		r.mu.Unlock()
		return
	}
	switch r.status {
	case RunRunning, RunWaitingApproval, RunWaitingQuestion:
	default:
		r.mu.Unlock()
		return
	}
	r.status = status
	r.mu.Unlock()

	r.events.Publish(Event{
		RunID:   runID,
		Type:    typ,
		Source:  SourceRef{Kind: "bot"},
		Payload: payload,
	})
	r.mu.Lock()
	if r.currentRun == runID && r.status == status {
		r.status = RunIdle
	}
	r.mu.Unlock()
}

func (r *Manager) publishStep(runID RunID, ev StepEvent) {
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
		// Text callbacks already publish the assistant stream. StepActionChat
		// is the coarse completion mirror of that same text; RunDone carries
		// the canonical fallback for providers that emitted no stream.
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
