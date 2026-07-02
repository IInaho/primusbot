//go:build linux

package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	tbsb "github.com/tirdyhouse/sandbox"
)

// Landlock fallback backend.
//
// We import github.com/tirdyhouse/sandbox for its self-exec Landlock helper:
// when a child is spawned with __SANDBOX_HELPER=1 (and a JSON config in
// __SANDBOX_CONFIG), tbsb's init() — registered simply by importing the
// package — sets up Landlock restrictions and exec's the real command.
//
// We do NOT call tbsb.Command() because its Cmd does not expose the process
// handle, so we could not enforce timeout/kill. Instead we replicate its
// applySandbox (env + argv layout) on top of exec.CommandContext, keeping
// full timeout/process-group control, while still letting tbsb's init() do
// the Landlock enforcement in the child.
//
// Known tbsb bug worked around here: setupLandlock omits prctl(PR_SET_NO_NEW_PRIVS),
// so landlock_restrict_self returns EPERM. We set no_new_privs once in the
// parent; it is inherited across exec so the self-exec child gets it too.

var noNewPrivsOnce sync.Once

func ensureNoNewPrivs() {
	noNewPrivsOnce.Do(func() {
		const prSetNoNewPrivs = 38 // PR_SET_NO_NEW_PRIVS
		_, _, _ = syscall.Syscall(syscall.SYS_PRCTL, prSetNoNewPrivs, 1, 0)
	})
}

// LandlockAvailable reports whether the tbsb Landlock backend is usable.
func LandlockAvailable() bool { return tbsb.Available() }

// LandlockReasonUnavailable returns why Landlock is unavailable, or "".
func LandlockReasonUnavailable() string { return tbsb.ReasonUnavailable() }

// RunLandlockBash runs command under the tbsb Landlock backend.
//
// Writable paths are the workspace plus allowed cache paths; everything else
// is read-only. Network is NOT isolated by this backend (Landlock only
// restricts file paths on ABI <4; TCP restrictions need kernel 6.7+ and are
// not applied here). Callers that need network isolation should prefer the
// native namespace backend (RunNativeBash); this is the fallback when user
// namespaces are unavailable.
func RunLandlockBash(ctx context.Context, command string, profile BashProfile, timeout time.Duration) (string, error) {
	if profile.Workspace == "" {
		return "", fmt.Errorf("sandbox workspace is required")
	}
	ws, err := filepath.Abs(profile.Workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}

	writable := []string{ws}
	cachePaths, err := allowedCachePaths(profile.CachePaths)
	if err != nil {
		return "", err
	}
	writable = append(writable, cachePaths...)

	// helperConfig mirrors tbsb's unexported helperConfig{WritableDirs `json:"w"`}.
	cfg := struct {
		WritableDirs []string `json:"w"`
	}{WritableDirs: writable}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal sandbox config: %w", err)
	}

	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get executable path: %w", err)
	}

	ensureNoNewPrivs()

	// Replicate tbsb applySandbox argv: <self> __sandbox__ -- bash -c <command>
	args := []string{"__sandbox__", "--", "bash", "-c", command}
	env := append(os.Environ(),
		"__SANDBOX_HELPER=1",
		"__SANDBOX_CONFIG="+string(cfgJSON),
	)
	sysproc := &syscall.SysProcAttr{Setpgid: true}

	// tbsb's init() prints "sandbox: landlock setup failed: ..." (with
	// restrict_self EPERM) when Landlock cannot be enforced. Match that
	// precisely so an EPERM from the command itself is not misclassified.
	unavail := func(stderr string, _ error) bool {
		return strings.Contains(stderr, "landlock setup failed")
	}

	return runChild(ctx, self, args, env, ws, timeout, sysproc, unavail, "landlock restrict failed")
}
