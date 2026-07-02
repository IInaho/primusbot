package toolutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePathInWorkspace_AllowsInsideCwd(t *testing.T) {
	ws := t.TempDir()
	old, _ := os.Getwd()
	defer os.Chdir(old)
	if err := os.Chdir(ws); err != nil {
		t.Fatal(err)
	}
	// create a file inside workspace
	inner := filepath.Join(ws, "src", "a.go")
	os.MkdirAll(filepath.Dir(inner), 0o755)
	os.WriteFile(inner, []byte("x"), 0o644)

	got, err := ValidatePathInWorkspace(inner)
	if err != nil {
		t.Fatalf("inside-cwd path should be allowed: %v", err)
	}
	if got == "" {
		t.Fatal("expected resolved path")
	}
}

func TestValidatePathInWorkspace_RejectsOutsideCwd(t *testing.T) {
	ws := t.TempDir()
	old, _ := os.Getwd()
	defer os.Chdir(old)
	if err := os.Chdir(ws); err != nil {
		t.Fatal(err)
	}
	// path outside workspace (a temp dir that is not under ws)
	outside := filepath.Join(t.TempDir(), "evil.txt")
	_, err := ValidatePathInWorkspace(outside)
	if err == nil {
		t.Fatal("path outside workspace should be rejected")
	}
}

func TestValidatePathInWorkspace_AllowsExtraRoot(t *testing.T) {
	ws := t.TempDir()
	extra := t.TempDir()
	old, _ := os.Getwd()
	defer func() {
		os.Chdir(old)
		os.Unsetenv("NEKOCODE_EXTRA_DIRS")
	}()
	if err := os.Chdir(ws); err != nil {
		t.Fatal(err)
	}
	os.Setenv("NEKOCODE_EXTRA_DIRS", extra)

	inner := filepath.Join(extra, "cache.txt")
	os.WriteFile(inner, []byte("x"), 0o644)
	if _, err := ValidatePathInWorkspace(inner); err != nil {
		t.Fatalf("path inside NEKOCODE_EXTRA_DIRS should be allowed: %v", err)
	}
}
