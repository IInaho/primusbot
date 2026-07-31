package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestManagerPublishesSessionChangeFromServiceState(t *testing.T) {
	runner := &sessionCommandRunner{current: "session_1"}
	runner.command = func(string, RunHost) CommandResult {
		runner.current = "session_2"
		return CommandResult{Action: CommandHandled}
	}
	rt := New(runner)

	runID, err := rt.StartRun(context.Background(), Input{
		Source: SourceRef{Kind: "test"},
		Text:   "/sessions session_2",
	})
	if err != nil {
		t.Fatal(err)
	}
	waitForRun(t, rt, runID)
	events := rt.events.History(EventFilter{
		RunID: runID,
		Types: []EventType{EventSessionChanged},
	})
	if len(events) != 1 {
		t.Fatalf("session_changed events = %d, want 1", len(events))
	}
	payload, ok := events[0].Payload.(SessionPayload)
	if !ok || payload.ID != "session_2" {
		t.Fatalf("session_changed payload = %#v, want session_2", events[0].Payload)
	}
}

// TestManagerSessionClearanceDoesNotPublishSessionChange guards against a
// regression where a plain command (e.g. /model) cleared the empty session ID
// via saveSession's no-message cleanup, which used to be announced as a
// session_changed event and wiped the command's own system output from the UI.
func TestManagerSessionClearanceDoesNotPublishSessionChange(t *testing.T) {
	runner := &sessionCommandRunner{current: "session_1"}
	runner.command = func(string, RunHost) CommandResult {
		runner.current = "" // simulate saveSession clearing an empty session
		return CommandResult{Action: CommandHandled, Output: "handled"}
	}
	rt := New(runner)

	runID, err := rt.StartRun(context.Background(), Input{
		Source: SourceRef{Kind: "test"},
		Text:   "/model deepseek",
	})
	if err != nil {
		t.Fatal(err)
	}
	waitForRun(t, rt, runID)
	events := rt.events.History(EventFilter{
		RunID: runID,
		Types: []EventType{EventSessionChanged},
	})
	if len(events) != 0 {
		t.Fatalf("session_changed events = %d, want 0 (empty session clearance must not be announced)", len(events))
	}
	// The command's system output must still be published.
	var sawOutput bool
	for _, ev := range rt.events.History(EventFilter{RunID: runID, Types: []EventType{EventSystemMessage}}) {
		p, ok := ev.Payload.(MessagePayload)
		if ok && p.Content == "handled" {
			sawOutput = true
		}
	}
	if !sawOutput {
		t.Fatal("command system output missing after session clearance")
	}
}

func TestManagerSessionMutationsPublishCurrentSession(t *testing.T) {
	runner := &sessionCommandRunner{current: "session_1"}
	rt := New(runner)

	if err := rt.ResumeSession("session_2"); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.NewSession(); err != nil {
		t.Fatal(err)
	}
	if err := rt.DeleteSession("session_new"); err != nil {
		t.Fatal(err)
	}

	events := rt.events.History(EventFilter{Types: []EventType{EventSessionChanged}})
	want := []string{"session_2", "session_new", ""}
	if len(events) != len(want) {
		t.Fatalf("session_changed events = %d, want %d", len(events), len(want))
	}
	for i, event := range events {
		payload, ok := event.Payload.(SessionPayload)
		if !ok || payload.ID != want[i] {
			t.Fatalf("event %d payload = %#v, want ID %q", i, event.Payload, want[i])
		}
	}
}

func TestManagerRejectsMutationsWhileRunIsActive(t *testing.T) {
	release := make(chan struct{})
	runner := &modelMutationRunner{}
	runner.run = func(_ string, _ RunHost) (string, error) {
		<-release
		return "", nil
	}
	rt := New(runner)
	runID, err := rt.StartRun(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "run"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.SwitchModel("other"); err == nil {
		t.Fatal("SwitchModel succeeded while run was active")
	} else {
		var protocolErr *ProtocolError
		if !errors.As(err, &protocolErr) || protocolErr.Code != ErrorBusy {
			t.Fatalf("SwitchModel error = %v, want busy", err)
		}
	}
	if runner.switches != 0 {
		t.Fatal("model mutation reached runner while busy")
	}
	close(release)
	waitForRun(t, rt, runID)
	if selection, err := rt.SwitchModel("other"); err != nil || selection.Model != "other" {
		t.Fatalf("SwitchModel after run = %+v, %v", selection, err)
	}
}
