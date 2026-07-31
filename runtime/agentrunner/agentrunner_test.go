package agentrunner

import (
	"testing"

	"nekocode/protocol"
	controlruntime "nekocode/runtime"
)

type captureHost struct {
	tools     []controlruntime.ToolEvent
	subagents []controlruntime.SubAgentEvent
}

func (*captureHost) Text(string)                     {}
func (*captureHost) Reason(string)                   {}
func (*captureHost) Phase(string)                    {}
func (*captureHost) Todos([]controlruntime.TodoItem) {}
func (*captureHost) Confirm(controlruntime.ConfirmRequest) controlruntime.ConfirmReply {
	return controlruntime.ConfirmReply{}
}
func (*captureHost) Ask(controlruntime.QuestionRequest) controlruntime.QuestionReply {
	return controlruntime.QuestionReply{}
}
func (h *captureHost) Tool(event controlruntime.ToolEvent) {
	h.tools = append(h.tools, event)
}
func (h *captureHost) SubAgent(event controlruntime.SubAgentEvent) {
	h.subagents = append(h.subagents, event)
}

func TestPublishStepMapsAgentEvents(t *testing.T) {
	host := &captureHost{}
	PublishStep(host, protocol.StepEvent{
		Action: protocol.StepActionToolStart, CallID: "call_1",
		ToolName: "web_fetch", ToolArgs: `{"url":"example.com"}`, Output: "preview",
	})
	PublishStep(host, protocol.StepEvent{
		Action: protocol.StepActionExecuteTool, CallID: "call_1",
		ToolName: "web_fetch", Output: "result",
	})
	PublishStep(host, protocol.StepEvent{
		Action:     protocol.StepActionSubAgentStart,
		SubAgentID: "sub_1", SubAgentType: "researcher", SubAgentColor: 2,
	})

	if len(host.tools) != 2 {
		t.Fatalf("tool events = %d, want 2", len(host.tools))
	}
	if host.tools[0].Kind != controlruntime.ToolEventStarted ||
		host.tools[0].Preview != "preview" || host.tools[0].Output != "" {
		t.Fatalf("started tool event = %#v", host.tools[0])
	}
	if host.tools[1].Kind != controlruntime.ToolEventCompleted ||
		host.tools[1].Output != "result" {
		t.Fatalf("completed tool event = %#v", host.tools[1])
	}
	if len(host.subagents) != 1 ||
		host.subagents[0].Kind != controlruntime.SubAgentEventStarted ||
		host.subagents[0].ID != "sub_1" {
		t.Fatalf("subagent events = %#v", host.subagents)
	}
}
