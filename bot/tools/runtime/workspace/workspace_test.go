package workspace

import (
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
