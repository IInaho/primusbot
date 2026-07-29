package runtime

import "sync/atomic"

// tokenMeter accumulates token usage for an agent. Prompt tokens are not
// tracked here — the caller passes the live context occupancy from ctxMgr;
// only completion tokens are accumulated, from LLM stream callbacks (hence
// the atomics: streaming callbacks and the loop run concurrently).
type tokenMeter struct {
	prompt     atomic.Int64
	completion atomic.Int64
	promptSnap int64
	complSnap  int64
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
	return contextTokens - int(m.promptSnap), int(m.completion.Load() - m.complSnap)
}

// snapshot records the baseline for turn() at the start of a run.
func (m *tokenMeter) snapshot(contextTokens int) {
	m.promptSnap = int64(contextTokens)
	m.complSnap = m.completion.Load()
}
