package ledger

import (
	"testing"

	"nekocode/bot/policy/semantics"
)

func TestLedgerRecordsModificationAndVerification(t *testing.T) {
	l := New()
	l.RecordTool(ToolEvent{
		Name:      "write",
		Args:      map[string]any{"path": "x.go"},
		Semantics: semantics.ClassifyToolCall("write", nil),
	})
	l.RecordTool(ToolEvent{
		Name:      "bash",
		Args:      map[string]any{"command": "go test ./..."},
		Output:    "ok",
		Semantics: semantics.ClassifyToolCall("bash", map[string]any{"command": "go test ./..."}),
	})

	snap := l.Snapshot()
	if !snap.HasModifications() {
		t.Fatal("expected modification")
	}
	if !snap.HasPassingVerification() {
		t.Fatal("expected passing verification")
	}
	if len(snap.Verifications) != 1 || !snap.Verifications[0].Trusted || snap.Verifications[0].ProjectRule {
		t.Fatalf("verification trust = %+v, want trusted direct verification", snap.Verifications)
	}
}

func TestWasRead(t *testing.T) {
	l := New()

	// Not read yet
	if l.WasRead("/tmp/test.go") {
		t.Error("file not read → should return false")
	}

	// Record a read (via a read tool event that has SourceProducing)
	l.RecordTool(ToolEvent{
		Name:      "read",
		Args:      map[string]any{"path": "/tmp/test.go"},
		Semantics: semantics.Semantics{SourceProducing: true},
	})

	if !l.WasRead("/tmp/test.go") {
		t.Error("file was read → should return true")
	}

	// Path cleaning should normalize
	if !l.WasRead("/tmp/foo/../test.go") {
		t.Error("cleaned path should match")
	}

	// Different file not read
	if l.WasRead("/tmp/other.go") {
		t.Error("unrelated file → should return false")
	}
}

func TestResetPreservesSessionReadFiles(t *testing.T) {
	l := New()
	l.RecordTool(ToolEvent{
		Name:      "read",
		Args:      map[string]any{"path": "/tmp/session-read.go"},
		Semantics: semantics.Semantics{SourceProducing: true},
	})

	// Reset is called between turns, but historical reads survive.
	l.Reset()

	if !l.WasRead("/tmp/session-read.go") {
		t.Fatal("Reset must preserve session-scoped readFiles across turns")
	}

	// Turn-scoped counters are still reset.
	snap := l.Snapshot()
	if snap.ToolEventCount != 0 {
		t.Fatalf("ToolEventCount = %d, want 0 after Reset", snap.ToolEventCount)
	}
	if len(snap.ModifiedFiles) != 0 {
		t.Fatalf("ModifiedFiles = %+v, want empty after Reset", snap.ModifiedFiles)
	}
	if len(snap.BlockedTools) != 0 || len(snap.ToolErrors) != 0 || len(snap.Verifications) != 0 {
		t.Fatalf("turn-scoped slices should be empty after Reset: %+v", snap)
	}

	// A new read after Reset is also retained (accumulates).
	l.RecordTool(ToolEvent{
		Name:      "read",
		Args:      map[string]any{"path": "/tmp/second-read.go"},
		Semantics: semantics.Semantics{SourceProducing: true},
	})
	if !l.WasRead("/tmp/session-read.go") || !l.WasRead("/tmp/second-read.go") {
		t.Fatal("both reads should be known after Reset+Record")
	}
}

func TestSuccessfulWriteMarksFileAsKnownContent(t *testing.T) {
	l := New()
	l.RecordTool(ToolEvent{
		Name:      "write",
		Args:      map[string]any{"path": "/tmp/new.go"},
		Semantics: semantics.ClassifyToolCall("write", nil),
	})

	if !l.WasRead("/tmp/new.go") {
		t.Fatal("successful write should mark file as known content")
	}
}

func TestRestorePreservesReadAndModifiedFiles(t *testing.T) {
	l := New()
	l.Restore(Snapshot{
		ReadFiles:      []string{"/tmp/../tmp/read.go"},
		ModifiedFiles:  []string{"./main.go"},
		ToolEventCount: 3,
	})

	if !l.WasRead("/tmp/read.go") {
		t.Fatal("restored read file should be known")
	}
	snap := l.Snapshot()
	if len(snap.ModifiedFiles) != 1 || snap.ModifiedFiles[0] != "main.go" {
		t.Fatalf("modified files = %+v, want main.go", snap.ModifiedFiles)
	}
	if snap.ToolEventCount != 3 {
		t.Fatalf("tool event count = %d, want 3", snap.ToolEventCount)
	}
}

func TestFailedWriteDoesNotMarkFileAsKnownContent(t *testing.T) {
	l := New()
	l.RecordTool(ToolEvent{
		Name:      "write",
		Args:      map[string]any{"path": "/tmp/new.go"},
		Error:     "write failed",
		Semantics: semantics.ClassifyToolCall("write", nil),
	})

	if l.WasRead("/tmp/new.go") {
		t.Fatal("failed write should not mark file as known content")
	}
}

