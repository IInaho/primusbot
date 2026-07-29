package runtime

import (
	"nekocode/bot/policy"
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

func (r *toolRunner) executeAndFeedback(calls []core.ToolCallItem, textContent string, callback RunCallback) bool {
	if textContent != "" && callback != nil {
		callback(commonview.StepEvent{Action: commonview.StepActionThink, Output: textContent})
	}

	filtered := r.filterToolCalls(calls)
	r.agent.deps.toolExecutor.PreparePreviews(filtered.Allowed)
	emitStartCallbacks(calls, filtered.Blocked, callback)

	cleanupSubagents := r.prepareSubagentCallbacks(filtered.Allowed, callback)
	defer cleanupSubagents()

	execResults := r.executeAllowedTools(filtered.Allowed, callback)
	results := mergeResults(calls, filtered.Blocked, execResults)
	policyResults := r.recordToolResults(calls, filtered.Blocked, results)

	msgs := emitResultCallbacks(calls, filtered.Blocked, results, callback)
	r.addToolResultsAndHints(calls, msgs, filtered.PreToolHints)

	if r.applyPolicyResults(policyResults) {
		return true
	}
	r.agent.run.step++
	return false
}

func (r *toolRunner) applyPolicyResults(results []policy.Result) bool {
	for _, result := range results {
		if r.agent.applyPostToolHookResult(result) {
			return true
		}
	}
	return false
}
