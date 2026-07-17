package recording

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nekocode/runtime/internal/core"
	"nekocode/runtime/internal/runstore"
)

func TestEventRecorderWritesEventsAndArtifacts(t *testing.T) {
	baseDir := t.TempDir()
	recorder, err := NewEventRecorder(baseDir)
	if err != nil {
		t.Fatalf("NewEventRecorder: %v", err)
	}
	runID := RunID("run_1")
	now := time.Now()

	recorder.Record(Event{
		ID:     "evt_1",
		RunID:  runID,
		Type:   core.EventInputAccepted,
		Time:   now,
		Source: core.SourceRef{Kind: "telegram"},
		Payload: core.MessagePayload{
			Content: "edit README",
		},
	})
	diff := "--- a/README.md\n+++ b/README.md\n@@\n-old\n+new"
	recorder.Record(Event{
		ID:    "evt_2",
		RunID: runID,
		Type:  core.EventToolPreview,
		Time:  now.Add(time.Second),
		Payload: core.ToolPayload{
			ToolName: "edit",
			Preview:  diff,
		},
	})
	recorder.Record(Event{
		ID:    "evt_3",
		RunID: runID,
		Type:  core.EventRunDone,
		Time:  now.Add(2 * time.Second),
		Payload: core.DonePayload{
			Output: "done",
		},
	})

	runDir := recorder.RunDir(runID)
	eventsData, err := os.ReadFile(filepath.Join(runDir, "events.jsonl"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if lines := strings.Count(strings.TrimSpace(string(eventsData)), "\n") + 1; lines != 3 {
		t.Fatalf("events line count = %d, want 3\n%s", lines, eventsData)
	}
	if !strings.Contains(string(eventsData), `"payload_type":"runtime.MessagePayload"`) {
		t.Fatalf("payload type not recorded:\n%s", eventsData)
	}

	diffData, err := os.ReadFile(filepath.Join(runDir, "artifacts", "diff-001.patch"))
	if err != nil {
		t.Fatalf("read diff: %v", err)
	}
	if string(diffData) != diff {
		t.Fatalf("diff = %q, want %q", diffData, diff)
	}

	resultData, err := os.ReadFile(filepath.Join(runDir, "artifacts", "result.md"))
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if string(resultData) != "done" {
		t.Fatalf("result = %q", resultData)
	}

	events, err := LoadRecordedEvents(baseDir)
	if err != nil {
		t.Fatalf("LoadRecordedEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("loaded events = %d, want 3", len(events))
	}
	store := runstore.NewRunStore(0)
	for _, ev := range events {
		store.Record(ev)
	}
	run, ok := store.RunView(runID)
	if !ok || run.Input != "edit README" || run.Output != "done" {
		t.Fatalf("restored run = %#v ok=%v", run, ok)
	}
}

func TestEventRecorderWritesFailedArtifacts(t *testing.T) {
	recorder, err := NewEventRecorder(t.TempDir())
	if err != nil {
		t.Fatalf("NewEventRecorder: %v", err)
	}
	recorder.Record(Event{
		ID:    "evt_1",
		RunID: "run_failed",
		Type:  core.EventRunFailed,
		Time:  time.Now(),
		Payload: core.DonePayload{
			Output: "partial",
			Error:  "boom",
		},
	})

	runDir := recorder.RunDir("run_failed")
	if data, err := os.ReadFile(filepath.Join(runDir, "artifacts", "partial-result.md")); err != nil || string(data) != "partial" {
		t.Fatalf("partial result = %q err=%v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(runDir, "artifacts", "error.txt")); err != nil || string(data) != "boom" {
		t.Fatalf("error artifact = %q err=%v", data, err)
	}
}
