//go:build linux

package impl

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// skipOnLandlockUnavailable skips the test if err indicates the Landlock
// backend became unavailable at runtime (probe passed but enforcement failed).
// This can happen in CI/container environments where the kernel reports
// Landlock support but intermittently blocks landlock_restrict_self at runtime.
func skipOnLandlockUnavailable(t *testing.T, err error) {
	t.Helper()
	var ue UnavailableError
	if errors.As(err, &ue) {
		t.Skipf("Landlock unavailable at runtime: %s", ue.Reason)
	}
}

func TestLandlockAvailable(t *testing.T) {
	if !landlockAvailable() {
		t.Skip("Landlock not available")
	}
}

func TestRunLandlockBash_SimpleEcho(t *testing.T) {
	if !landlockAvailable() {
		t.Skip("Landlock not available")
	}
	ws := t.TempDir()
	out, err := runLandlockBash(t.Context(), "echo hello", Profile{Workspace: ws}, 10*time.Second)
	skipOnLandlockUnavailable(t, err)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected 'hello' in output, got: %q", out)
	}
}

func TestRunLandlockBash_WriteProtection(t *testing.T) {
	if !landlockAvailable() {
		t.Skip("Landlock not available")
	}
	ws := t.TempDir()
	denied := t.TempDir()

	// Write inside workspace (allowed) — must succeed.
	_, err := runLandlockBash(t.Context(), "echo ok > wsfile.txt", Profile{Workspace: ws}, 10*time.Second)
	skipOnLandlockUnavailable(t, err)
	if err != nil {
		t.Fatalf("workspace write should succeed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, "wsfile.txt")); err != nil {
		t.Fatalf("workspace file not created: %v", err)
	}

	// Write outside workspace (denied by Landlock) — must fail.
	_, err = runLandlockBash(t.Context(), "echo bad > "+filepath.Join(denied, "denied.txt"), Profile{Workspace: ws}, 10*time.Second)
	skipOnLandlockUnavailable(t, err)
	if err == nil {
		t.Fatal("expected write outside workspace to be denied")
	}
	if _, err := os.Stat(filepath.Join(denied, "denied.txt")); !os.IsNotExist(err) {
		t.Fatalf("denied file should not exist: %v", err)
	}
}

func TestRunLandlockBash_Timeout(t *testing.T) {
	if !landlockAvailable() {
		t.Skip("Landlock not available")
	}
	ws := t.TempDir()
	_, err := runLandlockBash(t.Context(), "sleep 30", Profile{Workspace: ws}, 2*time.Second)
	skipOnLandlockUnavailable(t, err)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") && !strings.Contains(err.Error(), "signal: killed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRun_FallbackToLandlock verifies that when the native namespace
// backend is disabled, run falls back to the Landlock backend.
func TestRun_FallbackToLandlock(t *testing.T) {
	if !landlockAvailable() {
		t.Skip("Landlock not available")
	}
	t.Setenv("NEKOCODE_DISABLE_NATIVE_SANDBOX", "1")
	ws := t.TempDir()
	out, err := Run(t.Context(), "echo fallback", Profile{Workspace: ws}, 10*time.Second)
	skipOnLandlockUnavailable(t, err)
	if err != nil {
		t.Fatalf("expected fallback to landlock, got error: %v", err)
	}
	if !strings.Contains(out, "fallback") {
		t.Fatalf("expected 'fallback' in output, got: %q", out)
	}
}
