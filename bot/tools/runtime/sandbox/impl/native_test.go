//go:build linux
// +build linux

package impl

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestIsNativeAvailableRequiresLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native sandbox only available on linux")
	}
	// We just verify the function doesn't panic; result depends on kernel config.
	_ = isNativeAvailable()
}

func TestRunNativeBashRejectsEmptyWorkspace(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native sandbox only available on linux")
	}
	_, err := runNativeBash(t.Context(), "echo ok", Profile{}, 0)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
	if !strings.Contains(err.Error(), "workspace is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunNativeBashAcceptsArbitraryWritePath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native sandbox only available on linux")
	}
	// resolveWritePaths no longer rejects arbitrary paths — the sandbox
	// package does not enforce a whitelist. Authorization is the caller's
	// responsibility. Verify that a path like ~/.ssh is accepted (no error
	// from the resolve step). The actual bind may still fail at mount time,
	// but that is a backend concern, not a policy rejection.
	tmp := t.TempDir()
	_, err := runNativeBash(t.Context(), "echo ok", Profile{
		Workspace:  t.TempDir(),
		WritePaths: []string{tmp},
	}, 10*time.Second)
	if err != nil && strings.Contains(err.Error(), "outside allowed") {
		t.Fatalf("sandbox should not whitelist-reject write paths: %v", err)
	}
}

func TestBuildSandboxEnvUsesSandboxTmp(t *testing.T) {
	t.Setenv("TMPDIR", "/tmp/nix-shell-missing")
	t.Setenv("TMP", "/tmp/nix-shell-missing")
	t.Setenv("TEMP", "/tmp/nix-shell-missing")

	env := buildSandboxEnv()
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		if got := envValue(env, key); got != "/tmp" {
			t.Fatalf("%s = %q, want /tmp", key, got)
		}
	}
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix)
		}
	}
	return ""
}
