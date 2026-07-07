package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/sandbox"
)

// backend is the sandbox executor used by the shell tool. It defaults to
// sandbox.DefaultBackend and may be overridden (e.g. in tests) via setBackend.
var backend sandbox.Backend = sandbox.DefaultBackend{}

// setBackend replaces the package-level sandbox backend. Intended for tests
// that need to inject a fake backend without touching the real OS sandbox.
func setBackend(b sandbox.Backend) { backend = b }

type sandboxRequest struct {
	Mode          string
	Network       bool
	WritableRoots []string
}

func (r sandboxRequest) permissionCapabilities() []string {
	switch r.Mode {
	case "host":
		return []string{core.CapProcessHost}
	}
	var caps []string
	if r.Network {
		caps = append(caps, core.CapNetOutbound)
	}
	if len(r.WritableRoots) > 0 {
		caps = append(caps, core.CapFsWritePath)
	}
	return uniqueStrings(caps)
}

func runCommand(ctx context.Context, cmdStr string, req sandboxRequest, timeout time.Duration) (string, error) {
	workspace, _ := os.Getwd()
	caps := req.permissionCapabilities()

	// process.host: the escape hatch of last resort. Never enters the sandbox.
	// scope=once ensures it is never persisted — every invocation prompts.
	if hasCapability(caps, core.CapProcessHost) {
		return "", permissionRequired(
			"command requests unsandboxed host execution",
			[]string{core.CapProcessHost},
			"once",
			workspace,
			req.WritableRoots,
		)
	}

	// Explicit sandbox openings require authorization before entering the
	// sandbox. Throw a RequiredPermissionError; execute_one.go catches it,
	// prompts the user (or matches an existing grant), and calls
	// ExecuteWithPermission with the authorized capabilities to re-run inside
	// an enhanced sandbox.
	if len(caps) > 0 {
		return "", permissionRequired(
			fmt.Sprintf("command requests sandbox profile: %s", strings.Join(caps, ", ")),
			caps,
			"project",
			workspace,
			req.WritableRoots,
		)
	}

	profile, err := buildProfileFromRequest(workspace, req, nil)
	if err != nil {
		return "", err
	}
	out, err := backend.Run(ctx, cmdStr, profile, timeout)
	var unavailable sandbox.UnavailableError
	if errors.As(err, &unavailable) {
		return "", hostPermission(err.Error(), workspace, req.WritableRoots)
	}
	return out, err
}

// runCommandWithPermission is called by ExecuteWithPermission after the user
// (or a persisted grant) authorized the capability request. It re-runs the
// command — inside the sandbox, with the authorized openings applied — rather
// than escaping to the host. Only CapProcessHost escapes the sandbox entirely.
func runCommandWithPermission(ctx context.Context, cmdStr string, req sandboxRequest, timeout time.Duration, grant core.PermissionRequest) (string, error) {
	workspace, _ := os.Getwd()
	requestedCaps := req.permissionCapabilities()
	if len(requestedCaps) > 0 && !containsAllCapabilities(grant.Capabilities, requestedCaps) {
		scope := "project"
		reason := fmt.Sprintf("command requests sandbox profile: %s", strings.Join(requestedCaps, ", "))
		if hasCapability(requestedCaps, core.CapProcessHost) {
			scope = "once"
			reason = "command requests unsandboxed host execution"
		}
		return "", permissionRequired(
			reason,
			requestedCaps,
			scope,
			workspace,
			req.WritableRoots,
		)
	}

	// process.host is the only capability that runs on the host without
	// any sandbox isolation.
	if hasCapability(grant.Capabilities, core.CapProcessHost) {
		return backend.RunHost(ctx, cmdStr, timeout)
	}

	// All other capabilities: rebuild an enhanced profile from the authorized
	// capabilities and re-run inside the sandbox with those openings applied.
	profile, err := buildProfileFromRequest(workspace, req, grant.Capabilities)
	if err != nil {
		return "", err
	}
	out, err := backend.Run(ctx, cmdStr, profile, timeout)
	var unavailable sandbox.UnavailableError
	if errors.As(err, &unavailable) {
		return "", hostPermission(err.Error(), workspace, req.WritableRoots)
	}
	return out, err
}

func buildProfileFromRequest(workspace string, req sandboxRequest, authorizedCaps []string) (sandbox.Profile, error) {
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "workspace-write"
	}
	profile := sandbox.Profile{Workspace: workspace}
	switch mode {
	case "workspace-write":
		// Default mode.
	case "read-only":
		profile.Mode = sandbox.ModeReadOnly
	case "host":
		return profile, fmt.Errorf("host execution must use RunHost")
	default:
		return profile, fmt.Errorf("unsupported sandbox_mode %q (want read-only, workspace-write, or host)", mode)
	}
	if req.Network && hasCapability(authorizedCaps, core.CapNetOutbound) {
		profile.Network = true
	}
	if len(req.WritableRoots) > 0 && hasCapability(authorizedCaps, core.CapFsWritePath) {
		profile.WritePaths = append(profile.WritePaths, req.WritableRoots...)
	}
	return profile, nil
}

func hasCapability(caps []string, target string) bool {
	return slices.Contains(caps, target)
}

func containsAllCapabilities(have, need []string) bool {
	for _, n := range need {
		if !hasCapability(have, n) {
			return false
		}
	}
	return true
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
