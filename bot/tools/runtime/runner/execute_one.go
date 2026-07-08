package runner

import (
	"context"
	"fmt"
	"strings"

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
	tc = e.applySandboxProfile(tc)

	phaseFn, confirmFn, planMode := e.callbacks()
	if phaseFn != nil {
		phaseFn(common.PhaseRunning + " " + tc.Name)
	}
	if planMode {
		return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: "plan mode: blocked"}
	}

	// Predict the escalation request a privileged tool will raise from explicit
	// sandbox arguments. This lets the command approval dialog offer a single
	// "allow and authorize" path without exposing legacy capability args.
	// If a grant already matches, skip the engine's default "ask" prompt.
	predictedReq := e.predictedPermissionRequest(tc.Name, tc.Args)
	if dec := e.evaluatePermission(tc, &tool, predictedReq); dec.block {
		return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: dec.reason}
	} else if dec.prompt {
		reply, ok := e.promptConfirm(tc, tool, confirmFn, predictedReq, dec)
		if !ok {
			e.dropEscalationApproval(tc.ID)
			return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: "cancelled"}
		}
		if reply.Remember {
			toolName := tc.Name
			args := tc.Args
			if dec.rememberTool != "" {
				toolName = dec.rememberTool
				args = dec.rememberArgs
			}
			e.rememberAllowRule(toolName, args, dec.matchedRule)
		}
		if reply.AllowWithPermission && predictedReq != nil {
			e.preApproveEscalation(tc.ID, reply.Remember)
		}
	}

	var errMsg string
	var ok bool
	if tc, errMsg, ok = e.ensureWorkspaceAccess(tc, confirmFn); !ok {
		e.dropEscalationApproval(tc.ID)
		return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: errMsg}
	}

	paths := toolPaths(tc)
	output, execErr := e.callTool(ctx, tool, tc)
	if execErr != nil {
		preApproved, hasPreApproval := e.escalationPreApproved(tc.ID)
		escOut, escOk, escReason, escCause := e.tryPermissionEscalation(ctx, tool, tc, execErr, confirmFn, preApproved, hasPreApproval)
		if escOk {
			e.invalidateMutatedPaths(tc.Name, paths)
			return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Output: formatOutput(tc.Name, escOut)}
		}
		e.dropEscalationApproval(tc.ID)
		return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: permissionFailureMessage(tc, execErr, escReason, escCause)}
	}

	// Successful execution with no escalation: drop any pre-approval token
	// so it cannot be spent by a later call reusing the id (defense in depth
	// alongside the tc.ID-keyed map).
	e.dropEscalationApproval(tc.ID)
	e.invalidateMutatedPaths(tc.Name, paths)
	return core.ToolCallResult{ID: tc.ID, Name: tc.Name, Output: formatOutput(tc.Name, output)}
}

// permissionDecision is the outcome of evaluating the engine for one call.
type permissionDecision struct {
	block        bool
	reason       string
	prompt       bool
	promptTool   string
	promptArgs   map[string]any
	matchedRule  permission.Rule
	rememberTool string
	rememberArgs map[string]any
}

// evaluatePermission runs the rule engine and either blocks, prompts, or
// allows the call. If no explicit rule matched (engine fell to its default
// effect) AND a predicted capability grant already covers the call, we skip
// the basic prompt. Explicit ask rules (rm *, git push *, ...) always win:
// they are command-level safety prompts the user opted into, orthogonal to
// capability grants, so a net.outbound grant MUST NOT silence an "ask rm *".
func (e *Executor) evaluatePermission(tc core.ToolCallItem, tool *core.Tool, predictedReq *core.PermissionRequest) permissionDecision {
	// Non-run shell actions (wait/poll/list/stop) manage existing sessions —
	// they don't execute new commands and shouldn't trigger command-level asks.
	if tc.Name == "shell" && !shellActionIsRun(tc) {
		return permissionDecision{}
	}
	engine, ws, home := e.permissionEngine()
	if cmd, ok := shellCommandForPolicy(tc); ok {
		callInfo := permission.BuildCallInfo("shell", map[string]any{"command": cmd}, ws, home)
		dec := engine.Evaluate("shell", callInfo, defaultPermissionEffect("shell"))
		if decision := e.permissionDecisionForRule(dec, tc, predictedReq); decision.block || decision.prompt {
			decision.promptTool = "shell"
			decision.promptArgs = map[string]any{"command": cmd}
			decision.rememberTool = "shell"
			decision.rememberArgs = map[string]any{"command": cmd}
			return decision
		}
	}

	callInfo := permission.BuildCallInfo(tc.Name, tc.Args, ws, home)
	dec := engine.Evaluate(tc.Name, callInfo, defaultPermissionEffect(tc.Name))
	return e.permissionDecisionForRule(dec, tc, predictedReq)
}

