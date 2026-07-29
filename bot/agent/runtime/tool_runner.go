package runtime

import (
	"slices"

	"nekocode/bot/policy"
	"nekocode/bot/policy/budget"
	"nekocode/bot/tools/runtime/core"
	commonview "nekocode/common/view"
)

// toolRunner orchestrates tool execution within a turn: filtering, execution,
// result feedback, and post-tool hooks.
type toolRunner struct {
	agent *Agent
}

func newToolRunner(agent *Agent) *toolRunner {
	return &toolRunner{agent: agent}
}

func (r *toolRunner) executeAndFeedback(calls []core.ToolCallItem, textContent string, quota *budget.ToolQuota, callback RunCallback) bool {
	if textContent != "" && callback != nil {
		callback(commonview.StepEvent{Action: commonview.StepActionThink, Output: textContent})
	}

	filtered := r.filterToolCalls(calls, quota)
	r.agent.deps.toolExecutor.PreparePreviews(filtered.Allowed)
	emitStartCallbacks(calls, filtered.Blocked, callback)

	cleanupSubagents := r.prepareSubagentCallbacks(filtered.Allowed, callback)
	defer cleanupSubagents()

	execResults := r.executeAllowedTools(filtered.Allowed, callback)
	results := mergeResults(calls, filtered.Blocked, execResults)
	r.recordToolCalls(calls, filtered.Blocked, results)

	msgs := emitResultCallbacks(calls, filtered.Blocked, results, callback)
	postToolHints := r.evaluatePostToolUseHints(calls, filtered.Blocked, results)
	r.addToolResultsAndHints(calls, msgs, filtered.PreToolHints, postToolHints)

	if r.applyPostToolHooks() {
		return true
	}
	r.agent.run.step++
	return false
}

func (r *toolRunner) applyPostToolHooks() bool {
	gov := r.agent.deps.gov
	if gov == nil || gov.HookReg == nil {
		return false
	}
	return slices.ContainsFunc(gov.HookReg.Evaluate(policy.PostTool, "", false), func(result policy.Result) bool {
		return r.agent.applyPostToolHookResult(result)
	})
}
