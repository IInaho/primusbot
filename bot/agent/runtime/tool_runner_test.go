package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nekocode/bot/policy"
	"nekocode/bot/policy/budget"
	"nekocode/bot/policy/ledger"
	"nekocode/bot/tools/runtime/core"
)

func TestFilterToolCallsAppliesPreToolPolicyBlock(t *testing.T) {
	a := newTestAgent()
	a.deps.gov.HookReg.Register(policy.Hook{
		Name:  "block-read",
		Point: policy.PreToolUse,
		On: func(s policy.State) *policy.Result {
			return &policy.Result{BlockTool: &policy.BlockTool{
				Tool:   "read",
				Reason: "read blocked",
			}}
		},
	})

	filtered := a.toolRunner.filterToolCalls([]core.ToolCallItem{
		{Name: "read", Args: map[string]any{"path": "x.go"}},
	}, &budget.ToolQuota{MaxSlots: 8})

	if len(filtered.Allowed) != 0 {
		t.Fatalf("allowed = %d, want 0", len(filtered.Allowed))
	}
	if got := filtered.Blocked[0]; got != "read blocked" {
		t.Fatalf("blocked reason = %q, want read blocked", got)
	}
}

func TestFilterToolCallsReadBeforeWriteBlockComesFromHook(t *testing.T) {
	a := newTestAgent()
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	filtered := a.toolRunner.filterToolCalls([]core.ToolCallItem{
		{Name: "write", Args: map[string]any{"path": path}},
	}, &budget.ToolQuota{MaxSlots: 8})

	if len(filtered.Allowed) != 0 {
		t.Fatalf("allowed = %d, want 0", len(filtered.Allowed))
	}
	if got := filtered.Blocked[0]; !strings.Contains(got, "ledger 中没有该文件的读取记录") {
		t.Fatalf("blocked reason = %q, want read-before-write hook reason", got)
	}
}

func TestFilterToolCallsAllowsWriteAfterRead(t *testing.T) {
	a := newTestAgent()
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.deps.gov.RecordToolCall(ledger.ToolEvent{
		Name: "read",
		Args: map[string]any{"path": path},
	})

	filtered := a.toolRunner.filterToolCalls([]core.ToolCallItem{
		{Name: "write", Args: map[string]any{"path": path}},
	}, &budget.ToolQuota{MaxSlots: 8})

	if len(filtered.Allowed) != 1 {
		t.Fatalf("allowed = %d, want write allowed after read; blocked=%v", len(filtered.Allowed), filtered.Blocked)
	}
}

func TestFilterToolCallsAllowsEditWithSufficientAnchor(t *testing.T) {
	a := newTestAgent()
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {\n\tmessage := \"hello\"\n\tprintln(message)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	filtered := a.toolRunner.filterToolCalls([]core.ToolCallItem{
		{Name: "edit", Args: map[string]any{
			"path": path,
			"oldString": strings.Join([]string{
				"package main",
				"",
				"func main() {",
				"\tmessage := \"hello\"",
				"\tprintln(message)",
				"}",
			}, "\n"),
			"newString": "package main\n",
		}},
	}, &budget.ToolQuota{MaxSlots: 8})

	if len(filtered.Allowed) != 1 {
		t.Fatalf("allowed = %d, want sufficiently anchored edit allowed; blocked=%v", len(filtered.Allowed), filtered.Blocked)
	}
}

func TestFilterToolCallsBlocksEditWithShortAnchor(t *testing.T) {
	a := newTestAgent()
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	filtered := a.toolRunner.filterToolCalls([]core.ToolCallItem{
		{Name: "edit", Args: map[string]any{
			"path":      path,
			"oldString": "main",
			"newString": "app",
		}},
	}, &budget.ToolQuota{MaxSlots: 8})

	if len(filtered.Allowed) != 0 {
		t.Fatalf("allowed = %d, want short unread edit blocked", len(filtered.Allowed))
	}
	if got := filtered.Blocked[0]; !strings.Contains(got, "ledger 中没有该文件的读取记录") {
		t.Fatalf("blocked reason = %q, want read-before-write hook reason", got)
	}
}

func TestFilterToolCallsAllowsWriteToNewFile(t *testing.T) {
	a := newTestAgent()
	path := filepath.Join(t.TempDir(), "new.go")

	filtered := a.toolRunner.filterToolCalls([]core.ToolCallItem{
		{Name: "write", Args: map[string]any{"path": path}},
	}, &budget.ToolQuota{MaxSlots: 8})

	if len(filtered.Allowed) != 1 {
		t.Fatalf("allowed = %d, want new file write allowed; blocked=%v", len(filtered.Allowed), filtered.Blocked)
	}
}

func TestFilterToolCallsAllowsEditAfterSuccessfulWrite(t *testing.T) {
	a := newTestAgent()
	path := filepath.Join(t.TempDir(), "main.go")
	a.deps.gov.RecordToolCall(ledger.ToolEvent{
		Name: "write",
		Args: map[string]any{"path": path},
	})

	filtered := a.toolRunner.filterToolCalls([]core.ToolCallItem{
		{Name: "edit", Args: map[string]any{"path": path, "oldString": "package main\n", "newString": "package main\n\nfunc main() {}\n"}},
	}, &budget.ToolQuota{MaxSlots: 8})

	if len(filtered.Allowed) != 1 {
		t.Fatalf("allowed = %d, want edit allowed after write; blocked=%v", len(filtered.Allowed), filtered.Blocked)
	}
}

func TestApplyPostToolHooksForwardsStopResult(t *testing.T) {
	a := newTestAgent()
	a.deps.gov.HookReg.Register(policy.Hook{
		Name:  "stop",
		Point: policy.PostTool,
		On: func(s policy.State) *policy.Result {
			stop := policy.StopCompleted
			return &policy.Result{Stop: &stop}
		},
	})

	if !a.toolRunner.applyPostToolHooks() {
		t.Fatal("expected PostTool stop")
	}
	if a.run.stopReason != policy.StopCompleted {
		t.Fatalf("stopReason = %q, want %q", a.run.stopReason, policy.StopCompleted)
	}
}
