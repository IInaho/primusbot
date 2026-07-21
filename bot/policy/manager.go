package policy

import (
	"nekocode/bot/hooks"
	"nekocode/bot/policy/budget"
	"nekocode/bot/policy/ledger"
)

type Manager struct {
	HookReg     *hooks.Registry
	Ledger      *ledger.Ledger
	Exploration *budget.ExplorationTracker
}

func NewManager(hookReg *hooks.Registry) *Manager {
	return &Manager{
		HookReg:     hookReg,
		Ledger:      ledger.New(),
		Exploration: budget.NewExplorationTracker(),
	}
}
