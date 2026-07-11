package usage

import (
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider/types"
)

type Meter struct {
	State *state.State
}

func (u *Meter) Len() int {
	u.State.Mu.RLock()
	defer u.State.Mu.RUnlock()
	n := len(u.State.Ctx.Messages)
	if u.State.Ctx.CompactBoundary > 0 && u.State.Ctx.CompactBoundary < n {
		return n - u.State.Ctx.CompactBoundary
	}
	return n
}

func (u *Meter) Stats() (int, int, bool) {
	u.State.Mu.RLock()
	defer u.State.Mu.RUnlock()
	return len(u.State.Ctx.Messages), u.totalTokenEstimate(), u.State.Ctx.Archive != ""
}

func (u *Meter) RecordUsage(prompt, completion int) {
	u.State.Mu.RLock()
	defer u.State.Mu.RUnlock()
	u.State.Tracker.RecordUsage(prompt, completion)
}

func (u *Meter) RecordCache(hit, miss int) {
	u.State.Mu.RLock()
	defer u.State.Mu.RUnlock()
	u.State.Tracker.RecordCache(hit, miss)
}

func (u *Meter) ResetCache() {
	u.State.Mu.RLock()
	defer u.State.Mu.RUnlock()
	u.State.Tracker.ResetCache()
}

func (u *Meter) TokenUsage() (int, int) {
	u.State.Mu.RLock()
	defer u.State.Mu.RUnlock()
	return u.totalTokenEstimate(), u.State.ContextWindow
}

func (u *Meter) RecordSubagent(tokens, hit, miss int) {
	u.State.Mu.RLock()
	defer u.State.Mu.RUnlock()
	u.State.Tracker.RecordSubagent(tokens, hit, miss)
}

func (u *Meter) CacheStats() (hit, miss int) {
	u.State.Mu.RLock()
	defer u.State.Mu.RUnlock()
	return u.State.Tracker.CacheStats()
}

func (u *Meter) visibleMessages() []types.Message {
	visible := u.State.Ctx.Messages
	if u.State.Ctx.CompactBoundary > 0 && u.State.Ctx.Archive != "" && u.State.Ctx.CompactBoundary < len(visible) {
		visible = visible[u.State.Ctx.CompactBoundary:]
	}
	return visible
}

func (u *Meter) totalTokenEstimate() int {
	return token.EstimateTokens(u.visibleMessages()) + token.EstimateString(u.State.Ctx.Archive)
}
