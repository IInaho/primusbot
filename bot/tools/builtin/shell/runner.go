package shell

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/sandbox"
)

// cacheRoots are the package manager cache directories that fs.write.cache
// binds into the sandbox as read-write.
var cacheRoots = []string{
	"~/.npm",
	"~/.pnpm-store",
	"~/.cache/yarn",
	"~/.cache/go-build",
	"~/go/pkg/mod",
	"~/.cargo",
}

// backend is the sandbox executor used by the shell tool. It defaults to
// sandbox.DefaultBackend and may be overridden (e.g. in tests) via setBackend.
var backend sandbox.Backend = sandbox.DefaultBackend{}

// setBackend replaces the package-level sandbox backend. Intended for tests
// that need to inject a fake backend without touching the real OS sandbox.
func setBackend(b sandbox.Backend) { backend = b }

func runCommand(ctx context.Context, cmdStr string, caps []string, writePaths []string, timeout time.Duration) (string, error) {
	workspace, _ := os.Getwd()

	// process.host: the escape hatch of last resort. Never enters the sandbox.
	// scope=once ensures it is never persisted — every invocation prompts.
	if hasCapability(caps, core.CapProcessHost) {
		return "", permissionRequired(
			"command requests unsandboxed host execution",
			[]string{core.CapProcessHost},
			"once",
			workspace,
			writePaths,
		)
	}

	// Declared capabilities (net.outbound / fs.write.cache / fs.write.path)
	// require authorization before entering the sandbox. Throw a
	// RequiredPermissionError; execute_one.go catches it, prompts the user
	// (or matches an existing grant), and calls ExecuteWithPermission with
	// the authorized capabilities to re-run inside an enhanced sandbox.
	if len(caps) > 0 {
		return "", permissionRequired(
			fmt.Sprintf("command declares capabilities: %s", strings.Join(caps, ", ")),
			caps,
			"project",
			workspace,
			writePaths,
		)
	}

	// No capabilities declared — run in the default sandbox (strictest
	// isolation: no network, only workspace writable, system dirs read-only).
	out, err := backend.Run(ctx, cmdStr, sandbox.Profile{Workspace: workspace}, timeout)
	if err != nil {
		if _, ok := err.(sandbox.UnavailableError); ok {
			return "", hostPermission(err.Error(), workspace, writePaths)
		}
	}
	return out, err
}

// runCommandWithPermission is called by ExecuteWithPermission after the user
// (or a persisted grant) authorized the capability request. It re-runs the
// command — inside the sandbox, with the authorized openings applied — rather
// than escaping to the host. Only CapProcessHost escapes the sandbox entirely.
func runCommandWithPermission(ctx context.Context, cmdStr string, writePaths []string, timeout time.Duration, grant core.PermissionRequest) (string, error) {
	workspace, _ := os.Getwd()

	// process.host is the only capability that runs on the host without
	// any sandbox isolation.
	if hasCapability(grant.Capabilities, core.CapProcessHost) {
		return backend.RunHost(ctx, cmdStr, timeout)
	}

	// All other capabilities: rebuild an enhanced profile from the authorized
	// capabilities and re-run inside the sandbox with those openings applied.
	profile := buildProfile(workspace, grant.Capabilities, writePaths)
	out, err := backend.Run(ctx, cmdStr, profile, timeout)
	if err != nil {
		if _, ok := err.(sandbox.UnavailableError); ok {
			return "", hostPermission(err.Error(), workspace, writePaths)
		}
	}
	return out, err
}

// buildProfile constructs a sandbox.Profile from declared/authorized
// capabilities. Each capability opens a specific isolation boundary without
// leaving the sandbox:
//
//   - net.outbound     → share host network namespace (Network=true)
//   - fs.write.cache   → bind package manager cache dirs as read-write
//   - fs.write.path    → bind write_paths argument dirs as read-write
func buildProfile(workspace string, caps []string, writePaths []string) sandbox.Profile {
	profile := sandbox.Profile{Workspace: workspace}
	for _, cap := range caps {
		switch cap {
		case core.CapNetOutbound:
			profile.Network = true
		case core.CapFsWriteCache:
			profile.WritePaths = append(profile.WritePaths, cacheRoots...)
		case core.CapFsWritePath:
			profile.WritePaths = append(profile.WritePaths, writePaths...)
		}
	}
	return profile
}

func hasCapability(caps []string, target string) bool {
	for _, c := range caps {
		if c == target {
			return true
		}
	}
	return false
}

func permissionRequired(reason string, caps []string, scope string, workspace string, writePaths []string) core.RequiredPermissionError {
	req := core.PermissionRequest{
		Reason:       reason,
		Capabilities: uniqueStrings(caps),
		Scope:        scope,
		Details: map[string]any{
			"workspace": workspace,
			"sandbox":   "native",
		},
	}
	if len(writePaths) > 0 {
		req.Details["writePaths"] = strings.Join(writePaths, ", ")
	}
	return core.RequiredPermissionError{Request: req}
}

func hostPermission(reason string, workspace string, writePaths []string) core.RequiredPermissionError {
	err := permissionRequired(
		fmt.Sprintf("%s; unsandboxed host execution is required", reason),
		[]string{core.CapProcessHost},
		"once",
		workspace,
		writePaths,
	)
	err.Request.Details["sandbox"] = "unavailable"
	return err
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
