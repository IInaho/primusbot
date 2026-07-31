package components

import (
	"strings"
	"testing"

	"nekocode/interaction/tui/styles"
	controlruntime "nekocode/runtime"
)

func TestPermissionConfirmDoesNotRepeatCommand(t *testing.T) {
	sty := styles.DefaultStyles()
	cb := NewConfirmBar(&sty)
	cb.SetRequest(&controlruntime.ConfirmRequest{
		ToolName: "shell",
		Args: map[string]any{
			"command":                 `echo "喵~ bash 命令测试成功！当前工作目录: $(pwd)" && date`,
			"permission_reason":       "command requests unsandboxed host execution",
			"permission_capabilities": "process.host",
			"permission_scope":        "once",
			"workspace":               "/repo",
		},
		Kind: controlruntime.ConfirmKindPermission,
	}, nil)

	view := cb.View(100, 40)
	if strings.Contains(view, "echo") || strings.Contains(view, "pwd") {
		t.Fatalf("permission confirm should not repeat the full command:\n%s", view)
	}
	for _, want := range []string{"需要临时授权", "主机执行", "仅本次", "/repo"} {
		if !strings.Contains(view, want) {
			t.Fatalf("permission confirm missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Command class") || strings.Contains(view, "Capabilities:") {
		t.Fatalf("permission confirm should not expose raw backend labels:\n%s", view)
	}
}

func TestWorkspacePermissionConfirmShowsPath(t *testing.T) {
	sty := styles.DefaultStyles()
	cb := NewConfirmBar(&sty)
	cb.SetRequest(&controlruntime.ConfirmRequest{
		ToolName: "workspace",
		Args: map[string]any{
			"path":              "/repo/other",
			"access":            "read-only",
			"requested_path":    "/repo/other/src/a.go",
			"permission_reason": "add read-only workspace for read",
		},
		Kind: controlruntime.ConfirmKindPermission,
	}, nil)

	view := cb.View(100, 40)
	for _, want := range []string{"/repo/other", "/repo/other/src/a.go"} {
		if !strings.Contains(view, want) {
			t.Fatalf("workspace confirm missing %q:\n%s", want, view)
		}
	}
}

func TestPermissionConfirmDefaultsToAllowWithoutPreApproval(t *testing.T) {
	sty := styles.DefaultStyles()
	cb := NewConfirmBar(&sty)
	ch := make(chan controlruntime.ConfirmReply, 1)
	cb.SetRequest(&controlruntime.ConfirmRequest{
		ToolName:              "shell",
		Args:                  map[string]any{"command": "go get example.com/pkg", "permission_scope": "once"},
		Kind:                  controlruntime.ConfirmKindPermission,
		CanEscalatePermission: true,
	}, func(ok, remember bool) {
		ch <- controlruntime.ConfirmReply{Allowed: ok, Remember: ok && remember}
	})

	view := cb.View(100, 40)
	// Capability escalation MUST NOT be merged into the approval button. The
	// first dialog only approves running the call as-is; the escalation flow
	// (tryPermissionEscalation) issues a second dialog that names the actual
	// capabilities. Verify no "并授权" surface leaks here.
	if strings.Contains(view, "并授权") {
		t.Fatalf("confirm view must not expose merged allow+escalate button:\n%s", view)
	}
	if !strings.Contains(view, "仅本次允许") {
		t.Fatalf("confirm view should expose plain one-time approval:\n%s", view)
	}

	cb.Submit()
	reply := <-ch
	if !reply.Allowed || reply.Remember {
		t.Fatalf("unexpected reply: %+v", reply)
	}
	if reply.AllowWithPermission {
		t.Fatalf("first dialog must not pre-approve escalation, got AllowWithPermission=true")
	}
}

func TestPermissionConfirmRemembersWithoutPreApproval(t *testing.T) {
	sty := styles.DefaultStyles()
	cb := NewConfirmBar(&sty)
	ch := make(chan controlruntime.ConfirmReply, 1)
	cb.SetRequest(&controlruntime.ConfirmRequest{
		ToolName: "shell",
		Args: map[string]any{
			"command":          "go get example.com/pkg",
			"permission_scope": "project",
		},
		Kind:                  controlruntime.ConfirmKindPermission,
		CanEscalatePermission: true,
	}, func(ok, remember bool) {
		ch <- controlruntime.ConfirmReply{Allowed: ok, Remember: ok && remember}
	})

	view := cb.View(100, 40)
	if strings.Contains(view, "并授权") {
		t.Fatalf("confirm view must not expose merged allow+escalate button:\n%s", view)
	}
	if !strings.Contains(view, "始终允许") {
		t.Fatalf("confirm view should expose remember option for project scope:\n%s", view)
	}

	// Option order: 0=仅本次允许, 1=始终允许, 2=拒绝.
	cb.Move(1)
	cb.Submit()
	reply := <-ch
	if !reply.Allowed || !reply.Remember {
		t.Fatalf("unexpected reply: %+v", reply)
	}
	if reply.AllowWithPermission {
		t.Fatalf("remember must not pre-approve escalation, got AllowWithPermission=true")
	}
}
