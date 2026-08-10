package ledger

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLedgerRecordsDedicatedReadAndModificationTools(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.go")
	created := filepath.Join(dir, "new.go")
	if err := os.WriteFile(source, []byte("package source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(created, []byte("package created\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := New()
	l.RecordTool(ToolEvent{Name: "read", Args: map[string]any{"path": source}})
	l.RecordTool(ToolEvent{Name: "edit", Args: map[string]any{"path": source}})
	l.RecordTool(ToolEvent{Name: "write", Args: map[string]any{"path": created}})

	snap := l.Snapshot()
	if !l.WasRead(source) || !l.WasRead(created) {
		t.Fatalf("read files = %+v", snap.ReadFiles)
	}
	if !reflect.DeepEqual(snap.ModifiedFiles, []string{created, source}) {
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

func TestAuditFromAnotherActorDoesNotAuthorizeWrites(t *testing.T) {
	l := New()
	l.RecordAuditTool(ToolEvent{Name: "read", Args: map[string]any{"path": "shared.go"}})
	l.RecordAuditTool(ToolEvent{Name: "edit", Args: map[string]any{"path": "shared.go"}})

	if l.WasRead("shared.go") {
		t.Fatal("another actor's audit read became local authorization")
	}
	if got := l.Snapshot().ModifiedFiles; !reflect.DeepEqual(got, []string{"shared.go"}) {
		t.Fatalf("aggregate modification audit = %+v", got)
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
	path := filepath.Join(t.TempDir(), "read.go")
	if err := os.WriteFile(path, []byte("package read\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := New()
	l.RecordTool(ToolEvent{Name: "read", Args: map[string]any{"path": path}})
	l.RecordTool(ToolEvent{Name: "edit", Args: map[string]any{"path": path}})
	l.ResetRun()

	if l.WasRead(path) {
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
