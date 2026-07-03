package runner

import (
	"context"
	"fmt"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/execution"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/common"
)

func (e *Executor) executeOne(ctx context.Context, tc core.ToolCallItem) core.ToolCallResult {
	tool, err := e.registry.Get(tc.Name)
	if err != nil {
		return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: err.Error()}
	}

	phaseFn, confirmFn, planMode := e.callbacks()
	if phaseFn != nil {
		phaseFn(common.PhaseRunning + " " + tc.Name)
	}
	if planMode {
		return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: "plan mode: blocked"}
	}

	engine, ws, home := e.permissionEngine()
	callInfo := permission.BuildCallInfo(tc.Name, tc.Args, ws, home)
	dec := engine.Evaluate(tc.Name, callInfo, defaultPermissionEffect(tc.Name))
	switch dec.Effect {
	case permission.EffectDeny:
		reason := "denied by permission rule"
		if dec.Rule.Tool != "" {
			reason = "denied by rule " + dec.Rule.Tool + "(" + dec.Rule.Specifier + ")"
		}
		return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: reason}
	case permission.EffectAsk:
		reply, ok := e.promptConfirm(tc, tool, confirmFn)
		if !ok {
			return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: "cancelled"}
		}
		if reply.Remember {
			e.rememberAllowRule(tc.Name, tc.Args, dec.Rule)
		}
		if reply.AllowWithPermission {
			e.preApproveEscalation(tc.Name)
		}
	case permission.EffectAllow:
		// run without prompting
	}

	paths := toolPaths(tc)
	output, execErr := e.callTool(ctx, tool, tc)
	if execErr != nil {
		preApproved := e.escalationPreApproved(tc.Name)
		if output, ok := e.tryPermissionEscalation(ctx, tool, tc, execErr, confirmFn, preApproved); ok {
			e.invalidateMutatedPaths(tc.Name, paths)
			return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Output: formatOutput(tc.Name, output)}
		}
		return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: execErr.Error()}
	}

	e.invalidateMutatedPaths(tc.Name, paths)
	return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Output: formatOutput(tc.Name, output)}
}

// permissionEngine returns the configured engine plus workspace/home. The
// default engine is created by NewExecutor and contains builtin rules.
func (e *Executor) permissionEngine() (*permission.Engine, string, string) {
	e.fnMu.RLock()
	defer e.fnMu.RUnlock()
	return e.permEngine, e.permWorkspace, e.permHome
}

func defaultPermissionEffect(toolName string) permission.Effect {
	if toolName == "bash" || toolName == "shell" {
		return permission.EffectAsk
	}
	return permission.EffectAllow
}

// promptConfirm builds and issues a confirm request for a tool call that the
// permission engine decided needs a prompt.
// Returns (reply, true) if the user allowed, (zero, false) on deny/no-fn.
func (e *Executor) promptConfirm(tc core.ToolCallItem, tool core.Tool, confirmFn common.ConfirmFunc) (common.ConfirmReply, bool) {
	if confirmFn == nil {
		return common.ConfirmReply{}, false
	}
	req := common.NewConfirmRequest(tc.Name, confirmArgs(tc.Name, tc.Args), common.ConfirmKindPermission)
	if _, isPrivileged := tool.(core.PrivilegedTool); isPrivileged {
		req.CanEscalatePermission = true
	}
	reply := confirmFn(req)
	if !reply.Allowed {
		return reply, false
	}
	return reply, true
}

// rememberAllowRule persists an allow rule when the user approves an "ask"
// prompt with "remember". It derives the rule specifier from the tool call so
// future matching calls auto-approve, then rebuilds the engine immediately.
func (e *Executor) rememberAllowRule(toolName string, args map[string]any, matched permission.Rule) {
	e.fnMu.RLock()
	store := e.permStore
	ws := e.permWorkspace
	home := e.permHome
	engine := e.permEngine
	decl := e.permDecl
	e.fnMu.RUnlock()
	if store == nil || ws == "" || engine == nil {
		return
	}
	var rules []permission.Rule
	// Build a specifier from the call: bash uses the command prefix, file
	// tools use the path anchor. Fall back to the matched rule's specifier
	// (so a broad ask rule remembered becomes a broad allow).
	spec := matched.Specifier
	if toolName == "bash" {
		if cmd, _ := args["command"].(string); cmd != "" {
			for _, s := range bashRememberSpecs(cmd) {
				rules = append(rules, permission.Rule{Tool: toolName, Specifier: s, Effect: permission.EffectAllow})
			}
		}
	} else if p, _ := args["path"].(string); p != "" {
		spec = pathRememberSpec(p, ws, home)
	}
	if len(rules) == 0 {
		rules = append(rules, permission.Rule{Tool: toolName, Specifier: spec, Effect: permission.EffectAllow})
	}
	for _, rule := range rules {
		_ = store.RememberRule(ws, rule)
	}
	e.rebuildEngine(decl, store, ws)
}

func (e *Executor) callbacks() (common.PhaseFunc, common.ConfirmFunc, bool) {
	e.fnMu.RLock()
	defer e.fnMu.RUnlock()
	return e.phaseFn, e.confirmFn, e.planMode
}

func (e *Executor) callTool(ctx context.Context, tool core.Tool, tc core.ToolCallItem) (output string, execErr error) {
	defer func() {
		if r := recover(); r != nil {
			execErr = fmt.Errorf("panic: %v", r)
		}
	}()
	return tool.Execute(execution.WithExecutionState(ctx, e.state), tc.Args)
}

func (e *Executor) invalidateMutatedPaths(toolName string, paths []string) {
	if toolName != "write" && toolName != "edit" {
		return
	}
	for _, p := range paths {
		if resolved, err := validatePath(p); err == nil {
			if cache := e.state.FileCache; cache != nil {
				cache.Invalidate(resolved)
			}
		}
	}
}
