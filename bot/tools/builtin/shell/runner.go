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

func runCommand(ctx context.Context, cmdStr string, timeout time.Duration) (string, error) {
	workspace, _ := os.Getwd()
	plan := analyzeCommand(cmdStr, workspace)
	if err := validateSandboxPolicy(plan); err != nil {
		return "", err
	}
	return runSandboxed(ctx, cmdStr, timeout, plan, core.PermissionRequest{})
}

func runCommandWithPermission(ctx context.Context, cmdStr string, timeout time.Duration, grant core.PermissionRequest) (string, error) {
	workspace, _ := os.Getwd()
	plan := analyzeCommand(cmdStr, workspace)
	if containsCapability(grant, core.CapProcessHost) {
		return sandbox.RunHostBash(ctx, cmdStr, timeout)
	}
	return runSandboxed(ctx, cmdStr, timeout, plan, grant)
}

func validateSandboxPolicy(plan commandPlan) error {
	if plan.Unsafe || plan.Unknown {
		return permissionRequired("command contains dynamic shell syntax that cannot be safely persisted", []string{core.CapShellUnknown}, "once", plan)
	}
	if plan.NeedsNetwork {
		return permissionRequired("command requires public network access", []string{core.CapNetPublic, core.CapCacheWrite}, "project", plan)
	}
	return nil
}

func runSandboxed(ctx context.Context, cmdStr string, timeout time.Duration, plan commandPlan, grant core.PermissionRequest) (string, error) {
	out, err := sandbox.RunSandboxed(ctx, cmdStr, sandbox.BashProfile{
		Workspace:  plan.Workspace,
		Network:    containsCapability(grant, core.CapNetPublic),
		CachePaths: cachePathsForGrant(plan, grant),
	}, timeout)
	if err != nil {
		if unavailable, ok := err.(sandbox.UnavailableError); ok {
			return "", hostPermission(unavailable.Error(), plan)
		}
	}
	return out, err
}

func permissionRequired(reason string, caps []string, scope string, plan commandPlan) core.RequiredPermissionError {
	req := core.PermissionRequest{
		Reason:       reason,
		Capabilities: uniqueStrings(caps),
		Scope:        scope,
		Details: map[string]any{
			"workspace":    plan.Workspace,
			"commandClass": plan.CommandClass,
			"sandbox":      "native",
		},
	}
	if len(plan.WritePaths) > 0 {
		req.Details["writePaths"] = strings.Join(plan.WritePaths, ", ")
	}
	if len(plan.CachePaths) > 0 {
		req.Details["cachePaths"] = strings.Join(plan.CachePaths, ", ")
	}
	return core.RequiredPermissionError{Request: req}
}

func hostPermission(reason string, plan commandPlan) core.RequiredPermissionError {
	err := permissionRequired(fmt.Sprintf("%s; unsandboxed host execution is required", reason), []string{core.CapProcessHost}, "once", plan)
	err.Request.Details["sandbox"] = "unavailable"
	return err
}

func containsCapability(req core.PermissionRequest, cap string) bool {
	for _, c := range req.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

func cachePathsForGrant(plan commandPlan, grant core.PermissionRequest) []string {
	if containsCapability(grant, core.CapCacheWrite) {
		return plan.CachePaths
	}
	return nil
}
