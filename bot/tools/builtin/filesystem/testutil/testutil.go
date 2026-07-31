package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"nekocode/bot/tools/runtime/workspace"
)

func SetupTemp(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	// Workspace guards resolve roots from NEKOCODE_WORKSPACE; set it to this
	// temp dir so file tools accept paths inside the test fixture.
	t.Setenv("NEKOCODE_WORKSPACE", d)
	workspace.Configure(d, nil)
	if err := os.WriteFile(filepath.Join(d, "a.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "b.go"), []byte("package main\n\nfunc helper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(d, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "sub", "c.txt"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return d
}
