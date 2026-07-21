package snapshot

import (
	"testing"

	"nekocode/bot/contextmgr/context"
	"nekocode/bot/contextmgr/internal/history"
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/contextmgr/token"
)

func TestSnapshotRestore(t *testing.T) {
	st := &state.State{Ctx: content.New("test prompt"), Tracker: &token.Tracker{}, ContextWindow: 50000}
	(&history.Store{State: st}).Add("user", "hello world")

	snap := (&Store{State: st}).Snapshot()

	st2 := &state.State{Ctx: content.New("test prompt"), Tracker: &token.Tracker{}}
	(&Store{State: st2}).Restore(snap)

	if got, want := len(st2.Ctx.Messages), len(st.Ctx.Messages); got != want {
		t.Errorf("restored len = %d, want %d", got, want)
	}
	if st2.ContextWindow != 50000 {
		t.Errorf("restored budget = %d, want 50000", st2.ContextWindow)
	}
}
