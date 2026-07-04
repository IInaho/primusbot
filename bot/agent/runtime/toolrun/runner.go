package toolrun

import (
	"context"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/hooks"
	aggov "nekocode/bot/policy"
	"nekocode/bot/policy/budget"
	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/runner"
)

type Callback func(action, toolName, toolArgs, output string)

type Host interface {
	Context() context.Context
	ContextManager() *ctxmgr.Manager
	Executor() *runner.Executor
	Governance() *aggov.Manager
	SubSlots() *SlotManager
	InjectHint(*hooks.Hint)
	IncStep()
	ApplyPostToolHookResult(hooks.Result) (stop bool)
}

type Runner struct {
	host Host
}

func New(host Host) *Runner {
	return &Runner{host: host}
}

func (r *Runner) ExecuteAndFeedback(calls []core.ToolCallItem, textContent string, quota *budget.ToolQuota, callback Callback) bool {
	if textContent != "" && callback != nil {
		callback("think", "", "", textContent)
	}

	filtered := r.FilterToolCalls(calls, quota)
	r.host.Executor().PreparePreviews(filtered.Allowed)
	emitStartCallbacks(calls, filtered.Blocked, callback)

	cleanupSubagents := r.prepareSubagentCallbacks(filtered.Allowed, callback)
	defer cleanupSubagents()

	execResults := r.executeAllowedTools(filtered.Allowed, callback)
	results := mergeResults(calls, filtered.Blocked, execResults)
	r.recordToolCalls(calls, filtered.Blocked, results)

	msgs := emitResultCallbacks(calls, filtered.Blocked, results, callback)
	postToolHints := r.evaluatePostToolUseHints(calls, filtered.Blocked, results)
	r.addToolResultsAndHints(calls, msgs, filtered.PreToolHints, postToolHints)

	if r.ApplyPostToolHooks() {
		return true
	}
	r.host.IncStep()
	return false
}

func (r *Runner) ApplyPostToolHooks() bool {
	gov := r.host.Governance()
	if gov == nil || gov.HookReg == nil {
		return false
	}
	for _, result := range gov.HookReg.Evaluate(hooks.PostTool, "", false) {
		if r.host.ApplyPostToolHookResult(result) {
			return true
		}
	}
	return false
}
