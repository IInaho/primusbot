package app

import (
	"testing"

	"nekocode/bot/command"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/provider/types"
	"nekocode/bot/session"
)

func TestSessionCommandResumesDirectManager(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()

	target, err := session.New(cwd)
	if err != nil {
		t.Fatal(err)
	}
	target.Messages = []types.Message{{Role: "user", Content: "hello"}}
	if err := target.Save(); err != nil {
		t.Fatal(err)
	}

	manager := session.NewManager(session.ManagerOptions{
		CWD:     cwd,
		Context: ctxmgr.New(ctxmgr.Config{}),
	})
	if err := manager.Init(); err != nil {
		t.Fatal(err)
	}

	b := &Bot{sess: manager}
	parser := command.NewParser()
	b.registerSessionCommands(parser)
	if _, handled := parser.Execute(parser.Parse("/sessions " + target.ID)); !handled {
		t.Fatal("sessions command was not handled")
	}
	if manager.CurrentID() != target.ID {
		t.Fatalf("current session = %q, want %q", manager.CurrentID(), target.ID)
	}
	if !b.drainSessionResumed() || b.drainSessionResumed() {
		t.Fatal("resume signal should be emitted exactly once")
	}
}
