package usage

import (
	"testing"

	"nekocode/bot/contextmgr/context"
	"nekocode/bot/contextmgr/internal/history"
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/contextmgr/token"
)

func TestLenAndStats(t *testing.T) {
	st := &state.State{Ctx: content.New("test prompt"), Tracker: &token.Tracker{}}
	(&history.Store{State: st}).Add("user", "hello")

	u := &Meter{State: st}
	if n := u.Len(); n != 1 {
		t.Errorf("Len = %d, want 1", n)
	}
	_, tokens, _ := u.Stats()
	if tokens <= 0 {
		t.Error("tokens should be > 0 after adding messages")
	}
}

func TestTokenUsage(t *testing.T) {
	st := &state.State{Ctx: content.New("test prompt"), Tracker: &token.Tracker{}, ContextWindow: 10000}
	_, budget := (&Meter{State: st}).TokenUsage()
	if budget != 10000 {
		t.Errorf("budget = %d, want 10000", budget)
	}
}
