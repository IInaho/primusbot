//go:build linux
// +build linux

package impl

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRunNativeBash_SimpleEcho(t *testing.T) {
	if !isNativeAvailable() {
		t.Skip("native sandbox not available (user namespaces not working)")
	}

	ws := t.TempDir()
	out, err := runNativeBash(t.Context(), "echo hello", Profile{Workspace: ws}, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected 'hello' in output, got: %q", out)
	}
}

func TestRunNativeBash_WorkspaceWrite(t *testing.T) {
	if !isNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	ws := t.TempDir()
	_, err := runNativeBash(t.Context(), "echo test > wsfile.txt", Profile{Workspace: ws}, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(ws, "wsfile.txt"))
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if !strings.Contains(string(data), "test") {
		t.Fatalf("file content mismatch: %q", string(data))
	}
}

func TestRunNativeBash_ReadOnlyWorkspace(t *testing.T) {
	if !isNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	ws := t.TempDir()
	_, err := runNativeBash(t.Context(), "echo test > wsfile.txt", Profile{Workspace: ws, Mode: ModeReadOnly}, 10*time.Second)
	if err == nil {
		t.Fatal("expected workspace write to fail in read-only mode")
	}
	if _, statErr := os.Stat(filepath.Join(ws, "wsfile.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("read-only workspace leaked write, stat err = %v", statErr)
	}
}

func TestRunNativeBash_TmpIsIsolated(t *testing.T) {
	if !isNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	ws := t.TempDir()
	// Write to /tmp inside sandbox, verify it does NOT appear on host /tmp.
	_, err := runNativeBash(t.Context(), "echo isolated > /tmp/nekocode_isolated_test", Profile{Workspace: ws}, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = os.Stat("/tmp/nekocode_isolated_test")
	if err == nil {
		t.Fatal("file in sandbox /tmp leaked to host")
		os.Remove("/tmp/nekocode_isolated_test")
	}
}

func TestRunNativeBash_CommandFailure(t *testing.T) {
	if !isNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	ws := t.TempDir()
	_, err := runNativeBash(t.Context(), "exit 42", Profile{Workspace: ws}, 10*time.Second)
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
}

func TestRunNativeBash_CachePathMounted(t *testing.T) {
	if !isNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no home directory")
	}

	cacheDir := filepath.Join(home, ".npm")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("create cache dir: %v", err)
	}

	ws := t.TempDir()
	out, err := runNativeBash(t.Context(), "ls ~/.npm", Profile{
		Workspace:  ws,
		WritePaths: []string{cacheDir},
	}, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Output should not contain "No such file" — the mount worked.
	if strings.Contains(out, "No such file") {
		t.Fatalf("cache path not mounted: %q", out)
	}
}

// TestRunNativeBash_WritePathCreatedUnderHome verifies that a WritePath
// located under a read-only top-level directory (e.g. /home) that does not
// yet exist on the host can still be created and written inside the sandbox.
// In the legacy hide mode /home was hidden, so MkdirAll created fresh dirs in
// the tmpfs root; after the "restrict, don't hide" change /home is bound
// read-only, which would make MkdirAll fail with EPERM before the writable
// bind could be established.
func TestRunNativeBash_WritePathCreatedUnderHome(t *testing.T) {
	if !isNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no home directory")
	}

	// A path under $HOME that does not exist on the host.
	newDir := filepath.Join(home, ".nekocode_test_writepath_"+strconv.Itoa(os.Getpid()))
	// Ensure it does not exist on the host before the run.
	_ = os.RemoveAll(newDir)
	t.Cleanup(func() { _ = os.RemoveAll(newDir) })

	ws := t.TempDir()
	_, err := runNativeBash(t.Context(), "echo data > "+newDir+"/file.txt", Profile{
		Workspace:  ws,
		WritePaths: []string{newDir},
	}, 10*time.Second)
	if err != nil {
		t.Fatalf("write to non-existent WritePath under /home failed: %v", err)
	}
}

func TestRunNativeBash_ReadOnlySystem(t *testing.T) {
	if !isNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	ws := t.TempDir()
	// Try to write to /usr — should fail because it's read-only.
	_, err := runNativeBash(t.Context(), "echo x > /usr/test_write", Profile{Workspace: ws}, 10*time.Second)
	if err == nil {
		t.Fatal("expected write to /usr to fail")
	}
}

func TestRunNativeBash_Timeout(t *testing.T) {
	if !isNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	ws := t.TempDir()
	_, err := runNativeBash(t.Context(), "sleep 30", Profile{Workspace: ws}, 2*time.Second)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") && !strings.Contains(err.Error(), "signal: killed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunNativeBash_LargeOutput(t *testing.T) {
	if !isNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	ws := t.TempDir()
	// Generate output larger than maxOutputBytes (1MB).
	out, err := runNativeBash(t.Context(), "head -c 2000000 /dev/urandom | base64", Profile{Workspace: ws}, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) > maxOutputBytes+100 {
		t.Fatalf("output not truncated: got %d bytes", len(out))
	}
}
