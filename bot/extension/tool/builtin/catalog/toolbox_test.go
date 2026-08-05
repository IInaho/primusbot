package catalog

import (
	"path/filepath"
	"testing"

	"nekocode/bot/extension/tool/runtime/workspace"
)

func TestToolboxScopesTemporaryRootsBySession(t *testing.T) {
	box := NewToolbox(nil)
	t.Cleanup(func() { _ = box.Close() })
	primary := t.TempDir()
	extra := t.TempDir()
	box.Workspace().Configure(primary, nil)
	box.SetSessionID("one")
	if _, err := box.Workspace().AddSessionRoot(extra, workspace.AccessReadOnly); err != nil {
		t.Fatal(err)
	}

	box.SetSessionID("two")
	if _, _, ok, err := box.Workspace().CheckRead(filepath.Join(extra, "note.md")); err != nil || ok {
		t.Fatalf("temporary root leaked into another session: ok=%v err=%v", ok, err)
	}
	if err := box.CloseSession("one"); err != nil {
		t.Fatal(err)
	}
	box.SetSessionID("one")
	if _, _, ok, err := box.Workspace().CheckRead(filepath.Join(extra, "note.md")); err != nil || ok {
		t.Fatalf("closed session retained temporary root: ok=%v err=%v", ok, err)
	}
}
