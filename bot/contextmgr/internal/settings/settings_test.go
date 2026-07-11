package settings

import (
	"testing"

	"nekocode/bot/contextmgr/context"
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/contextmgr/token"
)

func TestSetContextWindow(t *testing.T) {
	st := &state.State{Ctx: context.New("test prompt"), Tracker: &token.Tracker{}}
	s := &Store{State: st}

	s.SetContextWindow(10000)
	if st.ContextWindow != 10000 {
		t.Errorf("budget = %d, want 10000", st.ContextWindow)
	}
	s.SetContextWindow(0)
	s.SetContextWindow(-1)
	if st.ContextWindow != 10000 {
		t.Errorf("non-positive budget should not change value: got %d", st.ContextWindow)
	}
}

func TestAllTasksDone_Empty(t *testing.T) {
	st := &state.State{Ctx: context.New("test prompt"), Tracker: &token.Tracker{}}
	if !(&Store{State: st}).AllTasksDone() {
		t.Error("empty todos should be 'done'")
	}
}
