package runner_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"nekocode/bot/tools"
	"nekocode/bot/tools/builtin/shell"
	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/bot/tools/runtime/runner"
	"nekocode/common"
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

	e.SetConfirmFn(func(req common.ConfirmRequest) common.ConfirmReply { return common.Deny() })

	results := e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "writer"},
	})
	if results[0].Error == "" {
		t.Error("expected confirm denial")
	}
}

func TestExecutorBgStartAndLogs(t *testing.T) {
	r := tools.NewRegistry()
	r.Register(&shell.BgTool{})
	e := runner.NewExecutor(r)
	e.SetPermissionPolicy(permission.PermissionsDecl{}, "/repo", "/home/user")
	prompts := 0
	e.SetConfirmFn(func(req common.ConfirmRequest) common.ConfirmReply {
		prompts++
		if req.Kind != common.ConfirmKindPermission {
			t.Fatalf("unexpected confirm kind: %+v", req)
		}
		if req.ToolName != "bash" {
			t.Fatalf("bg command approval should display bash, got %+v", req)
		}
		if req.Args["command"] != "echo executor-bg" {
			t.Fatalf("bg command approval should show actual command, got %+v", req.Args)
		}
		if prompts > 1 {
			t.Fatalf("plain bg command should only need command approval, got prompt %d: %+v", prompts, req)
		}
		if req.CanEscalatePermission {
			t.Fatalf("plain bg command should not expose merged capability escalation: %+v", req)
		}
		return common.ConfirmReply{Allowed: true}
	})

	start := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{
		ID:   "1",
		Name: "bg",
		Args: map[string]any{
			"action":  "start",
			"command": "echo executor-bg",
		},
	}})[0]
	if start.Error != "" {
		t.Fatalf("bg start failed: %+v", start)
	}
	if !strings.Contains(start.Output, "Task 1 started") {
		t.Fatalf("start output missing task id: %q", start.Output)
	}

	time.Sleep(200 * time.Millisecond)
	logs := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{
		ID:   "2",
		Name: "bg",
		Args: map[string]any{
			"action": "logs",
			"id":     1,
		},
	}})[0]
	if logs.Error != "" {
		t.Fatalf("bg logs failed: %+v", logs)
	}
	if !strings.Contains(logs.Output, "executor-bg") {
		t.Fatalf("logs output missing command output: %q", logs.Output)
	}
}

func TestExecutorBgStartAppliesBashDenyRules(t *testing.T) {
	r := tools.NewRegistry()
	r.Register(&shell.BgTool{})
	e := runner.NewExecutor(r)
	e.SetPermissionPolicy(permission.PermissionsDecl{}, "/repo", "/home/user")
	e.SetConfirmFn(func(req common.ConfirmRequest) common.ConfirmReply {
		t.Fatalf("denied bg command should not prompt: %+v", req)
		return common.Deny()
	})

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{
		ID:   "1",
		Name: "bg",
		Args: map[string]any{
			"action":  "start",
			"command": "sudo echo blocked",
		},
	}})[0]
	if got.Error == "" {
		t.Fatalf("expected bg start to be denied")
	}
	if !strings.Contains(got.Error, "denied by rule bash") {
		t.Fatalf("error = %q, want bash deny rule", got.Error)
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
