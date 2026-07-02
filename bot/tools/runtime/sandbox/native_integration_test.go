//go:build linux
// +build linux

package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunNativeBash_SimpleEcho(t *testing.T) {
	if !IsNativeAvailable() {
		t.Skip("native sandbox not available (user namespaces not working)")
	}

	ws := t.TempDir()
	out, err := RunNativeBash(t.Context(), "echo hello", BashProfile{Workspace: ws}, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected 'hello' in output, got: %q", out)
	}
}

func TestRunNativeBash_WorkspaceWrite(t *testing.T) {
	if !IsNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	ws := t.TempDir()
	_, err := RunNativeBash(t.Context(), "echo test > wsfile.txt", BashProfile{Workspace: ws}, 10*time.Second)
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

func TestRunNativeBash_TmpIsIsolated(t *testing.T) {
	if !IsNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	ws := t.TempDir()
	// Write to /tmp inside sandbox, verify it does NOT appear on host /tmp.
	_, err := RunNativeBash(t.Context(), "echo isolated > /tmp/nekocode_isolated_test", BashProfile{Workspace: ws}, 10*time.Second)
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
	if !IsNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	ws := t.TempDir()
	_, err := RunNativeBash(t.Context(), "exit 42", BashProfile{Workspace: ws}, 10*time.Second)
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
}

func TestRunNativeBash_CachePathMounted(t *testing.T) {
	if !IsNativeAvailable() {
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
	out, err := RunNativeBash(t.Context(), "ls ~/.npm", BashProfile{
		Workspace:  ws,
		CachePaths: []string{cacheDir},
	}, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Output should not contain "No such file" — the mount worked.
	if strings.Contains(out, "No such file") {
		t.Fatalf("cache path not mounted: %q", out)
	}
}

func TestRunNativeBash_ReadOnlySystem(t *testing.T) {
	if !IsNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	ws := t.TempDir()
	// Try to write to /usr — should fail because it's read-only.
	_, err := RunNativeBash(t.Context(), "echo x > /usr/test_write", BashProfile{Workspace: ws}, 10*time.Second)
	if err == nil {
		t.Fatal("expected write to /usr to fail")
	}
}

func TestRunNativeBash_Timeout(t *testing.T) {
	if !IsNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	ws := t.TempDir()
	_, err := RunNativeBash(t.Context(), "sleep 30", BashProfile{Workspace: ws}, 2*time.Second)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") && !strings.Contains(err.Error(), "signal: killed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunNativeBash_LargeOutput(t *testing.T) {
	if !IsNativeAvailable() {
		t.Skip("native sandbox not available")
	}

	ws := t.TempDir()
	// Generate output larger than maxOutputBytes (1MB).
	out, err := RunNativeBash(t.Context(), "head -c 2000000 /dev/urandom | base64", BashProfile{Workspace: ws}, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) > maxOutputBytes+100 {
		t.Fatalf("output not truncated: got %d bytes", len(out))
	}
}
