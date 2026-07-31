package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"nekocode/runtime/internal/connectors"
	"nekocode/runtime/internal/recording"
)

func TestManagerRedactsSensitiveInputEvents(t *testing.T) {
	rt := newTestRuntime(&testBot{})
	runID, err := rt.StartRun(context.Background(), Input{
		Source: SourceRef{Kind: "test"},
		Text:   "/connect telegram token 123456:super-secret",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForRun(t, rt, runID)
	run, ok := rt.LookupRun(runID)
	if !ok {
		t.Fatal("run snapshot missing")
	}
	if strings.Contains(run.Input, "super-secret") || !strings.Contains(run.Input, "[redacted]") {
		t.Fatalf("run input was not redacted: %q", run.Input)
	}
}

func TestManagerDisconnectCommandDoesNotDuplicateConnectorStatus(t *testing.T) {
	rt := newTestRuntime(&testBot{})
	rt.RegisterConnector("telegram", func(runtime ConnectorRuntime) connectors.Connector {
		return statusPublishingConnector{rt: runtime}
	})
	if _, err := rt.Connect(context.Background(), "telegram", nil); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	runID, err := rt.StartRun(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "/disconnect telegram"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForRun(t, rt, runID)

	for _, ev := range rt.events.History(EventFilter{RunID: runID}) {
		if ev.Type == EventSystemMessage {
			t.Fatalf("disconnect command should rely on connector status event, got system message: %#v", ev.Payload)
		}
	}

	var statusMessages int
	for _, ev := range rt.events.History(EventFilter{}) {
		if ev.Type == EventConnectorStatus {
			statusMessages++
		}
	}
	if statusMessages != 1 {
		t.Fatalf("connector status events = %d, want 1", statusMessages)
	}
}

func TestManagerEventRecordingAdvancesRunSequence(t *testing.T) {
	baseDir := t.TempDir()
	recorder, err := recording.NewEventRecorder(baseDir)
	if err != nil {
		t.Fatalf("NewEventRecorder: %v", err)
	}
	now := time.Now()
	recorder.Record(Event{
		ID:     "evt_1",
		RunID:  "run_41",
		Type:   EventInputAccepted,
		Time:   now,
		Source: SourceRef{Kind: "test"},
		Payload: MessagePayload{
			Content: "historical input",
			Source:  SourceRef{Kind: "test"},
		},
	})
	recorder.Record(Event{
		ID:    "evt_2",
		RunID: "run_41",
		Type:  EventRunDone,
		Time:  now.Add(time.Millisecond),
		Payload: RunResult{
			Output: "historical output",
		},
	})

	bot := &testBot{}
	bot.run = func(_ string, _ RunHost) (string, error) {
		return "fresh output", nil
	}
	rt := newTestRuntime(bot)
	if err := rt.EnableEventRecording(baseDir); err != nil {
		t.Fatalf("EnableEventRecording: %v", err)
	}

	runID, err := rt.StartRun(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "fresh input"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if runID != "run_42" {
		t.Fatalf("run id = %q, want run_42", runID)
	}
	waitForRun(t, rt, runID)

	historical, ok := rt.LookupRun("run_41")
	if !ok {
		t.Fatal("historical run missing")
	}
	if historical.Input != "historical input" || historical.Output != "historical output" {
		t.Fatalf("historical run was changed: %#v", historical)
	}
}

func TestManagerEventRecordingRejectsEmptyDirectory(t *testing.T) {
	rt := newTestRuntime(&testBot{})
	if err := rt.EnableEventRecording(""); err == nil {
		t.Fatal("EnableEventRecording accepted an empty directory")
	}
}

func TestManagerEventRecordingIsConfiguredOnceConcurrently(t *testing.T) {
	rt := newTestRuntime(&testBot{})
	baseDir := t.TempDir()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- rt.EnableEventRecording(baseDir)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("EnableEventRecording: %v", err)
		}
	}
	if rt.recorder == nil {
		t.Fatal("event recorder was not configured")
	}
}

func TestManagerEventRecordingRejectsClosedRuntime(t *testing.T) {
	rt := newTestRuntime(&testBot{})
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	err := rt.EnableEventRecording(t.TempDir())
	var protocolErr *ProtocolError
	if !errors.As(err, &protocolErr) || protocolErr.Code != ErrorClosed {
		t.Fatalf("EnableEventRecording error = %v, want closed protocol error", err)
	}
}

func TestManagerSteerRedactsSensitiveInputEvents(t *testing.T) {
	bot := &testBot{}
	rt := newTestRuntime(bot)
	started := make(chan struct{})
	release := make(chan struct{})
	bot.run = func(string, RunHost) (string, error) {
		close(started)
		<-release
		return "ok", nil
	}

	runID, err := rt.StartRun(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "start"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for bot run to start")
	}
	err = rt.SteerRun(context.Background(), runID, Input{
		Source: SourceRef{Kind: "test"},
		Text:   "/connect telegram token 123456:super-secret",
	})
	if err != nil {
		t.Fatalf("Steer: %v", err)
	}

	var found bool
	for _, ev := range rt.events.History(EventFilter{RunID: runID, Types: []EventType{EventInputAccepted}}) {
		p, ok := ev.Payload.(MessagePayload)
		if !ok || !strings.Contains(p.Content, "/connect telegram token") {
			continue
		}
		found = true
		if strings.Contains(p.Content, "super-secret") || !strings.Contains(p.Content, "[redacted]") {
			t.Fatalf("steer input was not redacted: %q", p.Content)
		}
	}
	if !found {
		t.Fatal("redacted steer input event not found")
	}
	close(release)
	waitForRun(t, rt, runID)
}
