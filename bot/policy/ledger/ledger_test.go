package ledger

import (
	"reflect"
	"testing"
)

func TestLedgerRecordsDedicatedReadAndModificationTools(t *testing.T) {
	l := New()
	l.RecordTool(ToolEvent{Name: "read", Args: map[string]any{"path": "/tmp/source.go"}})
	l.RecordTool(ToolEvent{Name: "edit", Args: map[string]any{"path": "/tmp/source.go"}})
	l.RecordTool(ToolEvent{Name: "write", Args: map[string]any{"path": "/tmp/new.go"}})

	snap := l.Snapshot()
	if !l.WasRead("/tmp/source.go") || !l.WasRead("/tmp/new.go") {
		t.Fatalf("read files = %+v", snap.ReadFiles)
	}
	if !reflect.DeepEqual(snap.ModifiedFiles, []string{"/tmp/new.go", "/tmp/source.go"}) {
		t.Fatalf("modified files = %+v", snap.ModifiedFiles)
	}
}

func TestLedgerDoesNotInferFactsFromShellCommandText(t *testing.T) {
	l := New()
	l.RecordTool(ToolEvent{Name: "shell", Args: map[string]any{
		"command": "cat main.go && go test ./... > test.out",
	}})

	snap := l.Snapshot()
	if l.WasRead("main.go") || len(snap.ModifiedFiles) != 0 {
		t.Fatalf("shell text became inferred evidence: %+v", snap)
	}
}

func TestFailedAndBlockedToolsOnlyRecordTheirConcreteOutcome(t *testing.T) {
	l := New()
	l.RecordTool(ToolEvent{Name: "read", Args: map[string]any{"path": "missing.go"}, Error: "not found"})
	l.RecordTool(ToolEvent{Name: "write", Args: map[string]any{"path": "blocked.go"}, Blocked: true, BlockText: "denied"})

	snap := l.Snapshot()
	if l.WasRead("missing.go") || len(snap.ModifiedFiles) != 0 {
		t.Fatalf("failed/blocked calls changed file evidence: %+v", snap)
	}
	if len(snap.ToolErrors) != 1 || len(snap.BlockedTools) != 1 || snap.ToolEventCount != 2 {
		t.Fatalf("outcome evidence = %+v", snap)
	}
}

func TestResetRunClearsPriorReadAuthorization(t *testing.T) {
	l := New()
	l.RecordTool(ToolEvent{Name: "read", Args: map[string]any{"path": "/tmp/read.go"}})
	l.RecordTool(ToolEvent{Name: "edit", Args: map[string]any{"path": "/tmp/read.go"}})
	l.ResetRun()

	if l.WasRead("/tmp/read.go") {
		t.Fatal("read from an earlier run remained trusted")
	}
	snap := l.Snapshot()
	if len(snap.ModifiedFiles) != 0 || snap.ToolEventCount != 0 {
		t.Fatalf("run evidence survived reset: %+v", snap)
	}
}

func TestRestoreDoesNotTrustPersistedReads(t *testing.T) {
	l := New()
	l.Restore(Snapshot{
		ReadFiles:      []string{"z.go", "dir/../a.go"},
		ModifiedFiles:  []string{"b.go", "a.go"},
		ToolEventCount: 3,
	})

	snap := l.Snapshot()
	if len(snap.ReadFiles) != 0 {
		t.Fatalf("persisted reads were restored as trusted evidence: %+v", snap.ReadFiles)
	}
	if !reflect.DeepEqual(snap.ModifiedFiles, []string{"a.go", "b.go"}) || snap.ToolEventCount != 3 {
		t.Fatalf("restored snapshot = %+v", snap)
	}
}