// shellActionIsRun reports whether a shell tool call actually executes a new
// command. Session-management actions (wait/poll/list/stop) should not be
// subject to the command-level permission prompts.
func shellActionIsRun(tc core.ToolCallItem) bool {
	action, _ := tc.Args["action"].(string)
	action = strings.ToLower(strings.TrimSpace(action))
	return action == "" || action == "run"
}

func (e *Executor) permissionDecisionForRule(dec permission.Decision, tc core.ToolCallItem, predictedReq *core.PermissionRequest) permissionDecision {
	switch dec.Effect {
	case permission.EffectDeny:
		reason := "denied by permission rule"
		if dec.Rule.Tool != "" {
			reason = "denied by rule " + dec.Rule.Tool + "(" + dec.Rule.Specifier + ")"
		}
		return permissionDecision{block: true, reason: reason}
	case permission.EffectAsk:
		// Only skip the basic prompt when (a) the engine fell through to its
		// default effect (no explicit rule matched — dec.Rule.Tool == ""),
		// and (b) a predicted capability request is already covered by a grant.
		// Otherwise the user's explicit command-level ask must be honored.
		if predictedReq != nil && dec.Rule.Tool == "" && e.permissionAllowed(tc.Name, *predictedReq) {
			return permissionDecision{}
		}
		return permissionDecision{prompt: true, promptTool: tc.Name, promptArgs: tc.Args, matchedRule: dec.Rule}
	default:
		return permissionDecision{}
	}
}

func shellCommandForPolicy(tc core.ToolCallItem) (string, bool) {
	switch tc.Name {
	case "shell":
		action, _ := tc.Args["action"].(string)
		action = strings.ToLower(strings.TrimSpace(action))
		if action != "" && action != "run" {
			return "", false
		}
		cmd, _ := tc.Args["command"].(string)
		cmd = strings.TrimSpace(cmd)
		return cmd, cmd != ""
	}
	return "", false
}

// permissionEngine returns the configured engine plus workspace/home. The
// default engine is created by NewExecutor and contains builtin rules.
func (e *Executor) permissionEngine() (*permission.Engine, string, string) {
	e.fnMu.RLock()
	defer e.fnMu.RUnlock()
	return e.permEngine, e.permWorkspace, e.permHome
}

// defaultPermissionEffect is the fallback effect when no rule matches. The
// shell tool itself defaults to allow so wait/poll/list do not prompt; shell
// run is evaluated through command-level shell(...) rules before this fallback.
// Unregistered MCP tools (mcp__*) also default to ask — we can't make a
// safety claim about tools we don't know. All other tools default to allow.
func defaultPermissionEffect(toolName string) permission.Effect {
	switch {
	case toolName == "shell":
		return permission.EffectAsk
	case strings.HasPrefix(toolName, "mcp__"):
		return permission.EffectAsk
	}
	return permission.EffectAllow
}

// promptConfirm builds and issues a confirm request for a tool call that the
// permission engine decided needs a prompt.
// Returns (reply, true) if the user allowed, (zero, false) on deny/no-fn.
//
// CanEscalatePermission is only set when the call has a predicted permission
// request. Offering a "允许并授权" button on a call that cannot escalate is
// misleading and used to leave stale pre-approval tokens that could be reused
// elsewhere.
func (e *Executor) promptConfirm(tc core.ToolCallItem, tool core.Tool, confirmFn common.ConfirmFunc, predictedReq *core.PermissionRequest, dec permissionDecision) (common.ConfirmReply, bool) {
	if confirmFn == nil {
		return common.ConfirmReply{}, false
	}
	toolName := dec.promptTool
	if toolName == "" {
		toolName = tc.Name
	}
	args := dec.promptArgs
	if args == nil {
		args = tc.Args
	}
	req := common.NewConfirmRequest(toolName, args, common.ConfirmKindPermission)
	if _, isPrivileged := tool.(core.PrivilegedTool); isPrivileged && predictedReq != nil {
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
	// Build a specifier from the call: shell uses the command prefix, file
	// tools use the path anchor. Fall back to the matched rule's specifier
	// (so a broad ask rule remembered becomes a broad allow).
	spec := matched.Specifier
	if toolName == "shell" {
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
		if resolved, err := resolvePath(p); err == nil {
			if cache := e.state.FileCache; cache != nil {
				cache.Invalidate(resolved)
			}
		}
	}
}
