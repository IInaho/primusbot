package policy

import (
	"testing"

	"nekocode/bot/policy/ledger"
)

func TestGovResetTurnBetweenPublishesQuotaAndInput(t *testing.T) {
	g := New(NewRegistry())
	g.HookReg.Register(Hook{
		Name:  "assert-state",
		Point: PreTurn,
		On: func(s State) *Result {
			if s.Get(StoreQuotaReads) != 5 {
				t.Fatalf("quota reads = %d, want 5", s.Get(StoreQuotaReads))
			}
			if s.Get(StoreStepInputLen) != 5 {
				t.Fatalf("input len = %d, want 5", s.Get(StoreStepInputLen))
			}
			return nil
		},
	})

	g.ResetTurnBetween("hello", 5)

	g.HookReg.Evaluate(PreTurn, "", false)
}

func TestGovResetClearsTrackingState(t *testing.T) {
	g := New(NewRegistry())

	g.Reset()

	if g.Exploration == nil || g.Ledger == nil || g.HookReg == nil {
		t.Fatalf("manager state should be initialized, got %+v", g)
	}
}
func TestGovRecordToolCallUpdatesResearcherAndMutationHooks(t *testing.T) {
	g := New(NewRegistry())
	g.HookReg.Register(Hook{
		Name:  "assert-recorded",
		Point: PreTurn,
		On: func(s State) *Result {
			if s.Get(StoreToolResearcher) != 1 {
				t.Fatalf("researcher count = %d, want 1", s.Get(StoreToolResearcher))
			}
			if s.Get(StoreHasEdits) != 1 {
				t.Fatalf("has edits = %d, want 1", s.Get(StoreHasEdits))
			}
			if s.Get(StoreTurnToolCalls) != 2 {
				t.Fatalf("turn tool calls = %d, want 2", s.Get(StoreTurnToolCalls))
			}
			return nil
		},
	})

	g.RecordToolCall(ledger.ToolEvent{
		Name: "task",
		Args: map[string]any{"type": "researcher"},
	})
	g.RecordToolCall(ledger.ToolEvent{
		Name: "write",
		Args: map[string]any{"path": "x.go"},
	})

	g.HookReg.Evaluate(PreTurn, "", false)
}

func TestGovRecordToolCallDoesNotMarkFailedOrBlockedEditAsEditProgress(t *testing.T) {
	g := New(NewRegistry())
	var hasEdits int64
	g.HookReg.Register(Hook{
		Name:  "capture-edits",
		Point: PreTurn,
		On: func(s State) *Result {
			hasEdits = s.Get(StoreHasEdits)
			return nil
		},
	})

	g.RecordToolCall(ledger.ToolEvent{Name: "edit", Args: map[string]any{"path": "x.go"}, Error: "anchor not found"})
	g.HookReg.Evaluate(PreTurn, "", false)
	if hasEdits != 0 {
		t.Fatalf("failed edit has_edits=%d, want 0", hasEdits)
	}

	g.RecordToolCall(ledger.ToolEvent{Name: "write", Args: map[string]any{"path": "x.go"}, Blocked: true, BlockText: "blocked"})
	g.HookReg.Evaluate(PreTurn, "", false)
	if hasEdits != 0 {
		t.Fatalf("blocked write has_edits=%d, want 0", hasEdits)
	}
}

func TestGovRecordToolCallMarksCurrentTurnProgressForSuccessfulEvidence(t *testing.T) {
	g := New(NewRegistry())

	g.RecordToolCall(ledger.ToolEvent{Name: "read", Args: map[string]any{"path": "x.go"}, Output: "content"})
	g.HookReg.Evaluate(PreTurn, "", false)
}
