package runner

import (
	"context"
	"errors"
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
