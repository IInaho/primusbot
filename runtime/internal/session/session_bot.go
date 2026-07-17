package session

import (
	"context"
	"strings"

	"nekocode/runtime/view"
)

func (r *SessionRuntime) configureBot() {
	r.bot.Configure(
		func(req view.ConfirmRequest) view.ConfirmReply {
			r.setStatus(RunWaitingApproval)
			reply := r.approvals.Request(req)
			r.setStatus(RunRunning)
			return reply
		},
		func(phase string) {
			r.events.Publish(Event{
				RunID:   r.CurrentRunID(),
				Type:    EventPhaseChanged,
				Source:  SourceRef{Kind: "bot"},
				Payload: PhasePayload{Phase: phase},
			})
		},
		func(items []view.TodoItem) {
			r.events.Publish(Event{
				RunID:   r.CurrentRunID(),
				Type:    EventTodosUpdated,
				Source:  SourceRef{Kind: "bot"},
				Payload: items,
			})
		},
		func(msg string) {
			r.events.Publish(Event{
				RunID:   r.CurrentRunID(),
				Type:    EventSystemMessage,
				Source:  SourceRef{Kind: "bot"},
				Payload: MessagePayload{Role: "system", Content: msg, Source: SourceRef{Kind: "bot"}},
			})
		},
		nil,
		func(req view.QuestionRequest) view.QuestionReply {
			r.setStatus(RunWaitingQuestion)
			reply := r.questions.Request(req)
			r.setStatus(RunRunning)
			return reply
		},
	)
}

func (r *SessionRuntime) run(ctx context.Context, runID RunID, input Input) {
	r.events.Publish(Event{RunID: runID, Type: EventRunStarted, Source: SourceRef{Kind: "runtime"}})

	if handled := r.handleRuntimeCommand(ctx, runID, input); handled {
		return
	}

	resp, cr := r.bot.ExecuteCommand(input.Text)
	if cr != view.CmdNone {
		if resp != "" {
			r.events.Publish(Event{
				RunID:   runID,
				Type:    EventSystemMessage,
				Source:  SourceRef{Kind: "bot"},
				Payload: MessagePayload{Role: "system", Content: resp, Source: SourceRef{Kind: "bot"}},
			})
		}
		if cr == view.CmdSessionResumed {
			r.events.Publish(Event{
				RunID:  runID,
				Type:   EventSessionResumed,
				Source: SourceRef{Kind: "bot"},
			})
		}
		if hint, wantsAgent := r.bot.SkillHint(); wantsAgent {
			r.events.Publish(Event{
				RunID:   runID,
				Type:    EventSystemMessage,
				Source:  SourceRef{Kind: "runtime"},
				Payload: MessagePayload{Role: "system", Content: "Loaded skill: " + hint, Source: SourceRef{Kind: "runtime"}},
			})
		} else {
			r.finishRun(runID, "", nil)
			return
		}
	}

	result, err := r.bot.Run(input.Text, view.RunCallbacks{
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
		Step: func(action, toolName, toolArgs, output string) {
			r.publishStep(runID, action, toolName, toolArgs, output)
		},
	})
	r.finishRun(runID, result, err)

	select {
	case <-ctx.Done():
	default:
	}
}

func (r *SessionRuntime) handleRuntimeCommand(ctx context.Context, runID RunID, input Input) bool {
	fields := strings.Fields(strings.TrimSpace(input.Text))
	if len(fields) == 0 {
		return false
	}
	name := strings.TrimPrefix(strings.ToLower(fields[0]), "/")
	if name != "connect" && name != "disconnect" && name != "devices" {
		return false
	}
	var resp string
	var err error
	switch name {
	case "connect":
		resp, err = r.connectors.Handle(ctx, fields[1:])
	case "disconnect":
		connName := ""
		if len(fields) > 1 {
			connName = fields[1]
		}
		resp, err = r.connectors.Disconnect(connName)
		if err == nil && connName != "" {
			resp = ""
		}
	case "devices":
		resp = r.connectors.Devices()
	}
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
	if r.Status() == RunAborted {
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

func (r *SessionRuntime) publishStep(runID RunID, action, toolName, toolArgs, output string) {
	typ := EventToolCompleted
	payload := ToolPayload{ToolName: toolName, Args: toolArgs, Output: output}
	switch action {
	case "tool_start", "sub_tool_start":
		typ = EventToolStarted
		payload.Preview = output
	case "tool_blocked":
		typ = EventToolBlocked
	case "tool_preview":
		typ = EventToolPreview
		payload.Preview = output
	case "sub_agent_start":
		typ = EventSubAgentStarted
	case "sub_agent_end":
		typ = EventSubAgentEnded
	case "chat":
		r.events.Publish(Event{
			RunID:   runID,
			Type:    EventAssistantDelta,
			Source:  SourceRef{Kind: "bot"},
			Payload: DeltaPayload{Delta: output},
		})
		return
	case "think":
		return
	}
	r.events.Publish(Event{
		RunID:   runID,
		Type:    typ,
		Source:  SourceRef{Kind: "bot"},
		Payload: payload,
	})
}
