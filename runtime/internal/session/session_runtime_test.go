package session

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"nekocode/runtime/internal/connectors"
	"nekocode/runtime/internal/core"
	"nekocode/runtime/view"
)

type testBot struct {
	mu       sync.Mutex
	confirm  view.ConfirmFunc
	steers   int
	run      func(string, view.RunCallbacks) (string, error)
	commands []string
}

func (b *testBot) Run(input string, callbacks view.RunCallbacks) (string, error) {
	if b.run != nil {
		return b.run(input, callbacks)
	}
	return "", nil
}

func (b *testBot) ExecuteCommand(string) (string, view.CmdResult) { return "", view.CmdNone }
func (b *testBot) SkillHint() (string, bool)                      { return "", false }
func (b *testBot) Stats() view.BotStats                           { return view.BotStats{} }
func (b *testBot) CommandNames() []string                         { return append([]string(nil), b.commands...) }
func (b *testBot) Configure(confirmFn view.ConfirmFunc, _ view.PhaseFunc, _ view.TodoFunc, _ func(string), _ chan view.ConfirmRequest, _ view.QuestionFunc) {
	b.confirm = confirmFn
}
func (b *testBot) Steer(string) {
	b.mu.Lock()
	b.steers++
	b.mu.Unlock()
}
func (b *testBot) Abort()                                  {}
func (b *testBot) Close()                                  {}
func (b *testBot) ProviderModel() (provider, model string) { return "", "" }
func (b *testBot) SessionMessages() []view.DisplayMessage  { return nil }
func (b *testBot) steerCount() int                         { b.mu.Lock(); defer b.mu.Unlock(); return b.steers }

type statusPublishingConnector struct {
	rt core.Runtime
}

func (c statusPublishingConnector) Name() string { return "telegram" }
func (c statusPublishingConnector) Start(context.Context) error {
	return nil
}
func (c statusPublishingConnector) Stop() error {
	if publisher, ok := c.rt.(interface{ Publish(Event) }); ok {
		publisher.Publish(Event{
			Type:   EventConnectorStatus,
			Source: SourceRef{Kind: "telegram"},
			Payload: ConnectorStatusPayload{
				Name:    "telegram",
				Status:  "stopped",
				Message: "Telegram connector stopped.",
			},
		})
	}
	return nil
}
func (c statusPublishingConnector) HandleCommand(context.Context, []string) (string, error) {
	return "connected", nil
}

func TestSessionRuntimeRedactsSensitiveInputEvents(t *testing.T) {
	rt := NewSessionRuntime(&testBot{})
	runID, err := rt.Submit(context.Background(), Input{
		Source: SourceRef{Kind: "test"},
		Text:   "/connect telegram token 123456:super-secret",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForRun(t, rt, runID)
	run, ok := rt.RunView(runID)
	if !ok {
		t.Fatal("RunView missing")
	}
	if strings.Contains(run.Input, "super-secret") || !strings.Contains(run.Input, "[redacted]") {
		t.Fatalf("run input was not redacted: %q", run.Input)
	}
}

func TestSessionRuntimeDisconnectCommandDoesNotDuplicateConnectorStatus(t *testing.T) {
	rt := NewSessionRuntime(&testBot{})
	rt.RegisterConnector("telegram", func(runtime connectors.Runtime) connectors.Connector {
		return statusPublishingConnector{rt: runtime}
	})
	if _, err := rt.Connect(context.Background(), "telegram", nil); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	runID, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "/disconnect telegram"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForRun(t, rt, runID)

	for _, ev := range rt.EventHistory(EventFilter{RunID: runID}) {
		if ev.Type == EventSystemMessage {
			t.Fatalf("disconnect command should rely on connector status event, got system message: %#v", ev.Payload)
		}
	}

	var statusMessages int
	for _, ev := range rt.EventHistory(EventFilter{}) {
		if ev.Type == EventConnectorStatus {
			statusMessages++
		}
	}
	if statusMessages != 1 {
		t.Fatalf("connector status events = %d, want 1", statusMessages)
	}
}

func TestSessionRuntimeRejectsMessageWhileWaitingApproval(t *testing.T) {
	bot := &testBot{}
	rt := NewSessionRuntime(bot)
	bot.run = func(string, view.RunCallbacks) (string, error) {
		req := view.NewConfirmRequest("shell", map[string]any{"command": "go test"}, view.ConfirmKindPermission)
		reply := bot.confirm(req)
		if !reply.Allowed {
			return "", nil
		}
		return "ok", nil
	}

	runID, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "run command"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForStatus(t, rt, RunWaitingApproval)

	gotRunID, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "new text"})
	if err == nil {
		t.Fatal("Submit while waiting approval succeeded")
	}
	if gotRunID != runID {
		t.Fatalf("run id = %q, want %q", gotRunID, runID)
	}
	if bot.steerCount() != 0 {
		t.Fatalf("steer count = %d, want 0", bot.steerCount())
	}

	pending := rt.PendingApprovals()
	if len(pending) != 1 {
		t.Fatalf("pending approvals = %d, want 1", len(pending))
	}
	if err := rt.Approve(context.Background(), pending[0].ID, ApprovalDecision{Allowed: true}); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	waitForRun(t, rt, runID)
}

func TestSessionRuntimeAbortRejectsPendingApproval(t *testing.T) {
	bot := &testBot{}
	rt := NewSessionRuntime(bot)
	bot.run = func(string, view.RunCallbacks) (string, error) {
		req := view.NewConfirmRequest("shell", map[string]any{"command": "go test"}, view.ConfirmKindPermission)
		reply := bot.confirm(req)
		if reply.Allowed {
			return "approved", nil
		}
		return "rejected", nil
	}

	runID, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "run command"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForStatus(t, rt, RunWaitingApproval)
	if err := rt.Abort(context.Background(), runID); err != nil {
		t.Fatalf("Abort: %v", err)
	}
	waitForRun(t, rt, runID)
	if pending := rt.PendingApprovals(); len(pending) != 0 {
		t.Fatalf("pending approvals after abort = %d", len(pending))
	}
}

func waitForStatus(t *testing.T, rt *SessionRuntime, status RunStatus) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if rt.Status() == status {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for status %s, got %s", status, rt.Status())
}

func waitForRun(t *testing.T, rt *SessionRuntime, runID RunID) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if run, ok := rt.RunView(runID); ok && (run.Status == RunDone || run.Status == RunFailed || run.Status == RunAborted) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	run, _ := rt.RunView(runID)
	t.Fatalf("timed out waiting for run %s to finish: %#v", runID, run)
}
