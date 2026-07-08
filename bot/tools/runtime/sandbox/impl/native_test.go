//go:build linux
// +build linux

package impl

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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

func TestSandboxTmpfsSizeUsesValidatedEnv(t *testing.T) {
	t.Setenv("NEKOCODE_SANDBOX_ROOT_SIZE", "4g")
	if got := sandboxTmpfsSize("NEKOCODE_SANDBOX_ROOT_SIZE", "2g"); got != "4g" {
		t.Fatalf("sandbox tmpfs size = %q, want 4g", got)
	}

	t.Setenv("NEKOCODE_SANDBOX_ROOT_SIZE", "4g,mode=777")
	if got := sandboxTmpfsSize("NEKOCODE_SANDBOX_ROOT_SIZE", "2g"); got != "2g" {
		t.Fatalf("invalid sandbox tmpfs size should fall back, got %q", got)
	}

	t.Setenv("NEKOCODE_SANDBOX_ROOT_SIZE", "g")
	if got := sandboxTmpfsSize("NEKOCODE_SANDBOX_ROOT_SIZE", "2g"); got != "2g" {
		t.Fatalf("suffix-only sandbox tmpfs size should fall back, got %q", got)
	}
}

func TestCleanupStaleStagingRootsOnlyRemovesDeadOwnedDirs(t *testing.T) {
	parent := t.TempDir()

	stale := filepath.Join(parent, stagingRootPrefix+"stale")
	if err := os.Mkdir(stale, 0o700); err != nil {
		t.Fatalf("create stale staging: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stale, stagingOwnerFile), []byte("0\n"), 0o600); err != nil {
		t.Fatalf("write stale owner: %v", err)
	}

	active := filepath.Join(parent, stagingRootPrefix+"active")
	if err := os.Mkdir(active, 0o700); err != nil {
		t.Fatalf("create active staging: %v", err)
	}
	if err := os.WriteFile(filepath.Join(active, stagingOwnerFile), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		t.Fatalf("write active owner: %v", err)
	}

	unmarked := filepath.Join(parent, stagingRootPrefix+"unmarked")
	if err := os.Mkdir(unmarked, 0o700); err != nil {
		t.Fatalf("create unmarked staging: %v", err)
	}

	cleanupStaleStagingRoots(parent)

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale staging should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(active); err != nil {
		t.Fatalf("active staging should remain: %v", err)
	}
	if _, err := os.Stat(unmarked); err != nil {
		t.Fatalf("unmarked staging should remain: %v", err)
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
