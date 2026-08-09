package connect

import (
	"strings"
	"testing"

	controlruntime "nekocode/runtime"
)

func TestApprovalTextShowsCombinedSecurityDetails(t *testing.T) {
	text := ApprovalText(controlruntime.ApprovalView{
		ID:       "apr_1",
		ToolName: "shell",
		Args: map[string]any{
			"command": `echo "$(date)"`,
		},
		Approval: &controlruntime.ApprovalContext{
			Risk:         "dynamic shell execution",
			Structures:   []string{"command_substitution"},
			Capabilities: []string{"net.outbound"},
			Scope:        controlruntime.ApprovalScopeOnce,
		},
	})
	for _, want := range []string{"命令替换", "出站网络", "仅当前调用"} {
		if !strings.Contains(text, want) {
			t.Fatalf("ApprovalText() missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "/always") {
		t.Fatalf("once-only approval must not advertise /always:\n%s", text)
	}
}
