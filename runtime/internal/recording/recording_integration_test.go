package recording

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nekocode/runtime/internal/core"
	"nekocode/runtime/internal/runstore"
)

func TestEventRecorderWritesAndRestoresEvents(t *testing.T) {
	baseDir := t.TempDir()
	recorder, err := NewEventRecorder(baseDir)
	if err != nil {
		t.Fatalf("NewEventRecorder: %v", err)
	}
	runID := core.RunID("run_1")
	now := time.Now()

	recorder.Record(core.Event{
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
	recorder.Record(core.Event{
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
	recorder.Record(core.Event{
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
	recorder.Record(core.Event{
		ID:    "evt_4",
		RunID: runID,
		Type:  core.EventToolCompleted,
		Time:  now.Add(1800 * time.Millisecond),
		Payload: core.ToolPayload{
			ToolName: "review",
			Output:   review,
		},
	})
	recorder.Record(core.Event{
		ID:    "evt_5",
		RunID: runID,
		Type:  core.EventRunDone,
		Time:  now.Add(2 * time.Second),
		Payload: core.RunResult{
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
	run, ok := store.Lookup(runID)
	if !ok || run.Input != "edit README" || run.Output != "done" {
		t.Fatalf("restored run = %#v ok=%v", run, ok)
	}
}

func TestRecordedV1SubAgentPayloadMigratesToV2(t *testing.T) {
	record := recordedEvent{
		Version: "1.0",
		Type:    core.EventSubAgentStarted,
		Payload: json.RawMessage(`{"tool_name":"research","args":"sub_1","output":"3"}`),
	}
	event, err := record.Event()
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := event.Payload.(core.SubAgentPayload)
	if !ok {
		t.Fatalf("payload type = %T", event.Payload)
	}
	if event.Version != core.ProtocolVersion ||
		payload.ID != "sub_1" || payload.Type != "research" || payload.Color != 3 {
		t.Fatalf("migrated event = %#v", event)
	}
}

func TestRecordedV1SessionEventMigratesToV2(t *testing.T) {
	event, err := (recordedEvent{
		Version: "1.0",
		Type:    core.EventType("session_resumed"),
	}).Event()
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != core.EventSessionChanged || event.Version != core.ProtocolVersion {
		t.Fatalf("migrated event = %#v", event)
	}
}
