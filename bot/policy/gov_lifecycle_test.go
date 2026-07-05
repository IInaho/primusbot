package policy

import (
	"testing"

	"nekocode/bot/hooks"
)

func TestGovResetTurnBetweenPublishesQuotaAndInput(t *testing.T) {
	g := NewManager(hooks.NewRegistry())
	g.HookReg.Register(hooks.Hook{
		Name:  "assert-state",
		Point: hooks.PreTurn,
		On: func(s hooks.State) *hooks.Result {
			if s.Get(hooks.StoreQuotaReads) != 5 {
				t.Fatalf("quota reads = %d, want 5", s.Get(hooks.StoreQuotaReads))
			}
			if s.Get(hooks.StoreStepInputLen) != 5 {
				t.Fatalf("input len = %d, want 5", s.Get(hooks.StoreStepInputLen))
			}
			return nil
		},
	})

	g.ResetTurnBetween("hello", 5)

	g.HookReg.Evaluate(hooks.PreTurn, "", false)
}

func TestGovResetClearsTrackingState(t *testing.T) {
	g := NewManager(hooks.NewRegistry())

	g.Reset()

	if g.Exploration == nil || g.Ledger == nil || g.HookReg == nil {
		t.Fatalf("manager state should be initialized, got %+v", g)
	}
}
