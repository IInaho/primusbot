package runtime

import "nekocode/bot/hooks"

func (a *Agent) applyPostToolHookResult(result hooks.Result) bool {
	if result.Stop != nil {
		a.run.stopReason = *result.Stop
		a.clearFinalState()
		return true
	}
	if result.RequireTool != nil {
		a.injectHint(&hooks.Hint{
			Type:     "require_tool",
			Severity: "critical",
			Content:  policyRequireTool(result.RequireTool.Tool, result.RequireTool.Reason),
		})
	}
	if result.BlockFinal != nil {
		a.injectHint(&hooks.Hint{Type: "block_final", Severity: "critical", Content: result.BlockFinal.Reason})
	}
	a.injectHint(result.Hint)
	return false
}

func (r *turnRunner) applyPostTurnHookResult(result hooks.Result, reasoning *ReasoningResult, recordable bool, callback RunCallback) (handled, finished bool) {
	a := r.agent
	if result.Stop != nil {
		a.run.stopReason = *result.Stop
		r.recordReasoningText(reasoning, recordable)
		return true, true
	}
	if result.BlockFinal != nil {
		if r.applyFinalPolicyBlock(reasoning, result.BlockFinal.Reason) {
			return true, false
		}
		return false, false
	}
	if result.RequireTool != nil {
		reason := policyRequireTool(result.RequireTool.Tool, result.RequireTool.Reason)
		if r.applyFinalPolicyBlock(reasoning, reason) {
			return true, false
		}
		return false, false
	}
	if result.Hint != nil {
		return true, r.applyPostTurnHint(reasoning, result.Hint, recordable, callback)
	}
	return false, false
}

func policyRequireTool(tool, reason string) string {
	if tool != "" {
		return "必须先调用 " + tool + "：" + reason
	}
	return reason
}

func (a *Agent) clearFinalState() {
	a.run.lastText = ""
	a.run.finalText = ""
	a.run.finalPersisted = false
}
