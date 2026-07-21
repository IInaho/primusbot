package recording

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafePathPart(t *testing.T) {
	got := safePathPart("../run:1")
	if got != "___run_1" {
		t.Fatalf("safePathPart = %q", got)
	}
}

func TestRecordErrorNilOrEmpty(t *testing.T) {
	var nilRecorder *EventRecorder
	if err := nilRecorder.RecordError(Event{RunID: "run_1"}); err != nil {
		t.Fatalf("nil recorder should not error: %v", err)
	}

	r := &EventRecorder{counts: make(map[RunID]int)}
	if err := r.RecordError(Event{RunID: ""}); err != nil {
		t.Fatalf("empty RunID should not error: %v", err)
	}
}

func TestRecordErrorReportsFailures(t *testing.T) {
	base := t.TempDir()
	r := &EventRecorder{
		sessionID: "test",
		baseDir:   base,
		runDir:    filepath.Join(base, "test"),
		counts:    make(map[RunID]int),
	}

	// Make the run directory unwritable so MkdirAll fails.
	if err := os.MkdirAll(r.runDir, 0o700); err != nil {
		t.Fatalf("setup run dir: %v", err)
	}
	if err := os.Chmod(r.runDir, 0o500); err != nil {
		t.Fatalf("chmod run dir: %v", err)
	}
	defer os.Chmod(r.runDir, 0o700)

	if err := r.RecordError(Event{RunID: "run_1"}); err == nil {
		t.Fatal("expected error for unwritable run dir")
	}
}
