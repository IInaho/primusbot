// Package agentrunner adapts the NekoCode agent core to the interaction
// runtime. Applications remain responsible only for constructing the agent
// with their prompt, model, context, policy, and tool registry.
package agentrunner

import (
	"context"

	agentcore "nekocode/bot/agent"
	"nekocode/protocol"
	controlruntime "nekocode/runtime"
)

// Runner exposes one agent as a runtime Runner with streaming, tool events,
// cancellation, metrics, and lifecycle handling.
type Runner struct {
	agent *agentcore.Agent
}

func New(agent *agentcore.Agent) *Runner {
	if agent == nil {
		panic("runtime/agentrunner: nil agent")
	}
	return &Runner{agent: agent}
}

func (r *Runner) Run(ctx context.Context, input string, host controlruntime.RunHost) (string, error) {
	releaseOutput := r.agent.BindOutput(agentcore.Output{
		Text:   host.Text,
		Reason: host.Reason,
		Phase:  host.Phase,
	})
	defer releaseOutput()
	releaseInteraction := r.agent.BindInteraction(agentcore.Interaction{
		Confirm: host.Confirm,
		Ask:     host.Ask,
		Todos:   host.Todos,
	})
	defer releaseInteraction()

	finished := make(chan struct{})
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		select {
		case <-ctx.Done():
			r.agent.Abort()
		case <-finished:
		}
	}()
	defer func() {
		close(finished)
		<-watchDone
	}()

	result := r.agent.Run(input, func(event protocol.StepEvent) {
		PublishStep(host, event)
	})
	return result.FinalOutput, result.Error
}

func (r *Runner) Metrics() controlruntime.MetricsSnapshot {
	return r.agent.Metrics(0)
}

func (r *Runner) Close() error {
	r.agent.Abort()
	return nil
}

// PublishStep converts one agent-core step into the public runtime protocol.
// Product runners with additional lifecycle work can reuse this conversion.
func PublishStep(host controlruntime.RunHost, event protocol.StepEvent) {
	switch event.Action {
	case protocol.StepActionToolStart:
		host.Tool(toolEvent(controlruntime.ToolEventStarted, event))
	case protocol.StepActionToolBlocked:
		host.Tool(toolEvent(controlruntime.ToolEventBlocked, event))
	case protocol.StepActionToolPreview:
		host.Tool(toolEvent(controlruntime.ToolEventPreview, event))
	case protocol.StepActionExecuteTool:
		host.Tool(toolEvent(controlruntime.ToolEventCompleted, event))
	case protocol.StepActionSubAgentStart:
		host.SubAgent(controlruntime.SubAgentEvent{
			Kind:  controlruntime.SubAgentEventStarted,
			ID:    event.SubAgentID,
			Type:  event.SubAgentType,
			Color: event.SubAgentColor,
		})
	case protocol.StepActionSubAgentEnd:
		host.SubAgent(controlruntime.SubAgentEvent{
			Kind: controlruntime.SubAgentEventEnded,
			ID:   event.SubAgentID,
		})
	}
}

func toolEvent(kind controlruntime.ToolEventKind, event protocol.StepEvent) controlruntime.ToolEvent {
	preview := ""
	output := event.Output
	if kind == controlruntime.ToolEventStarted || kind == controlruntime.ToolEventPreview {
		preview = event.Output
		output = ""
	}
	return controlruntime.ToolEvent{
		Kind:          kind,
		CallID:        event.CallID,
		Name:          event.ToolName,
		Args:          event.ToolArgs,
		Output:        output,
		Preview:       preview,
		IsError:       event.IsError,
		SubAgentID:    event.SubAgentID,
		SubAgentColor: event.SubAgentColor,
	}
}

var (
	_ controlruntime.Runner          = (*Runner)(nil)
	_ controlruntime.MetricsProvider = (*Runner)(nil)
)
