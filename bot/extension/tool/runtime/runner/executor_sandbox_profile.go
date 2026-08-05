package runner

import (
	"fmt"
	"os"
	"strings"

	"nekocode/bot/extension/tool/runtime/core"
	"nekocode/bot/extension/tool/runtime/permission"
)

func (e *Executor) predictedPermissionRequest(toolName string, args map[string]any) *core.PermissionRequest {
	caps := sandboxCapsFromArgs(args)
	if len(caps) == 0 {
		return nil
	}
	reason := fmt.Sprintf("command requests sandbox profile: %s", strings.Join(caps, ", "))
	_, ws, _ := e.permissionEngine()
	if ws == "" {
		ws, _ = os.Getwd()
	}
	req := core.PermissionRequest{
		Reason:       reason,
		Capabilities: caps,
		Scope:        "project",
		Details:      map[string]any{"workspace": ws, "sandbox": "native"},
	}
	if writePaths := stringSliceArg(args, "writable_roots"); len(writePaths) > 0 {
		req.Details["writePaths"] = writePaths
	}
	if hasProcessHost(caps) {
		req.Scope = "once"
	}
	return &req
}

func (e *Executor) applySandboxProfile(tc core.ToolCallItem) core.ToolCallItem {
	engine, ws, home := e.permissionEngine()
	if engine == nil {
		return tc
	}
	if cmd, ok := shellCommandForPolicy(tc); ok {
		callInfo := permission.BuildCallInfo(tc.Name, map[string]any{"command": cmd}, ws, home)
		if profile, matched := engine.SandboxFor(tc.Name, callInfo); matched {
			tc.Args = mergeSandboxArgs(tc.Args, profile)
		}
		return tc
	}
	if tc.Name != "shell" {
		return tc
	}
	callInfo := permission.BuildCallInfo(tc.Name, tc.Args, ws, home)
	if profile, matched := engine.SandboxFor(tc.Name, callInfo); matched {
		tc.Args = mergeSandboxArgs(tc.Args, profile)
	}
	return tc
}

func mergeSandboxArgs(args map[string]any, profile permission.SandboxProfile) map[string]any {
	if args == nil {
		args = map[string]any{}
	}
	next := make(map[string]any, len(args)+3)
	for k, v := range args {
		next[k] = v
	}
	if profile.SandboxMode != "" && !hasNonEmptyArg(next, "sandbox_mode") {
		next["sandbox_mode"] = profile.SandboxMode
	}
	// Rule-declared network is a default applied only when the caller did
	// not explicitly set one. If the LLM passed network=true/false, that
	// choice wins.
	if profile.Network {
		if _, ok := next["network"]; !ok {
			next["network"] = true
		}
	}
	if len(profile.WritableRoots) > 0 {
		if _, hasWritableRoots := next["writable_roots"]; !hasWritableRoots {
			next["writable_roots"] = append([]string(nil), profile.WritableRoots...)
		}
	}
	return next
}

func hasNonEmptyArg(args map[string]any, key string) bool {
	raw, ok := args[key]
	if !ok || raw == nil {
		return false
	}
	if s, ok := raw.(string); ok {
		return strings.TrimSpace(s) != ""
	}
	return true
}

func sandboxCapsFromArgs(args map[string]any) []string {
	mode, _ := args["sandbox_mode"].(string)
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "host":
		return []string{core.CapProcessHost}
	}
	var caps []string
	if boolArg(args, "network") {
		caps = append(caps, core.CapNetOutbound)
	}
	if len(stringSliceArg(args, "writable_roots")) > 0 {
		caps = append(caps, core.CapFsWritePath)
	}
	return caps
}

func boolArg(args map[string]any, key string) bool {
	raw, ok := args[key]
	if !ok {
		return false
	}
	v, ok := raw.(bool)
	return ok && v
}

func hasProcessHost(caps []string) bool {
	for _, c := range caps {
		if c == core.CapProcessHost {
			return true
		}
	}
	return false
}