func TestLedgerRecordsBashReadPaths(t *testing.T) {
	l := New()
	l.RecordTool(ToolEvent{
		Name:      "bash",
		Args:      map[string]any{"command": "cat bot/agent/ledger/ledger.go"},
		Semantics: semantics.ClassifyToolCall("bash", map[string]any{"command": "cat bot/agent/ledger/ledger.go"}),
	})
	l.RecordTool(ToolEvent{
		Name:      "bash",
		Args:      map[string]any{"command": "rg -n WasRead bot/agent/ledger/ledger_test.go"},
		Semantics: semantics.ClassifyToolCall("bash", map[string]any{"command": "rg -n WasRead bot/agent/ledger/ledger_test.go"}),
	})

	if !l.WasRead("bot/agent/ledger/ledger.go") {
		t.Fatal("cat path should be recorded as read")
	}
	if !l.WasRead("bot/agent/ledger/ledger_test.go") {
		t.Fatal("rg path should be recorded as read")
	}

	l.RecordTool(ToolEvent{
		Name:      "bash",
		Args:      map[string]any{"command": "cd bot && cat policy/ledger/ledger.go"},
		Semantics: semantics.ClassifyToolCall("bash", map[string]any{"command": "cd bot && cat policy/ledger/ledger.go"}),
	})
	if !l.WasRead("policy/ledger/ledger.go") {
		t.Fatal("cat after cd should be recorded as read")
	}
}

func TestLedgerRecordsBashWritePaths(t *testing.T) {
	cases := []struct {
		cmd  string
		path string
	}{
		{"go test ./... > test.out", "test.out"},
		{"go test ./...>test.out", "test.out"},
		{"go test ./... >| test.out", "test.out"},
		{"go test ./... >|test.out", "test.out"},
		{"go test ./... | tee test.out", "test.out"},
		{"go test ./...&&touch marker.txt", "marker.txt"},
		{"sed -i 's/a/b/' main.go", "main.go"},
		{"touch marker.txt", "marker.txt"},
		{"touch README", "README"},
		{"rm main.go", "main.go"},
		{"mkdir generated", "generated"},
	}
	for _, c := range cases {
		l := New()
		l.RecordTool(ToolEvent{
			Name:      "bash",
			Args:      map[string]any{"command": c.cmd},
			Semantics: semantics.ClassifyToolCall("bash", map[string]any{"command": c.cmd}),
		})
		snap := l.Snapshot()
		found := false
		for _, p := range snap.ModifiedFiles {
			if p == c.path {
				found = true
			}
		}
		if !found {
			t.Fatalf("%q modified files = %+v, want %q", c.cmd, snap.ModifiedFiles, c.path)
		}
	}
}

func TestLedgerDoesNotRecordFailedEditAsModified(t *testing.T) {
	l := New()
	l.RecordTool(ToolEvent{
		Name:      "edit",
		Args:      map[string]any{"path": "main.go"},
		Error:     "anchor not found",
		Semantics: semantics.ClassifyToolCall("edit", nil),
	})
	if snap := l.Snapshot(); len(snap.ModifiedFiles) != 0 {
		t.Fatalf("failed edit modified files = %+v, want none", snap.ModifiedFiles)
	}
}

func TestLedgerIgnoresDeviceWritePaths(t *testing.T) {
	l := New()
	cmd := "go test ./... > /dev/null"
	l.RecordTool(ToolEvent{
		Name:      "bash",
		Args:      map[string]any{"command": cmd},
		Semantics: semantics.ClassifyToolCall("bash", map[string]any{"command": cmd}),
	})
	if snap := l.Snapshot(); len(snap.ModifiedFiles) != 0 {
		t.Fatalf("modified files = %+v, want none for device redirect", snap.ModifiedFiles)
	}
}

func TestLedgerRecordsOnlyCopyDestinationAsModified(t *testing.T) {
	l := New()
	cmd := "cp src dst"
	l.RecordTool(ToolEvent{
		Name:      "bash",
		Args:      map[string]any{"command": cmd},
		Semantics: semantics.ClassifyToolCall("bash", map[string]any{"command": cmd}),
	})
	snap := l.Snapshot()
	if len(snap.ModifiedFiles) != 1 || snap.ModifiedFiles[0] != "dst" {
		t.Fatalf("modified files = %+v, want only dst", snap.ModifiedFiles)
	}
}

func TestLedgerSkipsChmodModeOperand(t *testing.T) {
	l := New()
	cmd := "chmod 644 main.go"
	l.RecordTool(ToolEvent{
		Name:      "bash",
		Args:      map[string]any{"command": cmd},
		Semantics: semantics.ClassifyToolCall("bash", map[string]any{"command": cmd}),
	})
	snap := l.Snapshot()
	if len(snap.ModifiedFiles) != 1 || snap.ModifiedFiles[0] != "main.go" {
		t.Fatalf("modified files = %+v, want only main.go", snap.ModifiedFiles)
	}
}
