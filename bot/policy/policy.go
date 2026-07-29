// Package policy implements the agent behavior-governance layer: a hook
// engine (Registry) that evaluates lifecycle events, plus governance
// semantics — tool-call instrumentation (ledger), exploration budgeting,
// and hook-point signals — all held by a single Policy handle.
//
// Policy is the package entry point; ledger/budget/semantics/builtin/plugin
// are the supporting subpackages.
package policy

import (
	"nekocode/bot/policy/budget"
	"nekocode/bot/policy/ledger"
	semanticspkg "nekocode/bot/policy/semantics"
)

// Policy owns agent governance state: the hooks registry it reports into,
// the tool-event ledger, and the exploration budget tracker.
type Policy struct {
	HookReg     *Registry
	Ledger      *ledger.Ledger
	Exploration *budget.ExplorationTracker
}

// New creates a Policy reporting into hookReg (nil tolerated).
func New(hookReg *Registry) *Policy {
	return &Policy{
		HookReg:     hookReg,
		Ledger:      ledger.New(),
		Exploration: budget.NewExplorationTracker(),
	}
}

func (g *Policy) ResetTurnBetween(input string, quotaReads int) {
	if g.HookReg == nil {
		return
	}
	g.HookReg.ResetTurn()
	g.HookReg.Set(StoreQuotaReads, int64(max(0, quotaReads)))
	g.HookReg.Set(StoreExploreScore, g.Exploration.Score.Load())
	g.HookReg.SetStr(StoreStepInput, input)
	g.HookReg.Set(StoreStepInputLen, int64(len([]rune(input))))
	g.SyncLedgerToHooks()
}

func (g *Policy) SyncLedgerToHooks() {
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
		g.HookReg.Set(StoreLedgerProgress, 1)
	} else {
		g.HookReg.Set(StoreLedgerProgress, 0)
	}
}

func (g *Policy) Reset() {
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

func (g *Policy) RecordToolCall(ev ledger.ToolEvent) {
	sem := semanticspkg.ClassifyToolCall(ev.Name, ev.Args)
	ev.Semantics = sem

	if g.Ledger != nil {
		g.Ledger.RecordTool(ev)
	}

	g.Exploration.RecordCall(ev.Name, ev.Args)
	if g.HookReg == nil {
		return
	}
	if sem.Exploratory {
		g.HookReg.Inc(StoreExploreCalls)
	}
	g.HookReg.Inc(StoreToolPrefix + ev.Name)
	g.HookReg.Inc(StoreTurnToolCalls)
	mutationApplied := !ev.Blocked && (ev.Name != "write" && ev.Name != "edit" || ev.Error == "")
	if sem.Mutating && mutationApplied {
		g.HookReg.Set(StoreHasEdits, 1)
		g.HookReg.Set(PolicyExploreExhausted, 0)
	}
	if ev.Name == "task" {
		if t, _ := ev.Args["type"].(string); t == "researcher" {
			g.HookReg.Inc(StoreToolResearcher)
		}
	}
}
