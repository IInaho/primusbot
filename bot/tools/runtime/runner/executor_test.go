package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/common"
)

type fakeRegistry map[string]core.Tool

func (r fakeRegistry) Get(name string) (core.Tool, error) {
	if t, ok := r[name]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

type fakeTool struct {
	name   string
	mode   core.ExecutionMode
	output string
}

func (t fakeTool) Name() string                                    { return t.name }
func (t fakeTool) Description() string                             { return "test" }
func (t fakeTool) Parameters() []core.Parameter                    { return nil }
func (t fakeTool) ExecutionMode(map[string]any) core.ExecutionMode { return t.mode }
func (t fakeTool) Execute(context.Context, map[string]any) (string, error) {
	if t.output != "" {
		return t.output, nil
	}
	return "ok", nil
}

func TestExecutorBatchPreservesCallOrderAcrossModes(t *testing.T) {
	e := NewExecutor(fakeRegistry{
		"read":  fakeTool{name: "read", mode: core.ModeParallel},
		"write": fakeTool{name: "write", mode: core.ModeSequential},
	})

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

func TestExecutorBlocksPermissionDenyAndPlanMode(t *testing.T) {
	e := NewExecutor(fakeRegistry{
		"blocked": fakeTool{name: "blocked", mode: core.ModeParallel},
		"writer":  fakeTool{name: "writer", mode: core.ModeSequential},
	})
	e.SetPermissionPolicy(permission.PermissionsDecl{Deny: []string{"blocked"}}, "/repo", "/home/user")

	if got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{ID: "1", Name: "blocked"}})[0]; got.Error == "" {
		t.Fatal("expected permission deny error")
	}

	e.SetPlanMode(true)
	if got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{ID: "2", Name: "writer"}})[0]; got.Error == "" {
		t.Fatal("expected plan mode error")
	}
}

func TestExecutorConfirmDenial(t *testing.T) {
	e := NewExecutor(fakeRegistry{
		"writer": fakeTool{name: "writer", mode: core.ModeSequential},
	})
	e.SetPermissionPolicy(permission.PermissionsDecl{Ask: []string{"writer"}}, "/repo", "/home/user")
	e.SetConfirmFn(func(common.ConfirmRequest) common.ConfirmReply { return common.Deny() })

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{ID: "1", Name: "writer"}})[0]
	if got.Error == "" {
		t.Fatal("expected confirm denial")
	}
}

func TestTruncateOutput(t *testing.T) {
	var output string
	for i := range maxLines + 5 {
		output += fmt.Sprintf("line %d\n", i)
	}
	got := truncateOutput(output)
	if len(got) >= len(output) {
		t.Fatal("expected truncated output")
	}
	if !strings.Contains(got, "truncated") {
		t.Fatalf("missing truncation marker: %q", got)
	}
	for _, want := range []string{"line 0", "line 49", "line 1955", "line 2004"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing retained line %q in %q", want, got)
		}
	}
	for _, notWant := range []string{"line 50\n", "line 1954\n"} {
		if strings.Contains(got, notWant) {
			t.Fatalf("middle line %q should be truncated from %q", notWant, got)
		}
	}
}

func TestExecutorPreservesTaskOutput(t *testing.T) {
	var output string
	for i := range maxLines + 5 {
		output += fmt.Sprintf("line %d\n", i)
	}
	e := NewExecutor(fakeRegistry{
		"task": fakeTool{name: "task", mode: core.ModeParallel, output: output},
	})

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{ID: "1", Name: "task"}})[0]

	if got.Output != output {
		t.Fatal("task output should not be truncated")
	}
}

type permissionTool struct {
	fakeTool
	privilegedCalls int
	wrapError       bool
	req             core.PermissionRequest
}

func (t *permissionTool) Execute(context.Context, map[string]any) (string, error) {
	req := t.req
	if len(req.Capabilities) == 0 {
		req = core.PermissionRequest{
			Reason:       "needs network",
			Capabilities: []string{core.CapNetOutbound},
			Details: map[string]any{
				"workspace": "/repo",
			},
		}
	}
	err := testPermissionError{req: req}
	if t.wrapError {
		return "", fmt.Errorf("sandbox rejected command: %w", err)
	}
	return "", err
}

