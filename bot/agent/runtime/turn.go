package runtime

import (
	"nekocode/bot/agent/runtime/toolrun"
	"nekocode/bot/hooks"
	aggov "nekocode/bot/policy"
	"nekocode/bot/policy/budget"
	"nekocode/bot/tools/runtime/core"
)

type turnRunner struct {
	agent *Agent
}

const policyBlockFinal = "final answer blocked by policy"

func newTurnRunner(agent *Agent) *turnRunner {
	return &turnRunner{agent: agent}
}

func (r *turnRunner) prepareTurn(input string) budget.ToolQuota {
	a := r.agent
	a.deps.ctxMgr.AutoCompactIfNeeded()
	quota := budget.ComputeQuota(a.deps.ctxMgr.TokenUsage())
	r.applyPreTurnHooks(input, quota)
	return quota
}

func (r *turnRunner) applyPreTurnHooks(input string, quota budget.ToolQuota) {
	a := r.agent
	if a.deps.gov == nil || a.deps.gov.HookReg == nil {
		a.applyTurnHints(nil)
		return
	}
	a.deps.gov.ResetTurnBetween(input, aggov.QuotaData{
		MaxSlots: quota.MaxSlots,
		Used:     quota.Used,
	})
	a.deps.gov.HookReg.Flag(hooks.StoreTasksAllDone, a.deps.ctxMgr.AllTasksDone())
	a.deps.gov.HookReg.Flag(hooks.StoreHasTasks, a.deps.ctxMgr.HasTasks())

	var hints []hooks.Hint
	for _, r := range a.deps.gov.HookReg.Evaluate(hooks.PreTurn, "", false) {
		if r.Hint != nil {
			hints = append(hints, *r.Hint)
		}
	}
	a.applyTurnHints(hints)
}

func (r *turnRunner) interruptedBeforeReasoning(callback RunCallback) bool {
	a := r.agent
	a.drainSteering()
	if a.getCtx().Err() == nil {
		return false
	}
	a.run.stopReason = hooks.StopInterrupted
	a.run.lastText = msgInterrupted
	if callback != nil {
		callback("chat", "", "", msgInterrupted)
	}
	return true
}

func (r *turnRunner) retryAfterInterruptedReasoning(reasoning *ReasoningResult, msgCountBefore int) bool {
	a := r.agent
	if !reasoning.Interrupted {
		return false
	}
	if a.life.finished.Load() {
		a.run.stopReason = hooks.StopInterrupted
		return false
	}
	// Count interrupted responses toward the step limit to prevent
	// unbounded loops when the LLM repeatedly produces interrupted output.
	a.run.step++
	a.deps.ctxMgr.TruncateTo(msgCountBefore)
	a.drainSteering()
	return true
}

func (r *turnRunner) handleToolCalls(calls []core.ToolCallItem, reasoning *ReasoningResult, quota *budget.ToolQuota, callback RunCallback) bool {
	a := r.agent
	a.run.consecutiveHints = 0
	a.run.consecutiveFailures = 0
	a.run.gate.Reset()
	return a.toolRunner.ExecuteAndFeedback(calls, reasoning.TextContent, quota, toolrun.Callback(callback))
}

func (r *turnRunner) handleText(reasoning *ReasoningResult, callback RunCallback) (finished bool) {
	a := r.agent
	if reasoning.IsError {
		a.run.consecutiveFailures++
		if a.run.consecutiveFailures >= maxConsecutiveFailures {
			a.run.step++
			a.run.stopReason = hooks.StopCompleted
			a.clearFinalState()
			return true
		}
	} else {
		a.run.consecutiveFailures = 0
	}

	recordable := isRecordableText(reasoning)
	if handled, finished := r.applyPostTurnHooks(reasoning, recordable, callback); handled {
		return finished
	}

	r.completeWithText(reasoning, recordable, callback)
	return true
}

func isRecordableText(reasoning *ReasoningResult) bool {
	return !reasoning.IsError && !reasoning.GarbledToolCall && reasoning.Action == ActionChat
}

