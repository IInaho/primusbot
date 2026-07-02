package runner

import (
	"context"
	"sync"
	"testing"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/common"
)

// fakeToolForPerm is a minimal tool for permission-engine integration tests.
type fakeToolForPerm struct {
	name   string
	danger common.DangerLevel
}

func (t fakeToolForPerm) Name() string                                    { return t.name }
func (t fakeToolForPerm) Description() string                             { return "test" }
func (t fakeToolForPerm) Parameters() []core.Parameter                    { return nil }
func (t fakeToolForPerm) ExecutionMode(map[string]any) core.ExecutionMode { return core.ModeSequential }
func (t fakeToolForPerm) DangerLevel(map[string]any) common.DangerLevel   { return t.danger }
func (t fakeToolForPerm) Execute(context.Context, map[string]any) (string, error) {
	return "ran", nil
}

func newPermTestExecutor() *Executor {
	e := NewExecutor(fakeRegistry{
		"bash":  fakeToolForPerm{name: "bash", danger: common.LevelWrite},
		"write": fakeToolForPerm{name: "write", danger: common.LevelWrite},
	})
	e.SetPermissionPolicy(permission.PermissionsDecl{}, "/repo", "/home/user")
	return e
}

func TestPermEngine_DeniesSudo(t *testing.T) {
	e := newPermTestExecutor()
	e.SetConfirmFn(func(common.ConfirmRequest) common.ConfirmReply { return common.AllowOnce() })

	r := e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "bash", Args: map[string]any{"command": "sudo rm -rf /"}},
	})[0]
	if r.Error == "" {
		t.Fatal("expected sudo to be denied by builtin rule")
	}
}

func TestPermEngine_AllowsReadOnlyBash(t *testing.T) {
	e := newPermTestExecutor()
	called := false
	e.SetConfirmFn(func(common.ConfirmRequest) common.ConfirmReply {
		called = true
		return common.AllowOnce()
	})

	r := e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "bash", Args: map[string]any{"command": "ls -la"}},
	})[0]
	if r.Error != "" {
		t.Fatalf("ls should be allowed without prompt, got error: %v", r.Error)
	}
	if called {
		t.Fatal("confirm should not be called for read-only ls (builtin allow)")
	}
}

func TestPermEngine_AsksForRm(t *testing.T) {
	e := newPermTestExecutor()
	asked := false
	e.SetConfirmFn(func(req common.ConfirmRequest) common.ConfirmReply {
		asked = true
		return common.AllowOnce()
	})

	r := e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "bash", Args: map[string]any{"command": "rm -rf /tmp/x"}},
	})[0]
	if !asked {
		t.Fatal("rm should trigger an ask prompt (builtin ask rule)")
	}
	if r.Error != "" {
		t.Fatalf("approved rm should run, got error: %v", r.Error)
	}
}

func TestPermEngine_AskDeniedCancels(t *testing.T) {
	e := newPermTestExecutor()
	e.SetConfirmFn(func(common.ConfirmRequest) common.ConfirmReply { return common.Deny() })

	r := e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "bash", Args: map[string]any{"command": "rm -rf /tmp/x"}},
	})[0]
	if r.Error == "" {
		t.Fatal("denied rm should return cancelled error")
	}
}

func TestPermEngine_WriteToolNoPrompt(t *testing.T) {
	e := newPermTestExecutor()
	called := false
	e.SetConfirmFn(func(common.ConfirmRequest) common.ConfirmReply {
		called = true
		return common.AllowOnce()
	})

	r := e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "write", Args: map[string]any{"path": "/repo/src/a.go", "content": "x"}},
	})[0]
	if r.Error != "" {
		t.Fatalf("workspace write should be allowed without prompt, got: %v", r.Error)
	}
	if called {
		t.Fatal("write in workspace should not prompt (builtin allow)")
	}
}

func TestPermEngine_DeclaredDenyOverridesBuiltinAllow(t *testing.T) {
	e := NewExecutor(fakeRegistry{
		"bash": fakeToolForPerm{name: "bash", danger: common.LevelWrite},
	})
	// User declares: deny Bash(git push *). Builtin has no git-push rule, so
	// without the deny it would fall to default (ask). The deny must block it.
	e.SetPermissionPolicy(permission.PermissionsDecl{
		Deny: []string{"Bash(git push *)"},
	}, "/repo", "/home/user")
	e.SetConfirmFn(func(common.ConfirmRequest) common.ConfirmReply { return common.AllowOnce() })

	r := e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "bash", Args: map[string]any{"command": "git push origin main"}},
	})[0]
	if r.Error == "" {
		t.Fatal("declared deny should block git push")
	}
}

func TestPermEngine_RememberedAllowSkipsFuturePrompt(t *testing.T) {
	dir := t.TempDir()
	store := permission.NewStore(dir + "/perms.json")
	e := NewExecutor(fakeRegistry{
		"bash": fakeToolForPerm{name: "bash", danger: common.LevelWrite},
	})
	e.SetPermissionStore(store)
	e.SetPermissionPolicy(permission.PermissionsDecl{}, "/repo", "/home/user")

	var mu sync.Mutex
	askCount := 0
	e.SetConfirmFn(func(req common.ConfirmRequest) common.ConfirmReply {
		mu.Lock()
		askCount++
		mu.Unlock()
		return common.AllowRemembered()
	})

	// First rm call: asks, user allows + remember → rule persisted.
	e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "1", Name: "bash", Args: map[string]any{"command": "rm -rf /tmp/x"}},
	})

	// Second rm call: remembered allow rule should skip the prompt.
	e.ExecuteBatch(context.Background(), []core.ToolCallItem{
		{ID: "2", Name: "bash", Args: map[string]any{"command": "rm -rf /tmp/y"}},
	})

	mu.Lock()
	defer mu.Unlock()
	if askCount != 1 {
		t.Fatalf("expected 1 prompt (remembered on 2nd), got %d", askCount)
	}
}
