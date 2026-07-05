package policy

import (
	"testing"

	"nekocode/bot/hooks"
	"nekocode/bot/policy/ledger"
)

func TestGovRecordToolCallUpdatesResearcherAndMutationHooks(t *testing.T) {
	g := NewManager(hooks.NewRegistry())
	g.HookReg.Register(hooks.Hook{
		Name:  "assert-recorded",
		Point: hooks.PreTurn,
		On: func(s hooks.State) *hooks.Result {
			if s.Get(hooks.StoreToolResearcher) != 1 {
				t.Fatalf("researcher count = %d, want 1", s.Get(hooks.StoreToolResearcher))
			}
			if s.Get(hooks.StoreHasEdits) != 1 {
				t.Fatalf("has edits = %d, want 1", s.Get(hooks.StoreHasEdits))
			}
			if s.Get(hooks.StoreTurnToolCalls) != 2 {
				t.Fatalf("turn tool calls = %d, want 2", s.Get(hooks.StoreTurnToolCalls))
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

	g.HookReg.Evaluate(hooks.PreTurn, "", false)
}

func TestGovRecordToolCallDoesNotMarkFailedOrBlockedEditAsEditProgress(t *testing.T) {
	g := NewManager(hooks.NewRegistry())
	var hasEdits int64
	g.HookReg.Register(hooks.Hook{
		Name:  "capture-edits",
		Point: hooks.PreTurn,
		On: func(s hooks.State) *hooks.Result {
			hasEdits = s.Get(hooks.StoreHasEdits)
			return nil
		},
	})

	g.RecordToolCall(ledger.ToolEvent{Name: "edit", Args: map[string]any{"path": "x.go"}, Error: "anchor not found"})
	g.HookReg.Evaluate(hooks.PreTurn, "", false)
	if hasEdits != 0 {
		t.Fatalf("failed edit has_edits=%d, want 0", hasEdits)
	}

	g.RecordToolCall(ledger.ToolEvent{Name: "write", Args: map[string]any{"path": "x.go"}, Blocked: true, BlockText: "blocked"})
	g.HookReg.Evaluate(hooks.PreTurn, "", false)
	if hasEdits != 0 {
		t.Fatalf("blocked write has_edits=%d, want 0", hasEdits)
	}
}

func TestGovRecordToolCallMarksCurrentTurnProgressForSuccessfulEvidence(t *testing.T) {
	g := NewManager(hooks.NewRegistry())

	g.RecordToolCall(ledger.ToolEvent{Name: "read", Args: map[string]any{"path": "x.go"}, Output: "content"})
	g.HookReg.Evaluate(hooks.PreTurn, "", false)
}
