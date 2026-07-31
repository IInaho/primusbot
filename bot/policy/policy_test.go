package policy

import "testing"

func TestBeginTurnPublishesFacts(t *testing.T) {
	p := New()
	p.Register(Hook{
		Name:  "capture",
		Point: PreModel,
		On: func(state State) *Result {
			facts := state.Facts()
			if facts.Turn.Input != "hello" || !facts.Turn.HasTasks || facts.Turn.TasksDone {
				t.Fatalf("turn facts = %+v", facts.Turn)
			}
			if facts.Turn.ReadsLeft != 8 {
				t.Fatalf("reads left = %d, want 8", facts.Turn.ReadsLeft)
			}
			return nil
		},
	})

	p.BeginTurn(Turn{Input: "hello", HasTasks: true}, 100, 10_000)
	p.BeforeModel(0)
}

func TestRecordToolsPublishesActivity(t *testing.T) {
	p := New()
	var got ActivityFacts
	p.Register(Hook{
		Name:  "capture",
		Point: PostToolBatch,
		On: func(state State) *Result {
			got = state.Facts().Activity
			return nil
		},
	})
	p.BeginTurn(Turn{Input: "task"}, 100, 10_000)
	p.RecordTools([]ToolResult{
		{Name: "task", Args: map[string]any{"type": "researcher"}},
		{Name: "write", Args: map[string]any{"path": "x.go"}},
	})

	if got.ToolCalls != 2 || got.ResearcherCalls != 1 || !got.HasEdits {
		t.Fatalf("activity = %+v", got)
	}
}

func TestFailedAndBlockedEditsDoNotCountAsEdits(t *testing.T) {
	p := New()
	p.BeginTurn(Turn{}, 0, 0)
	p.RecordTools([]ToolResult{
		{Name: "edit", Args: map[string]any{"path": "x.go"}, Error: "anchor not found"},
		{Name: "write", Args: map[string]any{"path": "y.go"}, Blocked: true},
	})

	if got := p.ledger.TurnSnapshot(); got.HasEdits {
		t.Fatalf("activity = %+v, failed edits must not count", got)
	}
}

func TestPolicySnapshotRestorePreservesReads(t *testing.T) {
	p := New()
	p.RecordTool(ToolResult{Name: "read", Args: map[string]any{"path": "main.go"}})
	snapshot := p.Snapshot()

	restored := New()
	restored.Restore(snapshot)
	if !restored.ledger.WasRead("main.go") {
		t.Fatal("restored policy lost read file")
	}
}

func TestBeforeToolEnforcesQuota(t *testing.T) {
	p := New()
	p.BeginTurn(Turn{}, 9_000, 10_000)

	for i := 0; i < 2; i++ {
		if results := p.BeforeTool(ToolRequest{Name: "read"}); len(results) != 0 {
			t.Fatalf("read %d unexpectedly blocked: %+v", i+1, results)
		}
	}
	results := p.BeforeTool(ToolRequest{Name: "read"})
	if len(results) != 1 || results[0].BlockTool == nil {
		t.Fatalf("third read results = %+v, want quota block", results)
	}
}

func TestToolQuotaCountsExploratoryShell(t *testing.T) {
	q := toolQuota{maxSlots: 1}
	if err := q.consumeCall("shell", map[string]any{"command": "cat README.md"}); err != nil {
		t.Fatalf("first exploratory shell command should fit quota: %v", err)
	}
	if err := q.consumeCall("shell", map[string]any{"command": "ls -la"}); err == nil {
		t.Fatal("second exploratory shell command should exceed quota")
	}
}

func TestToolQuotaDoesNotCountVerificationShell(t *testing.T) {
	q := toolQuota{maxSlots: 1}
	if err := q.consumeCall("shell", map[string]any{"command": "go test ./..."}); err != nil {
		t.Fatalf("verification shell command should not consume read quota: %v", err)
	}
	if q.used != 0 {
		t.Fatalf("verification shell command consumed quota: %d", q.used)
	}
}
