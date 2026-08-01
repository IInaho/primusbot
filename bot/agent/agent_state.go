package agent

import (
	"sync/atomic"

	"nekocode/bot/policy"
	"nekocode/protocol"
)

const steeringChBuffer = 4

const defaultMaxRetries = 2

// runState holds the mutable state of a single Run.
type runState struct {
	// Progress and termination.
	step       int               // current iteration, capped by maxAgentSteps
	stopReason policy.StopReason // why the run stopped; finishRun branches on it
	err        error             // unrecoverable preparation failure

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

	// Loop-stuck guards: consecutive Stop hint injections (capped by
	// maxConsecutiveHints) and consecutive LLM failures (maxConsecutiveFailures).
	consecutiveHints    int
	consecutiveFailures int

	// Hints staged across phases (UserSubmit/PostToolBatch), merged into the next
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

// streamState is the agent's fine-grained streaming output channel,
// complementing RunCallback (coarse step events). The three callbacks push
// phase labels, answer-text deltas, and reasoning deltas to the application
// host while the LLM response streams in. lastReason keeps the most recent reasoning
// summary so it can be persisted alongside the assistant message.
type streamState struct {
	phase      protocol.PhaseFunc
	text       func(string)
	reasoning  func(string)
	lastReason string
}

func (s *streamState) resetReasoning() {
	s.lastReason = ""
}

func (s *streamState) emitPhase(phase string) {
	if s.phase != nil {
		s.phase(phase)
	}
}

func (s *streamState) emitText(delta string) {
	if s.text != nil {
		s.text(delta)
	}
}

func (s *streamState) emitReasoning(delta string) {
	if s.reasoning != nil {
		s.reasoning(delta)
	}
}

// tokenMeter accumulates token usage for an agent. Prompt tokens are not
// tracked here — the caller passes the live context occupancy from ctxMgr;
// only completion tokens are accumulated, from LLM stream callbacks (hence
// the atomics: streaming callbacks and the loop run concurrently).
type tokenMeter struct {
	prompt     atomic.Int64
	completion atomic.Int64
	promptSnap atomic.Int64
	complSnap  atomic.Int64
}

func (m *tokenMeter) add(prompt, completion int) {
	m.prompt.Add(int64(prompt))
	m.completion.Add(int64(completion))
}

// total reports cumulative usage: live context occupancy plus all
// completion tokens seen so far.
func (m *tokenMeter) total(contextTokens int) (prompt, completion int) {
	return contextTokens, int(m.completion.Load())
}

// turn reports usage since the last snapshot (i.e. the current run).
func (m *tokenMeter) turn(contextTokens int) (prompt, completion int) {
	return contextTokens - int(m.promptSnap.Load()), int(m.completion.Load() - m.complSnap.Load())
}

// snapshot records the baseline for turn() at the start of a run.
func (m *tokenMeter) snapshot(contextTokens int) {
	m.promptSnap.Store(int64(contextTokens))
	m.complSnap.Store(m.completion.Load())
}