func (t *permissionTool) ExecuteWithPermission(context.Context, map[string]any, core.PermissionRequest) (string, error) {
	t.privilegedCalls++
	return "privileged ok", nil
}

type testPermissionError struct {
	req core.PermissionRequest
}

func (e testPermissionError) Error() string { return e.req.Reason }
func (e testPermissionError) PermissionRequest() core.PermissionRequest {
	return e.req
}

func TestExecutorUsesPersistedGrant(t *testing.T) {
	tool := &permissionTool{fakeTool: fakeTool{name: "bash", mode: core.ModeSequential}}
	store := permission.NewStore(filepath.Join(t.TempDir(), "permissions.json"))
	req := core.PermissionRequest{
		Capabilities: []string{core.CapNetOutbound},
		Details: map[string]any{
			"workspace": "/repo",
		},
	}
	if err := store.Allow("bash", req); err != nil {
		t.Fatalf("AllowProject: %v", err)
	}
	e := NewExecutor(fakeRegistry{"bash": tool})
	e.SetPermissionStore(store)
	e.SetPermissionPolicy(permission.PermissionsDecl{Allow: []string{"bash"}}, "/repo", "/home/user")
	e.SetConfirmFn(func(common.ConfirmRequest) common.ConfirmReply {
		t.Fatal("confirm should not be called when a persisted grant matches")
		return common.Deny()
	})

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{ID: "1", Name: "bash"}})[0]

	if got.Error != "" || got.Output != "privileged ok" {
		t.Fatalf("unexpected result: %+v", got)
	}
	if tool.privilegedCalls != 1 {
		t.Fatalf("privilegedCalls = %d, want 1", tool.privilegedCalls)
	}
}

func TestExecutorPermissionAllowOnceDoesNotPersistGrant(t *testing.T) {
	tool := &permissionTool{fakeTool: fakeTool{name: "bash", mode: core.ModeSequential}}
	store := permission.NewStore(filepath.Join(t.TempDir(), "permissions.json"))
	e := NewExecutor(fakeRegistry{"bash": tool})
	e.SetPermissionStore(store)
	e.SetPermissionPolicy(permission.PermissionsDecl{Allow: []string{"bash"}}, "/repo", "/home/user")
	e.SetConfirmFn(func(common.ConfirmRequest) common.ConfirmReply {
		return common.AllowOnce()
	})

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{ID: "1", Name: "bash"}})[0]
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}

	req := core.PermissionRequest{
		Capabilities: []string{core.CapNetOutbound},
		Details: map[string]any{
			"workspace": "/repo",
		},
	}
	if _, ok := store.Match("bash", req); ok {
		t.Fatal("allow-once should not persist grant")
	}
}

func TestExecutorPreApprovedEscalationOnceDoesNotPersistGrant(t *testing.T) {
	tool := &permissionTool{fakeTool: fakeTool{name: "bash", mode: core.ModeSequential}}
	store := permission.NewStore(filepath.Join(t.TempDir(), "permissions.json"))
	e := NewExecutor(fakeRegistry{"bash": tool})
	e.SetPermissionStore(store)
	e.SetPermissionPolicy(permission.PermissionsDecl{Ask: []string{"bash"}}, "/repo", "/home/user")
	e.SetConfirmFn(func(common.ConfirmRequest) common.ConfirmReply {
		return common.ConfirmReply{Allowed: true, AllowWithPermission: true}
	})

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{ID: "1", Name: "bash"}})[0]
	if got.Error != "" || got.Output != "privileged ok" {
		t.Fatalf("unexpected result: %+v", got)
	}

	req := core.PermissionRequest{
		Capabilities: []string{core.CapNetOutbound},
		Details: map[string]any{
			"workspace": "/repo",
		},
	}
	if _, ok := store.Match("bash", req); ok {
		t.Fatal("one-time pre-approved escalation should not persist grant")
	}
}

