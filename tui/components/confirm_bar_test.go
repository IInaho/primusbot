package components

import (
	"strings"
	"testing"

	"nekocode/common"
	"nekocode/tui/styles"
)

func TestPermissionConfirmDoesNotRepeatCommand(t *testing.T) {
	sty := styles.DefaultStyles()
	cb := NewConfirmBar(&sty)
	cb.SetRequest(&common.ConfirmRequest{
		ToolName: "bash",
		Args: map[string]any{
			"command":                 `echo "喵~ bash 命令测试成功！当前工作目录: $(pwd)" && date`,
			"permission_reason":       "command requests unsandboxed host execution",
			"permission_capabilities": "process.host",
			"permission_scope":        "once",
			"workspace":               "/repo",
		},
		Kind:     common.ConfirmKindPermission,
		Response: make(chan common.ConfirmReply, 1),
	})

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

func TestPermissionConfirmCanPreApproveEscalationOnce(t *testing.T) {
	sty := styles.DefaultStyles()
	cb := NewConfirmBar(&sty)
	ch := make(chan common.ConfirmReply, 1)
	cb.SetRequest(&common.ConfirmRequest{
		ToolName:              "bash",
		Args:                  map[string]any{"command": "go get example.com/pkg"},
		Kind:                  common.ConfirmKindPermission,
		Response:              ch,
		CanEscalatePermission: true,
	})

	view := cb.View(100, 40)
	if !strings.Contains(view, "仅本次允许并授权") {
		t.Fatalf("confirm view should expose one-time pre-approval:\n%s", view)
	}

	cb.Submit()
	reply := <-ch
	if !reply.Allowed || reply.Remember || !reply.AllowWithPermission {
		t.Fatalf("unexpected reply: %+v", reply)
	}
}

func TestPermissionConfirmCanPreApproveEscalationRemembered(t *testing.T) {
	sty := styles.DefaultStyles()
	cb := NewConfirmBar(&sty)
	ch := make(chan common.ConfirmReply, 1)
	cb.SetRequest(&common.ConfirmRequest{
		ToolName: "bash",
		Args: map[string]any{
			"command":          "go get example.com/pkg",
			"permission_scope": "project",
		},
		Kind:                  common.ConfirmKindPermission,
		Response:              ch,
		CanEscalatePermission: true,
	})

	view := cb.View(100, 40)
	if !strings.Contains(view, "始终允许并授权") {
		t.Fatalf("confirm view should expose remembered pre-approval:\n%s", view)
	}

	cb.Move(1)
	cb.Submit()
	reply := <-ch
	if !reply.Allowed || !reply.Remember || !reply.AllowWithPermission {
		t.Fatalf("unexpected reply: %+v", reply)
	}
}
