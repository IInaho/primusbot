package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nekocode/bot/checkpoint"
	"nekocode/bot/command"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/extension/tool/builtin/catalog"
	"nekocode/bot/extension/tool/builtin/shell"
	"nekocode/bot/extension/tool/runtime/workspace"
	"nekocode/bot/provider/types"
	"nekocode/bot/session"
)

func TestRewindMenuShowsTurnAndChangedFiles(t *testing.T) {
	cwd := t.TempDir()
	manager := session.New(cwd)
	cp := checkpoint.New(t.TempDir())
	cp.Activate(manager.CurrentID(), nil, 0)
	turn, err := cp.Begin(manager.CurrentID())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cwd, "main.go")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cp.Capture(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cp.Finalize(path); err != nil {
		t.Fatal(err)
	}
	if err := cp.Finish(manager.CurrentID()); err != nil {
		t.Fatal(err)
	}

	b := &Bot{cwd: cwd, sess: manager, checkpoints: cp}
	parser := command.NewParser()
	parser.Register("rewind", func(context.Context, *command.Command) (string, bool) { return "", true })
	b.registerCommandMenus(parser)
	menu, ok := parser.Menu(context.Background(), "/rewind")
	if !ok || len(menu.Items) != 1 || menu.Items[0].Value != "/rewind "+turn ||
		!menu.Items[0].Submit || !strings.Contains(menu.Items[0].Description, "1 files · +0 ~1 -0") {
		t.Fatalf("rewind menu = %+v, %v", menu, ok)
	}
}

func TestCheckpointChangeHelpersDoNotTreatUnknownAsModified(t *testing.T) {
	turn := checkpoint.TurnInfo{Changes: []checkpoint.FileChange{
		{Kind: checkpoint.ChangeCreated},
		{Kind: checkpoint.ChangeModified},
		{Kind: checkpoint.ChangeDeleted},
		{Kind: checkpoint.ChangeKind("renamed")},
	}}
	created, modified, deleted := checkpointChangeCounts(turn)
	if created != 1 || modified != 1 || deleted != 1 {
		t.Fatalf("change counts = +%d ~%d -%d", created, modified, deleted)
	}
}

func TestSessionCommandResumesDirectManager(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()

	seed := session.New(cwd)
	target := seed.Current()
	target.Messages = []types.Message{{Role: "user", Content: "hello"}}
	if err := seed.Save(target); err != nil {
		t.Fatal(err)
	}

	contextManager := ctxmgr.New(ctxmgr.Config{})
	manager := session.New(cwd)

	b := &Bot{sess: manager, ctxMgr: contextManager}
	parser := command.NewParser()
	b.registerSessionCommands(parser)
	menu, ok := parser.Menu(context.Background(), "/sessions")
	if !ok || len(menu.Items) != 1 || menu.Items[0].Value != "/sessions "+target.ID {
		t.Fatalf("sessions menu = %+v, %v", menu, ok)
	}
	if _, handled := parser.Execute(context.Background(), parser.Parse("/sessions "+target.ID)); !handled {
		t.Fatal("sessions command was not handled")
	}
	if manager.CurrentID() != target.ID {
		t.Fatalf("current session = %q, want %q", manager.CurrentID(), target.ID)
	}
}

func TestEnsureSessionIdentitySyncsManagedProcesses(t *testing.T) {
	manager := session.New(t.TempDir())
	manager.ClearCurrent()
	toolbox := catalog.NewToolbox(nil)
	t.Cleanup(func() { _ = toolbox.Close() })
	b := &Bot{sess: manager, toolbox: toolbox}

	b.ensureSessionIdentity()
	if manager.CurrentID() == "" {
		t.Fatal("session identity was not created")
	}
	registered, err := toolbox.Registry.Get("shell")
	if err != nil {
		t.Fatal(err)
	}
	shellTool := registered.(*shell.ShellTool)
	if shellTool.CurrentSessionID() != manager.CurrentID() {
		t.Fatalf("shell owner = %q, session = %q", shellTool.CurrentSessionID(), manager.CurrentID())
	}
}

func TestBotNewAndDeleteSessionResetCurrentConversation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()
	contextManager := ctxmgr.New(ctxmgr.Config{})
	contextManager.Add("user", "old conversation")
	manager := session.New(cwd)
	b := &Bot{cwd: cwd, ctxMgr: contextManager, sess: manager}

	meta, err := b.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if meta.ID == "" || manager.CurrentID() != meta.ID {
		t.Fatalf("new session = %#v, current = %q", meta, manager.CurrentID())
	}
	if got := len(contextManager.Snapshot().Messages); got != 0 {
		t.Fatalf("new session retained %d old messages", got)
	}

	if err := b.DeleteSession(meta.ID); err != nil {
		t.Fatal(err)
	}
	if manager.CurrentID() != "" {
		t.Fatalf("deleted session remained current: %q", manager.CurrentID())
	}
}

func TestResetConversationStopsOldSessionProcesses(t *testing.T) {
	cwd := t.TempDir()
	manager := session.New(cwd)
	toolbox := catalog.NewToolbox(nil)
	t.Cleanup(func() { _ = toolbox.Close() })
	oldID := manager.CurrentID()
	toolbox.SetSessionID(oldID)
	b := &Bot{
		cwd: cwd, ctxMgr: ctxmgr.New(ctxmgr.Config{}), sess: manager, toolbox: toolbox,
	}

	registered, err := toolbox.Registry.Get("shell")
	if err != nil {
		t.Fatal(err)
	}
	shellTool := registered.(*shell.ShellTool)
	ctx := workspace.WithManager(context.Background(), toolbox.Workspace())
	if _, err := shellTool.Execute(ctx, map[string]any{"command": "sleep 5", "name": "service"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shellTool.ProcessSummary(), "service(running") {
		t.Fatalf("managed process did not start: %q", shellTool.ProcessSummary())
	}

	if _, err := b.resetConversation(false); err != nil {
		t.Fatal(err)
	}
	shellTool.SetSessionID(oldID)
	if strings.Contains(shellTool.ProcessSummary(), "(running") {
		t.Fatalf("old session process survived reset: %q", shellTool.ProcessSummary())
	}
}
