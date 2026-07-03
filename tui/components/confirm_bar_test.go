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
			"permission_reason":       "command contains dynamic shell syntax that cannot be safely persisted",
			"permission_capabilities": "shell.unknown",
			"permission_scope":        "once",
			"workspace":               "/repo",
			"commandClass":            "unknown",
		},
		Kind:     common.ConfirmKindPermission,
		Response: make(chan common.ConfirmReply, 1),
	})

	view := cb.View(100, 40)
	if strings.Contains(view, "echo") || strings.Contains(view, "pwd") {
		t.Fatalf("permission confirm should not repeat the full command:\n%s", view)
	}
	for _, want := range []string{"需要临时授权", "动态 Shell", "仅本次", "/repo"} {
		if !strings.Contains(view, want) {
			t.Fatalf("permission confirm missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Command class") || strings.Contains(view, "Capabilities:") {
		t.Fatalf("permission confirm should not expose raw backend labels:\n%s", view)
	}
}
