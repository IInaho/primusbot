// Package policy owns deterministic agent behavior policy. Policy is the only
// runtime entry: it combines the tool-event ledger with hook evaluation. See
// README.md for the package's governance principles and non-goals.
package policy

import (
	"sync"

	"nekocode/bot/policy/ledger"
)

type Policy struct {
	mu sync.Mutex

	hooks  *hookEngine
	ledger *ledger.Ledger

	garbledCount int
}

// New initializes all policy modules with an empty hook set.
func New() *Policy {
	return &Policy{
		hooks:  newHookEngine(),
		ledger: ledger.New(),
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

// ResetRun clears all run policy state, including read authorization evidence.
func (p *Policy) ResetRun() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.garbledCount = 0
	p.mu.Unlock()
	p.ledger.ResetRun()
	p.hooks.reset()
}

// BeforeModel evaluates policy immediately before a model request.
func (p *Policy) BeforeModel() []Result {
	if p == nil {
		return nil
	}
	return p.evaluate(PreModel, ToolFacts{})
}

// BeforeStop records the response outcome and lets policy accept or reject it.
func (p *Policy) BeforeStop(result TurnResult) []Result {
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
	return p.hooks.evaluate(Stop, facts)
}

// OnUserSubmit evaluates hooks after user input enters the run.
func (p *Policy) OnUserSubmit() []Result {
	if p == nil {
		return nil
	}
	return p.evaluate(UserSubmit, ToolFacts{})
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
	p.mu.Lock()
	garbledCount := p.garbledCount
	p.mu.Unlock()
	return Facts{
		Tool:     tool,
		Response: ResponseFacts{GarbledCount: garbledCount},
	}
}
