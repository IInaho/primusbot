package runtime

import (
	"nekocode/bot/hooks"
	"nekocode/common/debug"
	commonview "nekocode/common/view"
)

// maxAgentSteps is the hard ceiling on agent loop iterations. Prevents
// infinite loops when the LLM keeps producing tool calls or PostTurn hooks
// keep injecting hints without progress.
const maxAgentSteps = 150

// maxConsecutiveHints is the maximum number of consecutive PostTurn hint
// injections without any intervening tool calls. Beyond this the agent
// is stuck in a text-only loop and should stop.
const maxConsecutiveHints = 3

// maxConsecutiveFailures is the maximum number of consecutive LLM call
// failures (after retries). Beyond this the LLM is likely broken.
const maxConsecutiveFailures = 5

const msgInterrupted = "Interrupted"

type RunResult struct {
	FinalOutput string
	Error       error
	Steps       int
	Interrupted bool
}

type RunCallback func(ev commonview.StepEvent)

type Loop struct {
	Done             func() bool
	StepLimitReached func() bool
	Step             func() bool
	FinishStep       func()
	EvaluateStop     func()
}

type loopRunner struct {
	agent *Agent
}

func newLoopRunner(agent *Agent) *loopRunner {
	return &loopRunner{agent: agent}
}

func (a *Agent) Run(input string, callback RunCallback) *RunResult {
	return a.loopRunner.run(input, callback)
}

func RunLoop(loop Loop) {
	for !loop.done() {
		if loop.stepLimitReached() {
			break
		}
		if loop.Step() {
			if loop.FinishStep != nil {
				loop.FinishStep()
			}
			break
		}
	}

	if loop.EvaluateStop != nil {
		loop.EvaluateStop()
	}
}

func (loop Loop) done() bool {
	return loop.Done != nil && loop.Done()
}

func (loop Loop) stepLimitReached() bool {
	return loop.StepLimitReached != nil && loop.StepLimitReached()
}

func (r *loopRunner) run(input string, callback RunCallback) *RunResult {
	a := r.agent
	r.startRun(input)
	defer r.logGovernanceSummary()
	r.applyUserSubmitHooks()

	RunLoop(Loop{
		Done:             func() bool { return a.life.finished.Load() },
		StepLimitReached: r.stepLimitReached,
		Step:             func() bool { return r.runTurn(input, callback) },
		FinishStep:       func() { a.life.finished.Store(true) },
		EvaluateStop:     r.evaluateStop,
	})
	result := r.finishRun(callback)
	// Flush any steering messages that arrived too late to be drained during
	// the run, so they are still recorded in the context instead of being
	// lost. Done after finishRun so the interrupted-run truncation cannot
	// cut them away.
	a.drainSteering()
	return result
}

func (r *loopRunner) startRun(input string) {
	a := r.agent
	a.Reset()
	a.run.startMsgCount = a.deps.ctxMgr.Len()
	a.deps.ctxMgr.Add("user", input, "user")
}

func (r *loopRunner) applyUserSubmitHooks() {
	a := r.agent
	if a.deps.gov == nil || a.deps.gov.HookReg == nil {
		return
	}
	for _, r := range a.deps.gov.HookReg.Evaluate(hooks.UserSubmit, "", false) {
		a.injectHint(r.Hint)
	}
}

func (r *loopRunner) logGovernanceSummary() {
	a := r.agent
	if a.deps.gov == nil || a.deps.gov.Ledger == nil {
		return
	}
	snap := a.deps.gov.Ledger.Snapshot()
	hookStats := ""
	if a.deps.gov.HookReg != nil {
		hookStats = a.deps.gov.HookReg.GovernanceStats()
	}
	debug.Log("[GOVERNANCE] task complete: steps=%d, %s%s",
		a.run.step, snap.Summary(), hookStats)
}

func (r *loopRunner) stepLimitReached() bool {
	a := r.agent
	if a.run.step < maxAgentSteps {
		return false
	}
	a.run.stopReason = hooks.StopCompleted
	a.clearFinalState()
	return true
}

func (r *loopRunner) finishRun(callback RunCallback) *RunResult {
	a := r.agent
	if a.getCtx().Err() != nil || a.run.stopReason == hooks.StopInterrupted {
		a.deps.ctxMgr.TruncateTo(a.run.startMsgCount)
		return &RunResult{FinalOutput: msgInterrupted, Steps: a.run.step, Interrupted: true}
	}
	if a.run.stopReason == hooks.StopCompleted {
		if a.run.finalText != "" {
			// Persist the final answer if it hasn't been already. Recordable
			// chat turns set finalPersisted=true; policy-block paths leave it
			// false, so the text the user saw would otherwise be absent from
			// the saved session.
			if !a.run.finalPersisted {
				a.deps.ctxMgr.AddAssistantResponse(a.run.finalText, "")
				a.run.finalPersisted = true
			}
			return &RunResult{FinalOutput: a.run.finalText, Steps: a.run.step}
		}
		if a.run.lastText != "" {
			return &RunResult{FinalOutput: a.run.lastText, Steps: a.run.step}
		}
	}
	output := a.modelRunner.Synthesize()
	a.run.finalPersisted = true
	if callback != nil {
		callback(commonview.StepEvent{Action: commonview.StepActionChat, Output: output})
	}
	return &RunResult{FinalOutput: output, Steps: a.run.step}
}

func (r *loopRunner) evaluateStop() {
	a := r.agent
	if a.deps.gov != nil && a.deps.gov.HookReg != nil {
		for _, result := range a.deps.gov.HookReg.Evaluate(hooks.Stop, "", false) {
			if result.Stop != nil {
				a.run.stopReason = *result.Stop
			}
			if result.Hint != nil {
				a.injectHint(result.Hint)
			}
		}
	}
}

func (r *loopRunner) runTurn(input string, callback RunCallback) (finished bool) {
	a := r.agent
	msgCountBefore := a.deps.ctxMgr.Len()

	quota := a.turnRunner.prepareTurn(input)
	defer a.deps.ctxMgr.SetHints("")

	if a.turnRunner.interruptedBeforeReasoning(callback) {
		return true
	}

	reasoning := a.modelRunner.Reason(input)
	if a.turnRunner.retryAfterInterruptedReasoning(reasoning, msgCountBefore) {
		return false
	}
	if a.run.stopReason == hooks.StopInterrupted {
		return true
	}

	calls := reasoning.ToolCalls
	if len(calls) > 0 {
		if a.turnRunner.handleToolCalls(calls, reasoning, &quota, callback) {
			return true
		}
		return false
	}

	return a.turnRunner.handleText(reasoning, callback)
}
