package app

import (
	"context"
	"testing"

	"nekocode/bot/agent/subagent"
	"nekocode/bot/tools/runtime/taskbridge"
	commonview "nekocode/common/view"
)

func TestBuildSubagentRunConfigUnknownAgent(t *testing.T) {
	if _, ok := buildSubagentRunConfig(subagentRunConfigInput{AgentTypeName: "missing"}); ok {
		t.Fatal("expected missing agent")
	}
}

func TestBuildSubagentRunConfigWiresCallbacks(t *testing.T) {
	subagent.RegisterPlugin(subagent.AgentType{Name: "tester"})
	defer subagent.UnregisterPlugin("tester")

	var eventAction string
	ctx := taskbridge.WithTaskCallback(context.Background(), func(ev commonview.StepEvent) {
		eventAction = string(ev.Action) + ":" + ev.ToolName + ":" + ev.ToolArgs + ":" + ev.Output
	})
	var phase string
	cfg, ok := buildSubagentRunConfig(subagentRunConfigInput{
		Context:       ctx,
		AgentTypeName: "tester",
		Prompt:        "do it",
		PhaseFn:       func(p string) { phase = p },
	})
	if !ok {
		t.Fatal("expected config")
	}
	if cfg.Prompt != "do it" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	cfg.OnToolCall(subagent.ToolCallEvent{Action: commonview.StepActionToolStart, ToolName: "read", ToolArgs: "path=a", Output: "ok"})
	if eventAction != "sub_tool_start:read:path=a:ok" {
		t.Fatalf("callback = %q", eventAction)
	}
	cfg.OnPhase("phase")
	if phase != "tester · phase" {
		t.Fatalf("phase = %q", phase)
	}
}

func TestSubagentTaskResultMapsStatus(t *testing.T) {
	tests := []struct {
		status subagent.Status
		want   taskbridge.TaskStatus
	}{
		{subagent.StatusCompleted, taskbridge.TaskStatusCompleted},
		{subagent.StatusFailed, taskbridge.TaskStatusFailed},
		{subagent.StatusPartial, taskbridge.TaskStatusPartial},
	}
	for _, tt := range tests {
		got := subagentTaskResult(&subagent.Result{Status: tt.status, Content: "ok"})
		if got.Status != tt.want || got.Content != "ok" {
			t.Fatalf("subagentTaskResult(%v) = %+v", tt.status, got)
		}
	}
	if subagentTaskResult(nil) != nil {
		t.Fatal("nil result should map to nil")
	}
}
