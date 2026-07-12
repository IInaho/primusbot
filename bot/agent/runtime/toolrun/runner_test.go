package toolrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/hooks"
	"nekocode/bot/hooks/builtin"
	aggov "nekocode/bot/policy"
	"nekocode/bot/policy/budget"
	"nekocode/bot/policy/ledger"
	"nekocode/bot/tools"
	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/runner"
)

type fakeHost struct {
	ctx      context.Context
	ctxMgr   *ctxmgr.Manager
	executor *runner.Executor
	gov      *aggov.Manager
	subSlots *SlotManager
	step     int
	hints    []*hooks.Hint
	postTool []hooks.Result
}

func newFakeHost() *fakeHost {
	hookReg := hooks.NewRegistry()
	builtin.Register(hookReg)
	return &fakeHost{
		ctx:      context.Background(),
		ctxMgr:   ctxmgr.NewSub("test", 128000, nil),
		executor: runner.NewExecutor(tools.NewRegistry()),
		gov:      aggov.NewManager(hookReg),
		subSlots: NewSlotManager(),
	}
}

func (h *fakeHost) Context() context.Context        { return h.ctx }
func (h *fakeHost) ContextManager() *ctxmgr.Manager { return h.ctxMgr }
func (h *fakeHost) Executor() *runner.Executor      { return h.executor }
func (h *fakeHost) Governance() *aggov.Manager      { return h.gov }
func (h *fakeHost) SubSlots() *SlotManager          { return h.subSlots }
func (h *fakeHost) InjectHint(hint *hooks.Hint)     { h.hints = append(h.hints, hint) }
func (h *fakeHost) IncStep()                        { h.step++ }
func (h *fakeHost) ApplyPostToolHookResult(result hooks.Result) bool {
	h.postTool = append(h.postTool, result)
	return result.Stop != nil
}

func TestFilterToolCallsAppliesPreToolPolicyBlock(t *testing.T) {
	host := newFakeHost()
	host.gov.HookReg.Register(hooks.Hook{
		Name:  "block-read",
		Point: hooks.PreToolUse,
		On: func(s hooks.State) *hooks.Result {
			return &hooks.Result{BlockTool: &hooks.BlockTool{
				Tool:   "read",
				Reason: "read blocked",
			}}
		},
	})

	filtered := New(host).FilterToolCalls([]core.ToolCallItem{
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
	host := newFakeHost()
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	filtered := New(host).FilterToolCalls([]core.ToolCallItem{
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
	host := newFakeHost()
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	host.gov.RecordToolCall(ledger.ToolEvent{
		Name: "read",
		Args: map[string]any{"path": path},
	})

	filtered := New(host).FilterToolCalls([]core.ToolCallItem{
		{Name: "write", Args: map[string]any{"path": path}},
	}, &budget.ToolQuota{MaxSlots: 8})

	if len(filtered.Allowed) != 1 {
		t.Fatalf("allowed = %d, want write allowed after read; blocked=%v", len(filtered.Allowed), filtered.Blocked)
	}
}

func TestFilterToolCallsAllowsEditWithSufficientAnchor(t *testing.T) {
	host := newFakeHost()
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {\n\tmessage := \"hello\"\n\tprintln(message)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	filtered := New(host).FilterToolCalls([]core.ToolCallItem{
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
	host := newFakeHost()
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	filtered := New(host).FilterToolCalls([]core.ToolCallItem{
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
	host := newFakeHost()
	path := filepath.Join(t.TempDir(), "new.go")

	filtered := New(host).FilterToolCalls([]core.ToolCallItem{
		{Name: "write", Args: map[string]any{"path": path}},
	}, &budget.ToolQuota{MaxSlots: 8})

	if len(filtered.Allowed) != 1 {
		t.Fatalf("allowed = %d, want new file write allowed; blocked=%v", len(filtered.Allowed), filtered.Blocked)
	}
}

func TestFilterToolCallsAllowsEditAfterSuccessfulWrite(t *testing.T) {
	host := newFakeHost()
	path := filepath.Join(t.TempDir(), "main.go")
	host.gov.RecordToolCall(ledger.ToolEvent{
		Name: "write",
		Args: map[string]any{"path": path},
	})

	filtered := New(host).FilterToolCalls([]core.ToolCallItem{
		{Name: "edit", Args: map[string]any{"path": path, "oldString": "package main\n", "newString": "package main\n\nfunc main() {}\n"}},
	}, &budget.ToolQuota{MaxSlots: 8})

	if len(filtered.Allowed) != 1 {
		t.Fatalf("allowed = %d, want edit allowed after write; blocked=%v", len(filtered.Allowed), filtered.Blocked)
	}
}

func TestApplyPostToolHooksForwardsStopResult(t *testing.T) {
	host := newFakeHost()
	host.gov.HookReg.Register(hooks.Hook{
		Name:  "stop",
		Point: hooks.PostTool,
		On: func(s hooks.State) *hooks.Result {
			stop := hooks.StopCompleted
			return &hooks.Result{Stop: &stop}
		},
	})

	if !New(host).ApplyPostToolHooks() {
		t.Fatal("expected PostTool stop")
	}
	if len(host.postTool) != 1 {
		t.Fatalf("postTool results = %d, want 1", len(host.postTool))
	}
	if got := host.postTool[0].Stop; got == nil || *got != hooks.StopCompleted {
		t.Fatalf("forwarded stop = %+v, want completed", got)
	}
}
