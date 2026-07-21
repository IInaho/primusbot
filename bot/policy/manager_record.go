package policy

import (
	"nekocode/bot/hooks"
	"nekocode/bot/policy/ledger"
	semanticspkg "nekocode/bot/policy/semantics"
)

func (g *Manager) RecordToolCall(ev ledger.ToolEvent) {
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
		g.HookReg.Inc(hooks.StoreExploreCalls)
	}
	g.HookReg.Inc(hooks.StoreToolPrefix + ev.Name)
	g.HookReg.Inc(hooks.StoreTurnToolCalls)
	mutationApplied := !ev.Blocked && (ev.Name != "write" && ev.Name != "edit" || ev.Error == "")
	if sem.Mutating && mutationApplied {
		g.HookReg.Set(hooks.StoreHasEdits, 1)
		g.HookReg.Set(hooks.PolicyExploreExhausted, 0)
	}
	if ev.Name == "task" {
		if t, _ := ev.Args["type"].(string); t == "researcher" {
			g.HookReg.Inc(hooks.StoreToolResearcher)
		}
	}
}
