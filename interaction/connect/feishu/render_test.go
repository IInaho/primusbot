package feishu

import (
	"strings"
	"testing"

	controlruntime "nekocode/runtime"
)

func TestRenderToolCompleted(t *testing.T) {
	ev := controlruntime.Event{
		Type:    controlruntime.EventToolCompleted,
		Payload: controlruntime.ToolPayload{ToolName: "read", Args: "path=a.go"},
	}
	if got := renderEvent(ev); got != "- read path=a.go" {
		t.Fatalf("render = %q", got)
	}

	ev.Payload = controlruntime.ToolPayload{ToolName: "shell", IsError: true}
	if got := renderEvent(ev); !strings.Contains(got, "(failed)") {
		t.Fatalf("failed tool render = %q", got)
	}
}

func TestRenderApprovalIncludesIDs(t *testing.T) {
	ev := controlruntime.Event{
		Type: controlruntime.EventApprovalRequested,
		Payload: controlruntime.ApprovalView{
			ID:       "ap_1",
			ToolName: "shell",
			Args:     map[string]any{"command": "rm -rf /tmp/x"},
		},
	}
	got := renderEvent(ev)
	if !strings.Contains(got, "/approve ap_1") || !strings.Contains(got, "/reject ap_1") {
		t.Fatalf("approval render missing commands: %q", got)
	}
	if !strings.Contains(got, "rm -rf /tmp/x") {
		t.Fatalf("approval render missing command: %q", got)
	}
}

func TestRenderQuestionListsOptions(t *testing.T) {
	ev := controlruntime.Event{
		Type: controlruntime.EventQuestionRequested,
		Payload: controlruntime.QuestionView{
			ID: "q_1",
			Questions: []controlruntime.QuestionItem{
				{Question: "Continue?"},
			},
		},
	}
	if got := renderEvent(ev); !strings.Contains(got, "Continue?") {
		t.Fatalf("question render = %q", got)
	}
}

func TestRenderIgnoresUnknownEvents(t *testing.T) {
	for _, ev := range []controlruntime.Event{
		{Type: controlruntime.EventRunStarted},
		{Type: controlruntime.EventToolCompleted, Payload: "wrong type"},
	} {
		if got := renderEvent(ev); got != "" {
			t.Fatalf("event %v should render nothing, got %q", ev.Type, got)
		}
	}
}
