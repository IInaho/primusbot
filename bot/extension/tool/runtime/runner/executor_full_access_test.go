package runner

import (
	"context"
	"strings"
	"testing"

	"nekocode/bot/extension/tool/runtime/core"
	"nekocode/bot/extension/tool/runtime/permission"
)

// Full-takeover mode: guarded commands run immediately, no confirm needed.
func TestFullAccessRunsGuardedCommandsWithoutPrompt(t *testing.T) {
	e := newTestExecutor(fakeRegistry{
		"shell": fakeTool{name: "shell", mode: core.ModeSequential},
	})
	e.SetPermissionPolicy(permission.PermissionsDecl{Ask: []string{"shell(rm *)"}}, "/repo", "/home/user")
	// No confirmFn at all: manual mode must fail (prompt impossible), full
	// mode must run.
	call := core.ToolCallItem{ID: "1", Name: "shell", Args: map[string]any{"command": "rm -rf /tmp/x"}}

	e.SetFullAccess(true)
	if got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{call})[0]; got.Error != "" || got.Output != "ok" {
		t.Fatalf("full access should run the guarded command: %+v", got)
	}

	e.SetFullAccess(false)
	if got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{call})[0]; got.Error == "" {
		t.Fatal("manual mode must still prompt (and fail without a confirm function)")
	}
}

// Full-takeover mode bypasses approvals, never explicit deny rules.
func TestFullAccessKeepsDenyRules(t *testing.T) {
	e := newTestExecutor(fakeRegistry{
		"shell": fakeTool{name: "shell", mode: core.ModeSequential},
	})
	e.SetPermissionPolicy(permission.PermissionsDecl{Deny: []string{"shell(rm *)"}}, "/repo", "/home/user")
	e.SetFullAccess(true)

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "shell", Args: map[string]any{"command": "rm -rf /tmp/x"}},
	})[0]
	if got.Error == "" || !strings.Contains(got.Error, "denied by rule") {
		t.Fatalf("deny rule must still block in full-access mode: %+v", got)
	}
}

// Full-takeover mode also bypasses the out-of-workspace prompt: file tools
// get session-scoped access to the requested root without a dialog.
func TestFullAccessGrantsWorkspaceAccessWithoutPrompt(t *testing.T) {
	e := newTestExecutor(fakeRegistry{
		"write": fakeTool{name: "write", mode: core.ModeSequential},
	})
	// No confirmFn: manual mode must fail closed, full mode must proceed.
	call := core.ToolCallItem{ID: "1", Name: "write", Args: map[string]any{"path": "/tmp/nekocode-fa-workspace/x.txt"}}

	e.SetFullAccess(true)
	if got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{call})[0]; got.Error != "" || got.Output != "ok" {
		t.Fatalf("full access should grant workspace access without a prompt: %+v", got)
	}

	e2 := newTestExecutor(fakeRegistry{
		"write": fakeTool{name: "write", mode: core.ModeSequential},
	})
	if got := e2.ExecuteBatch(context.Background(), []core.ToolCallItem{call})[0]; got.Error == "" {
		t.Fatal("manual mode must still gate out-of-workspace writes")
	}
}

// Full-takeover mode escalates privileged tools without any dialog — the
// path manual mode only reaches through a confirm (or a stored grant).
func TestFullAccessEscalatesWithoutDialog(t *testing.T) {
	tool := &permissionTool{fakeTool: fakeTool{name: "shell", mode: core.ModeSequential}}
	e := newTestExecutor(fakeRegistry{"shell": tool})
	e.SetPermissionPolicy(permission.PermissionsDecl{}, "/repo", "/home/user")
	// No confirmFn: manual mode dies at failNoConfirmFn; full mode must
	// retry privileged directly.
	e.SetFullAccess(true)

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "shell", Args: map[string]any{"command": "curl https://example.com"}},
	})[0]
	if got.Error != "" || !strings.Contains(got.Output, "privileged ok") {
		t.Fatalf("full access should escalate without a dialog: %+v", got)
	}
	if tool.privilegedCalls != 1 {
		t.Fatalf("privilegedCalls = %d, want 1", tool.privilegedCalls)
	}
}
