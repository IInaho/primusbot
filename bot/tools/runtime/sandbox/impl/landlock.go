//go:build linux

package impl

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

// landlockAvailable reports whether the tbsb Landlock backend is usable.
//
// It performs a one-time self-test so that kernels which report Landlock
// support but block enforcement at runtime (common in containers / CI where
// landlock_restrict_self returns EPERM) are treated as unavailable. Without
// this, callers would attempt the backend, fail to enforce, and surface a
// confusing "landlock restrict failed" error instead of falling back.
func landlockAvailable() bool {
	if !tbsb.Available() {
		return false
	}
	landlockProbeOnce.Do(func() {
		landlockProbeOK = landlockSelfTest()
	})
	return landlockProbeOK
}

// landlockReasonUnavailable returns why Landlock is unavailable, or "".
func landlockReasonUnavailable() string {
	if reason := tbsb.ReasonUnavailable(); reason != "" {
		return reason
	}
	// Kernel reports Landlock as present, but enforcement failed at runtime
	// (e.g. landlock_restrict_self EPERM under a restricted container).
	if tbsb.Available() && !landlockAvailable() {
		if landlockProbeErr != nil {
			return fmt.Sprintf("Landlock enforcement blocked at runtime: %v", landlockProbeErr)
		}
		return "Landlock enforcement blocked at runtime (landlock_restrict_self EPERM)"
	}
	return ""
}

var (
	landlockProbeOnce sync.Once
	landlockProbeOK   bool
	landlockProbeErr  error
)

// landlockSelfTest runs a trivial command through the Landlock backend to
// verify that enforcement actually works at runtime. The kernel may report
// Landlock as available (ABI version > 0) while the container/runtime blocks
// landlock_restrict_self (EPERM); in that case the backend is unusable even
// though tbsb.Available() returns true.
func landlockSelfTest() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ws, err := os.MkdirTemp("", "landlock-probe-")
	if err != nil {
		landlockProbeErr = err
		return false
	}
	defer os.RemoveAll(ws)
	_, err = runLandlockBash(ctx, "echo ok", Profile{Workspace: ws}, 10*time.Second)
	landlockProbeErr = err
	return err == nil
}

// runLandlockBash runs command under the tbsb Landlock backend.
//
// Writable paths are the workspace plus allowed cache paths; everything else
// is read-only. Network is NOT isolated by this backend (Landlock only
// restricts file paths on ABI <4; TCP restrictions need kernel 6.7+ and are
// not applied here). Callers that need network isolation should prefer the
// native namespace backend (runNativeBash); this is the fallback when user
// namespaces are unavailable.
func runLandlockBash(ctx context.Context, command string, profile Profile, timeout time.Duration) (string, error) {
	if profile.Workspace == "" {
		return "", fmt.Errorf("sandbox workspace is required")
	}
	ws, err := filepath.Abs(profile.Workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}

	var writable []string
	if profile.Mode != ModeReadOnly {
		writable = append(writable, ws)
	}
	writePaths, err := resolveWritePaths(profile.WritePaths)
	if err != nil {
		return "", err
	}
	writable = append(writable, writePaths...)

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

func startLandlockBash(ctx context.Context, command string, profile Profile) (*Process, error) {
	if profile.Workspace == "" {
		return nil, fmt.Errorf("sandbox workspace is required")
	}
	ws, err := filepath.Abs(profile.Workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}
	var writable []string
	if profile.Mode != ModeReadOnly {
		writable = append(writable, ws)
	}
	writePaths, err := resolveWritePaths(profile.WritePaths)
	if err != nil {
		return nil, err
	}
	writable = append(writable, writePaths...)

	cfg := struct {
		WritableDirs []string `json:"w"`
	}{WritableDirs: writable}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal sandbox config: %w", err)
	}
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get executable path: %w", err)
	}
	ensureNoNewPrivs()
	args := []string{"__sandbox__", "--", "bash", "-c", command}
	env := append(os.Environ(),
		"__SANDBOX_HELPER=1",
		"__SANDBOX_CONFIG="+string(cfgJSON),
	)
	return startCommand(ctx, self, args, env, ws, &syscall.SysProcAttr{Setpgid: true}, nil)
}
