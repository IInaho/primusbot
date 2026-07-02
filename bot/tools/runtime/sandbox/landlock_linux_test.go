//go:build linux

package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLandlockAvailable(t *testing.T) {
	if !LandlockAvailable() {
		t.Skipf("Landlock not available: %s", LandlockReasonUnavailable())
	}
}

func TestRunLandlockBash_SimpleEcho(t *testing.T) {
	if !LandlockAvailable() {
		t.Skipf("Landlock not available: %s", LandlockReasonUnavailable())
	}
	ws := t.TempDir()
	out, err := RunLandlockBash(t.Context(), "echo hello", BashProfile{Workspace: ws}, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected 'hello' in output, got: %q", out)
	}
}

func TestRunLandlockBash_WriteProtection(t *testing.T) {
	if !LandlockAvailable() {
		t.Skipf("Landlock not available: %s", LandlockReasonUnavailable())
	}
	ws := t.TempDir()
	denied := t.TempDir()

	// Write inside workspace (allowed) — must succeed.
	_, err := RunLandlockBash(t.Context(), "echo ok > wsfile.txt", BashProfile{Workspace: ws}, 10*time.Second)
	if err != nil {
		t.Fatalf("workspace write should succeed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, "wsfile.txt")); err != nil {
		t.Fatalf("workspace file not created: %v", err)
	}

	// Write outside workspace (denied by Landlock) — must fail.
	_, err = RunLandlockBash(t.Context(), "echo bad > "+filepath.Join(denied, "denied.txt"), BashProfile{Workspace: ws}, 10*time.Second)
	if err == nil {
		t.Fatal("expected write outside workspace to be denied")
	}
	if _, err := os.Stat(filepath.Join(denied, "denied.txt")); !os.IsNotExist(err) {
		t.Fatalf("denied file should not exist: %v", err)
	}
}

func TestRunLandlockBash_Timeout(t *testing.T) {
	if !LandlockAvailable() {
		t.Skipf("Landlock not available: %s", LandlockReasonUnavailable())
	}
	ws := t.TempDir()
	_, err := RunLandlockBash(t.Context(), "sleep 30", BashProfile{Workspace: ws}, 2*time.Second)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") && !strings.Contains(err.Error(), "signal: killed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRunSandboxed_FallbackToLandlock verifies that when the native namespace
// backend is disabled, RunSandboxed falls back to the Landlock backend.
func TestRunSandboxed_FallbackToLandlock(t *testing.T) {
	if !LandlockAvailable() {
		t.Skipf("Landlock not available: %s", LandlockReasonUnavailable())
	}
	t.Setenv("NEKOCODE_DISABLE_NATIVE_SANDBOX", "1")
	ws := t.TempDir()
	out, err := RunSandboxed(t.Context(), "echo fallback", BashProfile{Workspace: ws}, 10*time.Second)
	if err != nil {
		t.Fatalf("expected fallback to landlock, got error: %v", err)
	}
	if !strings.Contains(out, "fallback") {
		t.Fatalf("expected 'fallback' in output, got: %q", out)
	}
}
