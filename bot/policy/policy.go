// Package policy owns agent behavior policy. Policy is the only runtime entry:
// it combines exploration scoring, tool-event collection and hook evaluation.
package policy

import (
	"sync"

	"nekocode/bot/policy/budget"
	"nekocode/bot/policy/exploration"
	"nekocode/bot/policy/ledger"
)

type Policy struct {
	mu sync.Mutex

	hooks       *hookEngine
	ledger      *ledger.Ledger
	exploration *exploration.Tracker

	turn           Turn
	quota          budget.ToolQuota
	readsLeft      int
	modelResults   int
	garbledCount   int
	readOnlyStreak int
}

// New initializes all policy modules with an empty hook set.
func New() *Policy {
	return &Policy{
		hooks:       newHookEngine(),
		ledger:      ledger.New(),
		exploration: exploration.New(),
	}
}

// Register installs a hook. A hook with the same name is replaced.
func (p *Policy) Register(hook Hook) {
	if p != nil {
		p.hooks.register(hook)
	}
}

// UnregisterPrefix removes hooks whose names start with prefix.
func (p *Policy) UnregisterPrefix(prefix string) {
	if p != nil {
		p.hooks.unregisterPrefix(prefix)
	}
}

// SetSessionID attaches the current session id to policy audit events.
func (p *Policy) SetSessionID(id string) {
	if p != nil {
		p.hooks.setSessionID(id)
	}
}

// ResetRun clears run policy state while preserving files already read in the
// current persisted session.
func (p *Policy) ResetRun() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.turn = Turn{}
	p.quota = budget.ToolQuota{}
	p.readsLeft = 0
	p.modelResults = 0
	p.garbledCount = 0
	p.readOnlyStreak = 0
	p.mu.Unlock()
	p.exploration.Reset()
	p.ledger.ResetRun()
	p.hooks.reset()
}

// BeginTurn starts one policy turn and evaluates its hooks.
func (p *Policy) BeginTurn(turn Turn, usedTokens, contextWindow int) []Result {
	if p == nil {
		return nil
	}
	p.ledger.BeginTurn()
	p.mu.Lock()
	p.turn = turn
	p.quota = budget.ComputeQuota(usedTokens, contextWindow)
	p.readsLeft = p.quota.MaxSlots
	p.modelResults = 0
	p.mu.Unlock()
	return p.evaluate(PreTurn, ToolFacts{})
}

// BeforeModel evaluates policy immediately before a model request.
func (p *Policy) BeforeModel(toolResultCount int) []Result {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	p.modelResults = toolResultCount
	p.mu.Unlock()
	return p.evaluate(PreModelRequest, ToolFacts{})
}

// AfterTurn records the response outcome and evaluates end-of-turn policy.
func (p *Policy) AfterTurn(result TurnResult) []Result {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	if result.Garbled {
		p.garbledCount++
	}
	p.mu.Unlock()
	facts := p.facts(ToolFacts{})
	facts.Response.Intent = result.Intent
	return p.hooks.evaluate(PostTurn, facts)
}

// OnUserSubmit evaluates hooks after user input enters the run.
func (p *Policy) OnUserSubmit() []Result {
	if p == nil {
		return nil
	}
	return p.evaluate(UserSubmit, ToolFacts{})
}

// OnStop evaluates hooks before the run stops.
func (p *Policy) OnStop() []Result {
	if p == nil {
		return nil
	}
	return p.evaluate(Stop, ToolFacts{})
}

// Snapshot returns the persistable event ledger.
func (p *Policy) Snapshot() ledger.Snapshot {
	if p == nil {
		return ledger.Snapshot{}
	}
	return p.ledger.Snapshot()
}

// Restore replaces the event ledger from a persisted snapshot.
func (p *Policy) Restore(snapshot ledger.Snapshot) {
	if p != nil {
		p.ledger.Restore(snapshot)
	}
}

// Summary returns run-level ledger and hook audit statistics.
func (p *Policy) Summary() string {
	if p == nil {
		return ""
	}
	return p.ledger.Snapshot().Summary() + p.hooks.summary()
}

func (p *Policy) evaluate(point HookPoint, tool ToolFacts) []Result {
	return p.hooks.evaluate(point, p.facts(tool))
}

func (p *Policy) facts(tool ToolFacts) Facts {
	activity := p.ledger.TurnSnapshot()
	p.mu.Lock()
	turn := p.turn
	readsLeft := p.readsLeft
	modelResults := p.modelResults
	garbledCount := p.garbledCount
	readOnlyStreak := p.readOnlyStreak
	p.mu.Unlock()
	return Facts{
		Turn: TurnFacts{
			Input:     turn.Input,
			ReadsLeft: readsLeft,
			HasTasks:  turn.HasTasks,
			TasksDone: turn.TasksDone,
		},
		Tool: tool,
		Activity: ActivityFacts{
			ToolCalls:       activity.ToolCalls,
			ExploreCalls:    activity.ExploreCalls,
			ResearcherCalls: activity.ResearcherCalls,
			HasEdits:        activity.HasEdits,
			HasProgress:     activity.HasProgress,
			ReadOnlyStreak:  readOnlyStreak,
		},
		Exploration: ExplorationFacts{Score: p.exploration.Value()},
		Model:       ModelFacts{ToolResults: modelResults},
		Response:    ResponseFacts{GarbledCount: garbledCount},
	}
}
