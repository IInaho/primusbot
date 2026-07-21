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
	patch := "*** Begin Patch\n*** Update File: README.md\n@@\n-old\n+new\n*** End Patch"
	recorder.Record(Event{
		ID:    "evt_3",
		RunID: runID,
		Type:  core.EventToolPreview,
		Time:  now.Add(1500 * time.Millisecond),
		Payload: core.ToolPayload{
			ToolName: "apply_patch",
			Preview:  patch,
		},
	})
	review := "Findings\nSeverity: high\nMissing runtime artifact coverage."
	recorder.Record(Event{
		ID:    "evt_4",
		RunID: runID,
		Type:  core.EventToolCompleted,
		Time:  now.Add(1800 * time.Millisecond),
		Payload: core.ToolPayload{
			ToolName: "review",
			Output:   review,
		},
	})
	recorder.Record(Event{
		ID:    "evt_5",
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
	if lines := strings.Count(strings.TrimSpace(string(eventsData)), "\n") + 1; lines != 5 {
		t.Fatalf("events line count = %d, want 5\n%s", lines, eventsData)
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

	patchData, err := os.ReadFile(filepath.Join(runDir, "artifacts", "patch-002.patch"))
	if err != nil {
		t.Fatalf("read patch: %v", err)
	}
	if string(patchData) != patch {
		t.Fatalf("patch = %q, want %q", patchData, patch)
	}

	reviewData, err := os.ReadFile(filepath.Join(runDir, "artifacts", "review-003.md"))
	if err != nil {
		t.Fatalf("read review: %v", err)
	}
	if string(reviewData) != review {
		t.Fatalf("review = %q, want %q", reviewData, review)
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
	if len(events) != 5 {
		t.Fatalf("loaded events = %d, want 5", len(events))
	}
	store := runstore.NewRunStore(0)
	for _, ev := range events {
		store.Record(ev)
	}
	run, ok := store.RunView(runID)
	if !ok || run.Input != "edit README" || run.Output != "done" {
		t.Fatalf("restored run = %#v ok=%v", run, ok)
	}
	artifact, ok := store.ArtifactView(runID)
	if !ok || len(artifact.Patches) != 1 || len(artifact.Reviews) != 1 {
		t.Fatalf("restored artifacts = %#v ok=%v", artifact, ok)
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
