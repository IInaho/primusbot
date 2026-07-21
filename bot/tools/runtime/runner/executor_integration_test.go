package runner_test

import (
	"context"
	"nekocode/bot/view"
	"strings"
	"testing"
	"time"

	"nekocode/bot/tools"
	"nekocode/bot/tools/builtin/shell"
	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/bot/tools/runtime/runner"
)

type testTool struct{ name string }

func (t *testTool) Name() string                                    { return t.name }
func (t *testTool) Description() string                             { return "test" }
func (t *testTool) Parameters() []core.Parameter                    { return nil }
func (t *testTool) ExecutionMode(map[string]any) core.ExecutionMode { return core.ModeParallel }
func (t *testTool) Execute(context.Context, map[string]any) (string, error) {
	return "ok", nil
}

type writeTool struct{ testTool }

func (t *writeTool) ExecutionMode(map[string]any) core.ExecutionMode { return core.ModeSequential }

func TestExecutorBatch(t *testing.T) {
	r := tools.NewRegistry()
	r.Register(&testTool{name: "read"})
	r.Register(&testTool{name: "safe"})
	r.Register(&writeTool{testTool{name: "writer"}})
	e := runner.NewExecutor(r)

	// Empty batch.
	results := e.ExecuteBatch(context.Background(), nil)
	if len(results) != 0 {
		t.Error("expected empty results")
	}

	// Safe tool runs.
	results = e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "2", Name: "safe"},
	})
	if results[0].Error != "" || results[0].Output != "ok" {
		t.Errorf("unexpected result: %+v", results[0])
	}
}

func TestExecutorBatchPreservesCallOrderAcrossModes(t *testing.T) {
	r := tools.NewRegistry()
	r.Register(&testTool{name: "read"})
	r.Register(&writeTool{testTool{name: "write"}})
	e := runner.NewExecutor(r)

	results := e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "write", Args: map[string]any{"path": "a.go"}},
		{ID: "2", Name: "read", Args: map[string]any{"path": "a.go"}},
		{ID: "3", Name: "write", Args: map[string]any{"path": "b.go"}},
	})

	for i, wantID := range []string{"1", "2", "3"} {
		if results[i].ID != wantID {
			t.Fatalf("result %d has ID %q, want %q; results=%+v", i, results[i].ID, wantID, results)
		}
	}
}

func TestExecutorPlanMode(t *testing.T) {
	r := tools.NewRegistry()
	r.Register(&writeTool{testTool{name: "writer"}})
	e := runner.NewExecutor(r)
	e.SetPlanMode(true)

	results := e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "writer"},
	})
	if results[0].Error == "" {
		t.Error("expected plan mode block")
	}
}

func TestExecutorConfirm(t *testing.T) {
	r := tools.NewRegistry()
	r.Register(&writeTool{testTool{name: "writer"}})
	e := runner.NewExecutor(r)
	e.SetPermissionPolicy(permission.PermissionsDecl{Ask: []string{"writer"}}, "/repo", "/home/user")

	e.SetConfirmFn(func(req view.ConfirmRequest) view.ConfirmReply { return view.Deny() })

	results := e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "writer"},
	})
	if results[0].Error == "" {
		t.Error("expected confirm denial")
	}
}

func TestExecutorShellRunAndPoll(t *testing.T) {
	r := tools.NewRegistry()
	r.Register(&shell.ShellTool{})
	e := runner.NewExecutor(r)
	e.SetPermissionPolicy(permission.PermissionsDecl{}, "/repo", "/home/user")
	prompts := 0
	e.SetConfirmFn(func(req view.ConfirmRequest) view.ConfirmReply {
		prompts++
		if req.Kind != view.ConfirmKindPermission {
			t.Fatalf("unexpected confirm kind: %+v", req)
		}
		if req.ToolName != "shell" {
			t.Fatalf("shell command approval should display shell, got %+v", req)
		}
		if req.Args["command"] != "sleep 0.2 && echo executor-shell" {
			t.Fatalf("shell command approval should show actual command, got %+v", req.Args)
		}
		if prompts > 1 {
			t.Fatalf("plain shell command should only need command approval, got prompt %d: %+v", prompts, req)
		}
		if req.CanEscalatePermission {
			t.Fatalf("plain shell command should not expose merged capability escalation: %+v", req)
		}
		return view.ConfirmReply{Allowed: true}
	})

	start := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{
		ID:   "1",
		Name: "shell",
		Args: map[string]any{
			"action":        "run",
			"command":       "sleep 0.2 && echo executor-shell",
			"yield_time_ms": 20,
			"timeout_ms":    2000,
		},
	}})[0]
	if start.Error != "" {
		t.Fatalf("shell run failed: %+v", start)
	}
	if !strings.Contains(start.Output, "session_id: 1") {
		t.Fatalf("run output missing session id: %q", start.Output)
	}

	// The command needs ~200ms to finish; poll with a bounded retry instead of
	// a fixed sleep so the test stays stable under -race and slow machines.
	var logs core.ToolCallResult
	for range 20 {
		logs = e.ExecuteBatch(context.Background(), []core.ToolCallItem{{
			ID:   "2",
			Name: "shell",
			Args: map[string]any{
				"action":     "poll",
				"session_id": 1,
			},
		}})[0]
		if logs.Error != "" {
			t.Fatalf("shell poll failed: %+v", logs)
		}
		if strings.Contains(logs.Output, "executor-shell") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(logs.Output, "executor-shell") {
		t.Fatalf("logs output missing command output: %q", logs.Output)
	}
}

func TestExecutorShellRunAppliesBashDenyRules(t *testing.T) {
	r := tools.NewRegistry()
	r.Register(&shell.ShellTool{})
	e := runner.NewExecutor(r)
	e.SetPermissionPolicy(permission.PermissionsDecl{}, "/repo", "/home/user")
	e.SetConfirmFn(func(req view.ConfirmRequest) view.ConfirmReply {
		t.Fatalf("denied shell command should not prompt: %+v", req)
		return view.Deny()
	})

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{
		ID:   "1",
		Name: "shell",
		Args: map[string]any{
			"action":  "run",
			"command": "sudo echo blocked",
		},
	}})[0]
	if got.Error == "" {
		t.Fatalf("expected shell run to be denied")
	}
	if !strings.Contains(got.Error, "denied by rule shell") {
		t.Fatalf("error = %q, want shell deny rule", got.Error)
	}
}

func TestExecutorDoesNotOwnReadBeforeWriteGovernance(t *testing.T) {
	r := tools.NewRegistry()
	r.Register(&testTool{name: "read"})
	r.Register(&writeTool{testTool{name: "write"}})
	e := runner.NewExecutor(r)

	// Read-before-write is governed by agent ledger policy, not executor-local
	// state. The executor must not reject a call solely because it lacks local
	// read history, otherwise bash/read evidence recorded in the ledger is ignored.
	results := e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "write", Args: map[string]any{"path": "existing.go"}},
	})
	if results[0].Error != "" {
		t.Errorf("executor should not apply read-before-write policy: %s", results[0].Error)
	}
}
