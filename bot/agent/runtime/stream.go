package runtime

import "nekocode/bot/view"

// StreamCallback receives answer-text deltas as the LLM response streams in.
// isToolCall is true when the delta belongs to a tool-call payload.
type StreamCallback func(delta string, isToolCall bool)

// ReasoningCallback receives reasoning (chain-of-thought) deltas.
type ReasoningCallback func(delta string)

// streamState is the agent's fine-grained streaming output channel,
// complementing RunCallback (coarse step events). The three callbacks push
// phase labels, answer-text deltas, and reasoning deltas to the UI while the
// LLM response streams in. lastReason keeps the most recent reasoning
// summary so it can be persisted alongside the assistant message.
type streamState struct {
	phase      view.PhaseFunc
	text       StreamCallback
	reasoning  ReasoningCallback
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
		s.text(delta, false)
	}
}

func (s *streamState) emitReasoning(delta string) {
	if s.reasoning != nil {
		s.reasoning(delta)
	}
}