func (r *turnRunner) completeWithText(reasoning *ReasoningResult, recordable bool, callback RunCallback) {
	a := r.agent
	a.run.stopReason = hooks.StopCompleted
	a.run.step++
	r.recordReasoningText(reasoning, recordable)
	if callback != nil {
		callback(reasoning.Action.String(), "", "", reasoning.ActionInput)
	}
}

func (r *turnRunner) recordReasoningText(reasoning *ReasoningResult, recordable bool) {
	a := r.agent
	a.run.lastText = reasoning.ActionInput
	if recordable {
		a.deps.ctxMgr.AddAssistantResponse(reasoning.ActionInput, a.stream.lastReason)
		a.run.finalText = reasoning.ActionInput
		a.run.finalPersisted = true
	} else {
		a.run.finalText = ""
		a.run.finalPersisted = false
	}
}

func (r *turnRunner) applyPostTurnHooks(reasoning *ReasoningResult, recordable bool, callback RunCallback) (handled, finished bool) {
	a := r.agent
	if a.deps.gov == nil || a.deps.gov.HookReg == nil {
		return false, false
	}
	if reasoning.GarbledToolCall {
		a.deps.gov.HookReg.Inc(hooks.StoreRespGarbled)
	}
	// Expose the final-answer intent to PostTurn hooks.
	// Only recordable text (non-error, non-garbled chat) is governed.
	if recordable {
		a.deps.gov.HookReg.SetStr(hooks.StoreFinalIntent, hooks.FinalIntentFinal)
	} else {
		a.deps.gov.HookReg.SetStr(hooks.StoreFinalIntent, finalIntentForReasoning(reasoning))
	}

	for _, result := range a.deps.gov.HookReg.Evaluate(hooks.PostTurn, "", false) {
		handled, finished := r.applyPostTurnHookResult(result, reasoning, recordable, callback)
		if handled {
			return true, finished
		}
	}
	return false, false
}

func finalIntentForReasoning(reasoning *ReasoningResult) string {
	switch {
	case reasoning.IsError:
		return hooks.FinalIntentError
	case reasoning.GarbledToolCall:
		return hooks.FinalIntentFormatError
	default:
		return hooks.FinalIntentNonFinal
	}
}

func (r *turnRunner) applyPostTurnHint(reasoning *ReasoningResult, hint *hooks.Hint, recordable bool, callback RunCallback) bool {
	a := r.agent
	a.run.consecutiveHints++
	if a.run.consecutiveHints >= maxConsecutiveHints {
		a.run.step++
		a.run.stopReason = hooks.StopCompleted
		if reasoning.IsError || reasoning.GarbledToolCall {
			a.clearFinalState()
		} else {
			r.recordReasoningText(reasoning, recordable)
		}
		return true
	}
	r.recordReasoningText(reasoning, recordable)
	if recordable {
		if callback != nil {
			callback(reasoning.Action.String(), "", "", reasoning.ActionInput)
		}
	}
	a.injectHint(hint)
	a.run.step++
	return false
}

func (r *turnRunner) applyFinalPolicyBlock(reasoning *ReasoningResult, reason string) bool {
	a := r.agent
	if reason == "" {
		reason = policyBlockFinal
	}
	a.run.lastText = reasoning.ActionInput
	// Keep finalText in sync so finishRun won't fall through to Synthesize
	// (which would otherwise append a spurious synthesized answer on top of
	// the real final text the user already saw). finalPersisted stays false:
	// the blocked text has NOT been appended to the context yet, so finishRun
	// must persist it before returning (otherwise it vanishes on reload).
	a.run.finalText = reasoning.ActionInput
	a.run.finalPersisted = false

	retry, hint := a.run.gate.TryRetry(reason)
	if !retry {
		return false
	}
	a.injectHint(&hooks.Hint{Type: "policy_block", Severity: "critical", Content: hint})
	a.run.step++
	return true
}
