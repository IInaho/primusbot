package bot

import (
	"context"
	"testing"

	"nekocode/bot/command"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/provider/types"
	"nekocode/bot/session"
)

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
	if _, handled := parser.Execute(context.Background(), parser.Parse("/sessions "+target.ID)); !handled {
		t.Fatal("sessions command was not handled")
	}
	if manager.CurrentID() != target.ID {
		t.Fatalf("current session = %q, want %q", manager.CurrentID(), target.ID)
	}
}

func TestBotNewAndDeleteSessionResetCurrentConversation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()
	contextManager := ctxmgr.New(ctxmgr.Config{})
	contextManager.Add("user", "old conversation")
	manager := session.New(cwd)
	b := &Bot{cwd: cwd, ctxMgr: contextManager, sess: manager}

	meta := b.NewSession()
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
