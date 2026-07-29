package runtime

import (
	"nekocode/bot/policy"
)

const steeringChBuffer = 4

const defaultMaxRetries = 2

// runState holds the mutable state of a single Run.
type runState struct {
	// Progress and termination.
	step       int               // current iteration, capped by maxAgentSteps
	stopReason policy.StopReason // why the run stopped; finishRun branches on it

	// Context rollback point: ctxMgr length at run start; an interrupted run
	// truncates back to here.
	startMsgCount int

	// Final-answer tri-state. lastText is the model's latest text of any
	// kind (including errors) — finishRun's display fallback. finalText is
	// the final-answer candidate (recordable text, or policy-blocked text).
	// finalPersisted tracks whether finalText has already been appended to
	// the context manager (via AddAssistantResponse). finishRun uses this to
	// decide whether the returned FinalOutput still needs persisting — paths
	// like applyFinalPolicyBlock set finalText without persisting, which used
	// to lose the final answer on session reload.
	lastText       string
	finalText      string
	finalPersisted bool

	// Loop-stuck guards: consecutive PostTurn hint injections (capped by
	// maxConsecutiveHints) and consecutive LLM failures (maxConsecutiveFailures).
	consecutiveHints    int
	consecutiveFailures int

	// Hints staged across phases (UserSubmit/PostTool), merged into the next
	// turn's transient hints.
	pendingHints []policy.Hint
}

func (s *runState) reset() {
	*s = runState{stopReason: policy.StopCompleted}
}

// clearFinalState drops the recorded final/last answer, e.g. when the run
// ends without a valid final text (step limit, hook stop, repeated errors).
func (a *Agent) clearFinalState() {
	a.run.lastText = ""
	a.run.finalText = ""
	a.run.finalPersisted = false
}
