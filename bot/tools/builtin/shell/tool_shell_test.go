package shell

import (
	"context"
	"strings"
	"testing"

	"nekocode/bot/tools/runtime/core"
)

func permReq() core.PermissionRequest {
	return core.PermissionRequest{Capabilities: []string{core.CapNetOutbound}}
}

func TestShellRunCompletesWithinYield(t *testing.T) {
	tool := &ShellTool{}
	out, err := tool.ExecuteWithPermission(context.Background(), map[string]any{
		"command":       "echo shell-done",
		"yield_time_ms": 2000,
	}, permReq())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out, "session_id: 1") || !strings.Contains(out, "status: done") || !strings.Contains(out, "shell-done") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestShellRunReturnsSessionThenWaits(t *testing.T) {
	tool := &ShellTool{}
	out, err := tool.ExecuteWithPermission(context.Background(), map[string]any{
		"command":       "sleep 0.2 && echo shell-later",
		"yield_time_ms": 20,
		"timeout_ms":    2000,
	}, permReq())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out, "session_id: 1") || !strings.Contains(out, "status: running") {
		t.Fatalf("expected running session, got:\n%s", out)
	}

	wait, err := tool.Execute(context.Background(), map[string]any{
		"action":        "wait",
		"session_id":    1,
		"yield_time_ms": 1000,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !strings.Contains(wait, "status: done") || !strings.Contains(wait, "shell-later") {
		t.Fatalf("unexpected wait output:\n%s", wait)
	}
}

func TestShellRunTimesOut(t *testing.T) {
	tool := &ShellTool{}
	out, err := tool.ExecuteWithPermission(context.Background(), map[string]any{
		"command":       "sleep 5",
		"yield_time_ms": 20,
		"timeout_ms":    100,
	}, permReq())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out, "status: running") {
		t.Fatalf("expected running session, got:\n%s", out)
	}

	wait, err := tool.Execute(context.Background(), map[string]any{
		"action":        "wait",
		"session_id":    1,
		"yield_time_ms": 1000,
	})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !strings.Contains(wait, "status: timeout") {
		t.Fatalf("expected timeout status, got:\n%s", wait)
	}
}
