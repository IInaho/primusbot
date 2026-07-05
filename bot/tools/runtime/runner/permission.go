package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/execution"
	"nekocode/common"
)

func (e *Executor) tryPermissionEscalation(ctx context.Context, tool core.Tool, tc core.ToolCallItem, execErr error, confirmFn common.ConfirmFunc, preApproved escalationApproval, hasPreApproval bool) (string, bool) {
	var permErr core.PermissionError
	if !errors.As(execErr, &permErr) {
		return "", false
	}
	privileged, ok := tool.(core.PrivilegedTool)
	if !ok {
		return "", false
	}

	req := permErr.PermissionRequest()
	if e.permissionDenied(tc.Name, req) {
		return "", false
	}
	if e.permissionAllowed(tc.Name, req) {
		output, err := privileged.ExecuteWithPermission(execution.WithExecutionState(ctx, e.state), tc.Args, req)
		if err == nil {
			return output, true
		}
		return "", false
	}
	if confirmFn == nil {
		return "", false
	}
	if hasPreApproval {
		if preApproved.remember {
			e.rememberPermission(tc.Name, req)
		}
		output, err := privileged.ExecuteWithPermission(execution.WithExecutionState(ctx, e.state), tc.Args, req)
		return output, err == nil
	}
	confirmReq := common.NewConfirmRequest(tc.Name, permissionConfirmArgs(tc.Args, req), common.ConfirmKindPermission)
	reply := confirmFn(confirmReq)
	if !reply.Allowed {
		return "", false
	}
	if reply.Remember {
		e.rememberPermission(tc.Name, req)
	}
	output, err := privileged.ExecuteWithPermission(execution.WithExecutionState(ctx, e.state), tc.Args, req)
	return output, err == nil
}

func permissionFailureMessage(tc core.ToolCallItem, execErr error) string {
	var permErr core.PermissionError
	if !errors.As(execErr, &permErr) {
		return execErr.Error()
	}
	req := permErr.PermissionRequest()
	if tc.Name != "bash" && tc.Name != "shell" {
		return execErr.Error()
	}
	caps := strings.Join(req.Capabilities, `", "`)
	if caps == "" {
		return execErr.Error()
	}
	if callHasCapabilities(tc.Args, req.Capabilities) {
		return fmt.Sprintf("%s. The bash call already requested capabilities [\"%s\"], but no approval was granted. Wait for and answer the permission prompt; if no prompt appears, this runtime has no confirmation callback for host/sandbox escalation.", execErr.Error(), caps)
	}
	return fmt.Sprintf("%s. To request approval, retry the bash call with capabilities [\"%s\"] in the tool arguments instead of asking in chat.", execErr.Error(), caps)
}

func callHasCapabilities(args map[string]any, required []string) bool {
	if len(required) == 0 {
		return true
	}
	got := stringSliceArg(args, "capabilities")
	for _, need := range required {
		found := false
		for _, cap := range got {
			if cap == need {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func stringSliceArg(args map[string]any, key string) []string {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func (e *Executor) permissionDenied(toolName string, req core.PermissionRequest) bool {
	e.fnMu.RLock()
	store := e.permStore
	e.fnMu.RUnlock()
	if store == nil {
		return false
	}
	_, denied := store.Denied(toolName, req)
	return denied
}

func (e *Executor) permissionAllowed(toolName string, req core.PermissionRequest) bool {
	e.fnMu.RLock()
	store := e.permStore
	e.fnMu.RUnlock()
	if store == nil {
		return false
	}
	_, ok := store.Match(toolName, req)
	return ok
}

func (e *Executor) rememberPermission(toolName string, req core.PermissionRequest) {
	// CapProcessHost must never be persisted — every unsandboxed host
	// execution requires an explicit user prompt, regardless of prior
	// grants. The store layer also enforces this, but we short-circuit
	// here to make the architectural intent explicit.
	for _, cap := range req.Capabilities {
		if cap == core.CapProcessHost {
			return
		}
	}
	e.fnMu.RLock()
	store := e.permStore
	e.fnMu.RUnlock()
	if store == nil {
		return
	}
	_ = store.Allow(toolName, req)
}

func permissionConfirmArgs(args map[string]any, req core.PermissionRequest) map[string]any {
	out := map[string]any{
		"permission_reason":       req.Reason,
		"permission_capabilities": strings.Join(req.Capabilities, ", "),
		"permission_scope":        req.Scope,
	}
	if cmd, ok := args["command"].(string); ok {
		out["command"] = cmd
	}
	for k, v := range req.Details {
		out[k] = v
	}
	return out
}
