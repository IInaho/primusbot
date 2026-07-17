package runstore

import (
	"testing"
	"time"

	"nekocode/runtime/internal/core"
)

func TestRunStoreBuildsRunAndArtifactViews(t *testing.T) {
	store := NewRunStore(0)
	runID := RunID("run_1")
	now := time.Now()

	store.Record(Event{
		RunID: runID,
		Type:  core.EventInputAccepted,
		Time:  now,
		Payload: core.MessagePayload{
			Content: "edit README",
			Source:  core.SourceRef{Kind: "telegram", ID: "chat"},
			Sender:  core.SenderRef{Username: "alice"},
		},
	})
	store.Record(Event{
		RunID: runID,
		Type:  core.EventToolStarted,
		Time:  now.Add(time.Second),
		Payload: core.ToolPayload{
			ToolName: "edit",
			Args:     `{"path":"README.md"}`,
		},
	})
	diff := "--- a/README.md\n+++ b/README.md\n@@\n-old\n+new"
	store.Record(Event{
		RunID: runID,
		Type:  core.EventToolPreview,
		Time:  now.Add(2 * time.Second),
		Payload: core.ToolPayload{
			ToolName: "edit",
			Preview:  diff,
		},
	})
	store.Record(Event{
		RunID: runID,
		Type:  core.EventRunDone,
		Time:  now.Add(3 * time.Second),
		Payload: core.DonePayload{
			Output: "done",
		},
	})

	view, ok := store.RunView(runID)
	if !ok {
		t.Fatal("RunView missing")
	}
	if view.Status != core.RunDone {
		t.Fatalf("status = %s, want %s", view.Status, core.RunDone)
	}
	if view.Input != "edit README" || view.Source.Kind != "telegram" || view.Sender.Username != "alice" {
		t.Fatalf("input/source/sender not captured: %#v", view)
	}
	if len(view.Tools) != 1 || view.Tools[0].Preview != diff {
		t.Fatalf("tool preview not captured: %#v", view.Tools)
	}
	if view.Output != "done" || view.FinishedAt == nil {
		t.Fatalf("done output not captured: %#v", view)
	}

	artifact, ok := store.ArtifactView(runID)
	if !ok {
		t.Fatal("ArtifactView missing")
	}
	if len(artifact.Diffs) != 1 || artifact.Diffs[0].Content != diff {
		t.Fatalf("diff artifact not captured: %#v", artifact.Diffs)
	}
	if len(artifact.Results) != 1 || artifact.Results[0].Content != "done" {
		t.Fatalf("result artifact not captured: %#v", artifact.Results)
	}
}

func TestRunStoreCopiesViews(t *testing.T) {
	store := NewRunStore(0)
	store.Record(Event{
		RunID:   "run_1",
		Type:    core.EventToolStarted,
		Time:    time.Now(),
		Payload: core.ToolPayload{ToolName: "shell"},
	})
	first, ok := store.RunView("run_1")
	if !ok {
		t.Fatal("RunView missing")
	}
	first.Tools[0].Name = "changed"
	second, _ := store.RunView("run_1")
	if second.Tools[0].Name != "shell" {
		t.Fatalf("RunView returned mutable slice: %#v", second.Tools)
	}
}
