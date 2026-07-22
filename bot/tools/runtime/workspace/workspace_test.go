package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceReadWriteAccess(t *testing.T) {
	primary := t.TempDir()
	docs := t.TempDir()
	Configure(primary, []Root{{Path: docs, Access: AccessReadOnly}})

	if _, _, ok, err := CheckRead(filepath.Join(primary, "a.go")); err != nil || !ok {
		t.Fatalf("primary should be readable: ok=%v err=%v", ok, err)
	}
	if _, _, ok, err := CheckWrite(filepath.Join(primary, "a.go")); err != nil || !ok {
		t.Fatalf("primary should be writable: ok=%v err=%v", ok, err)
	}
	if _, _, ok, err := CheckRead(filepath.Join(docs, "note.md")); err != nil || !ok {
		t.Fatalf("read-only root should be readable: ok=%v err=%v", ok, err)
	}
	if _, _, ok, err := CheckWrite(filepath.Join(docs, "note.md")); err != nil || ok {
		t.Fatalf("read-only root should not be writable: ok=%v err=%v", ok, err)
	}

	outside := filepath.Join(t.TempDir(), "evil.txt")
	if _, _, ok, err := CheckRead(outside); err != nil || ok {
		t.Fatalf("outside root should not be readable: ok=%v err=%v", ok, err)
	}
	if _, _, ok, err := CheckWrite(outside); err != nil || ok {
		t.Fatalf("outside root should not be writable: ok=%v err=%v", ok, err)
	}
}

func TestWorkspaceAddSessionRootUpgradesAccess(t *testing.T) {
	primary := t.TempDir()
	extra := t.TempDir()
	Configure(primary, []Root{{Path: extra, Access: AccessReadOnly}})

	if _, err := AddSessionRoot(extra, AccessReadWrite); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := CheckWrite(filepath.Join(extra, "out.txt")); err != nil || !ok {
		t.Fatalf("upgraded root should be writable: ok=%v err=%v", ok, err)
	}
}

func TestWorkspaceExtraReadWriteRoot(t *testing.T) {
	primary := t.TempDir()
	extra := t.TempDir()
	Configure(primary, []Root{{Path: extra, Access: AccessReadWrite}})

	if _, _, ok, err := CheckWrite(filepath.Join(extra, "cache.txt")); err != nil || !ok {
		t.Fatalf("extra read-write root should be writable: ok=%v err=%v", ok, err)
	}
}

func TestConfigurePreservesSessionRoots(t *testing.T) {
	primary := t.TempDir()
	sessionRoot := t.TempDir()

	Configure(primary, nil)
	if _, err := AddSessionRoot(sessionRoot, AccessReadOnly); err != nil {
		t.Fatal(err)
	}

	// Simulate a hot reload (ApplyConfig -> reinit -> Configure): the
	// session root added during the session must survive.
	Configure(primary, nil)

	if _, _, ok, err := CheckRead(filepath.Join(sessionRoot, "note.md")); err != nil || !ok {
		t.Fatalf("session root should survive Configure: ok=%v err=%v", ok, err)
	}
}

func TestResolveExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory available")
	}
	if real, err := filepath.EvalSymlinks(home); err == nil {
		home = real
	}
	resolved, err := Resolve("~/nekocode-test-file")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "nekocode-test-file")
	if resolved != want {
		t.Fatalf("Resolve(~/) = %q, want %q", resolved, want)
	}
}