func TestExecutorPreApprovedEscalationRememberPersistsGrant(t *testing.T) {
	tool := &permissionTool{fakeTool: fakeTool{name: "bash", mode: core.ModeSequential}}
	store := permission.NewStore(filepath.Join(t.TempDir(), "permissions.json"))
	e := NewExecutor(fakeRegistry{"bash": tool})
	e.SetPermissionStore(store)
	e.SetPermissionPolicy(permission.PermissionsDecl{Ask: []string{"bash"}}, "/repo", "/home/user")
	e.SetConfirmFn(func(common.ConfirmRequest) common.ConfirmReply {
		return common.ConfirmReply{Allowed: true, Remember: true, AllowWithPermission: true}
	})

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{ID: "1", Name: "bash"}})[0]
	if got.Error != "" || got.Output != "privileged ok" {
		t.Fatalf("unexpected result: %+v", got)
	}

	req := core.PermissionRequest{
		Capabilities: []string{core.CapNetOutbound},
		Details: map[string]any{
			"workspace": "/repo",
		},
	}
	if _, ok := store.Match("bash", req); !ok {
		t.Fatal("remembered pre-approved escalation should persist grant")
	}
}

func TestExecutorRecognizesWrappedPermissionError(t *testing.T) {
	tool := &permissionTool{
		fakeTool:  fakeTool{name: "bash", mode: core.ModeSequential},
		wrapError: true,
	}
	e := NewExecutor(fakeRegistry{"bash": tool})
	e.SetPermissionPolicy(permission.PermissionsDecl{Allow: []string{"bash"}}, "/repo", "/home/user")
	e.SetConfirmFn(func(common.ConfirmRequest) common.ConfirmReply {
		return common.AllowOnce()
	})

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{ID: "1", Name: "bash"}})[0]
	if got.Error != "" || got.Output != "privileged ok" {
		t.Fatalf("unexpected result: %+v", got)
	}
	if tool.privilegedCalls != 1 {
		t.Fatalf("privilegedCalls = %d, want 1", tool.privilegedCalls)
	}
}

func TestExecutorPermissionRememberPersistsGrant(t *testing.T) {
	tool := &permissionTool{fakeTool: fakeTool{name: "bash", mode: core.ModeSequential}}
	store := permission.NewStore(filepath.Join(t.TempDir(), "permissions.json"))
	e := NewExecutor(fakeRegistry{"bash": tool})
	e.SetPermissionStore(store)
	e.SetPermissionPolicy(permission.PermissionsDecl{Allow: []string{"bash"}}, "/repo", "/home/user")
	e.SetConfirmFn(func(common.ConfirmRequest) common.ConfirmReply {
		return common.AllowRemembered()
	})

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{ID: "1", Name: "bash"}})[0]
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}

	req := core.PermissionRequest{
		Capabilities: []string{core.CapNetOutbound},
		Details: map[string]any{
			"workspace": "/repo",
		},
	}
	if _, ok := store.Match("bash", req); !ok {
		t.Fatal("remembered permission should persist grant")
	}
}

func TestExecutorPermissionErrorTellsBashToDeclareCapabilities(t *testing.T) {
	tool := &permissionTool{
		fakeTool: fakeTool{name: "bash", mode: core.ModeSequential},
		req: core.PermissionRequest{
			Reason:       "needs host",
			Capabilities: []string{core.CapProcessHost},
		},
	}
	e := NewExecutor(fakeRegistry{"bash": tool})
	e.SetPermissionPolicy(permission.PermissionsDecl{Allow: []string{"bash"}}, "/repo", "/home/user")

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{ID: "1", Name: "bash", Args: map[string]any{"command": "npm run dev"}}})[0]

	if !strings.Contains(got.Error, `retry the bash call with capabilities ["process.host"]`) {
		t.Fatalf("error = %q", got.Error)
	}
}

func TestExecutorPermissionErrorExplainsMissingApprovalForDeclaredCapabilities(t *testing.T) {
	tool := &permissionTool{
		fakeTool: fakeTool{name: "bash", mode: core.ModeSequential},
		req: core.PermissionRequest{
			Reason:       "needs host",
			Capabilities: []string{core.CapProcessHost},
		},
	}
	e := NewExecutor(fakeRegistry{"bash": tool})
	e.SetPermissionPolicy(permission.PermissionsDecl{Allow: []string{"bash"}}, "/repo", "/home/user")

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{
		ID:   "1",
		Name: "bash",
		Args: map[string]any{
			"command":      "npm run dev",
			"capabilities": []any{core.CapProcessHost},
		},
	}})[0]

	if !strings.Contains(got.Error, "already requested capabilities") || !strings.Contains(got.Error, "no approval was granted") {
		t.Fatalf("error = %q", got.Error)
	}
}
