package policy

import (
	"nekocode/bot/hooks"
)

func (g *Manager) ResetTurnBetween(input string, quotaReads int) {
	if g.HookReg == nil {
		return
	}
	g.HookReg.ResetTurn()
	g.HookReg.Set(hooks.StoreQuotaReads, int64(max(0, quotaReads)))
	g.HookReg.Set(hooks.StoreExploreScore, int64(g.Exploration.Score))
	g.HookReg.SetStr(hooks.StoreStepInput, input)
	g.HookReg.Set(hooks.StoreStepInputLen, int64(len([]rune(input))))
	g.SyncLedgerToHooks()
}

func (g *Manager) SyncLedgerToHooks() {
	if g.Ledger == nil || g.HookReg == nil {
		return
	}
	snap := g.Ledger.Snapshot()
	// Stall detection only considers turn-scoped activity. Session-persisted
	// readFiles intentionally do not count, otherwise progress would be reported
	// forever once any file had been read in the session.
	if len(snap.ModifiedFiles) > 0 || len(snap.Verifications) > 0 ||
		len(snap.BlockedTools) > 0 || len(snap.ToolErrors) > 0 ||
		snap.ToolEventCount > 0 {
		g.HookReg.Set(hooks.StoreLedgerProgress, 1)
	} else {
		g.HookReg.Set(hooks.StoreLedgerProgress, 0)
	}
}

func (g *Manager) Reset() {
	if g.Exploration != nil {
		g.Exploration.Reset()
	}
	if g.Ledger != nil {
		g.Ledger.Reset()
	}
	if g.HookReg != nil {
		g.HookReg.ResetSession()
	}
}
