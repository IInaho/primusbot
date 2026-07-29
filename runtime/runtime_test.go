package runtime

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"nekocode/runtime/internal/connectors"
	"nekocode/runtime/internal/core"
	"nekocode/runtime/internal/recording"
)

type testBot struct {
	mu        sync.Mutex
	confirm   func(ConfirmRequest) ConfirmReply
	steers    int
	aborts    int
	run       func(string, RunCallbacks) (string, error)
	command   func(string) (string, CmdResult)
	skill     func() (string, bool)
	commands  []string
	callbacks ControlCallbacks
}

func (b *testBot) Run(input string, callbacks RunCallbacks) (string, error) {
	if b.run != nil {
		return b.run(input, callbacks)
	}
	return "", nil
}

func (b *testBot) ExecuteCommand(input string) (string, CmdResult) {
	if b.command != nil {
		return b.command(input)
	}
	return "", CmdNone
}
func (b *testBot) SkillHint() (string, bool) {
	if b.skill != nil {
		return b.skill()
	}
	return "", false
}
func (b *testBot) Stats() BotStats        { return BotStats{} }
func (b *testBot) CommandNames() []string { return append([]string(nil), b.commands...) }
func (b *testBot) ConfigureRuntime(callbacks ControlCallbacks) {
	b.confirm = callbacks.Confirm
	b.callbacks = callbacks
}
func (b *testBot) Steer(string) {
	b.mu.Lock()
	b.steers++
	b.mu.Unlock()
}
func (b *testBot) Abort() {
	b.mu.Lock()
	b.aborts++
	b.mu.Unlock()
}
func (b *testBot) Close()                                  {}
func (b *testBot) ProviderModel() (provider, model string) { return "", "" }
func (b *testBot) SessionMessages() []DisplayMessage       { return nil }
func (b *testBot) steerCount() int                         { b.mu.Lock(); defer b.mu.Unlock(); return b.steers }
func (b *testBot) abortCount() int                         { b.mu.Lock(); defer b.mu.Unlock(); return b.aborts }

func newTestRuntime(b *testBot) *Manager {
	return New(b)
}

type statusPublishingConnector struct {
	rt core.ConnectorRuntime
}

func (c statusPublishingConnector) Name() string { return "telegram" }
func (c statusPublishingConnector) Start(context.Context) error {
	return nil
}
func (c statusPublishingConnector) Stop() error {
	c.rt.ReportConnectorStatus(ConnectorStatusPayload{
		Name:    "telegram",
		Status:  "stopped",
		Message: "Telegram connector stopped.",
	})
	return nil
}
func (c statusPublishingConnector) HandleCommand(context.Context, []string) (string, error) {
	return "connected", nil
}

func TestManagerRedactsSensitiveInputEvents(t *testing.T) {
	rt := newTestRuntime(&testBot{})
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

func TestManagerDisconnectCommandDoesNotDuplicateConnectorStatus(t *testing.T) {
	rt := newTestRuntime(&testBot{})
	rt.RegisterConnector("telegram", func(runtime connectors.Host) connectors.Connector {
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
		Payload: DonePayload{
			Output: "historical output",
		},
	})

	bot := &testBot{}
	bot.run = func(string, RunCallbacks) (string, error) {
		return "fresh output", nil
	}
	rt := newTestRuntime(bot)
	if err := rt.EnableEventRecording(baseDir); err != nil {
		t.Fatalf("EnableEventRecording: %v", err)
	}

	runID, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "fresh input"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if runID != "run_42" {
		t.Fatalf("run id = %q, want run_42", runID)
	}
	waitForRun(t, rt, runID)

	historical, ok := rt.RunView("run_41")
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

func TestManagerCommandFinishesWithoutRunningAgent(t *testing.T) {
	bot := &testBot{}
	bot.command = func(string) (string, CmdResult) {
		return "handled", CmdHandled
	}
	bot.run = func(string, RunCallbacks) (string, error) {
		t.Fatal("agent should not run for handled command without skill continuation")
		return "", nil
	}
	rt := newTestRuntime(bot)

	runID, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "/help"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForRun(t, rt, runID)
	run, ok := rt.RunView(runID)
	if !ok {
		t.Fatal("RunView missing")
	}
	if run.Status != RunDone || run.Output != "" {
		t.Fatalf("run = %#v, want done with empty output", run)
	}
	var sawCommandResponse bool
	for _, ev := range rt.EventHistory(EventFilter{RunID: runID, Types: []EventType{EventSystemMessage}}) {
		p, ok := ev.Payload.(MessagePayload)
		if ok && p.Content == "handled" {
			sawCommandResponse = true
		}
	}
	if !sawCommandResponse {
		t.Fatal("command response event not found")
	}
}

func TestManagerCommandWithSkillHintContinuesToAgent(t *testing.T) {
	bot := &testBot{}
	bot.command = func(string) (string, CmdResult) {
		return "skill selected", CmdHandled
	}
	bot.skill = func() (string, bool) {
		return "review", true
	}
	bot.run = func(string, RunCallbacks) (string, error) {
		return "agent ran", nil
	}
	rt := newTestRuntime(bot)

	runID, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "/skill review"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForRun(t, rt, runID)
	run, ok := rt.RunView(runID)
	if !ok {
		t.Fatal("RunView missing")
	}
	if run.Output != "agent ran" {
		t.Fatalf("run output = %q, want agent ran", run.Output)
	}
	var sawSkillHint bool
	for _, ev := range rt.EventHistory(EventFilter{RunID: runID, Types: []EventType{EventSystemMessage}}) {
		p, ok := ev.Payload.(MessagePayload)
		if ok && p.Content == "Loaded skill: review" {
			sawSkillHint = true
		}
	}
	if !sawSkillHint {
		t.Fatal("skill hint event not found")
	}
}

func TestManagerAsyncCommandFinishesOnCallback(t *testing.T) {
	bot := &testBot{}
	executed := make(chan struct{})
	bot.command = func(string) (string, CmdResult) {
		close(executed)
		return "working", CmdConfirming
	}
	rt := newTestRuntime(bot)

	runID, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "/plugin install demo"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	select {
	case <-executed:
	case <-time.After(time.Second):
		t.Fatal("async command did not execute")
	}
	time.Sleep(20 * time.Millisecond)
	if run, ok := rt.RunView(runID); !ok || run.Status != RunRunning {
		t.Fatalf("async command run = %#v, want running", run)
	}

	bot.callbacks.CommandDone()
	waitForRun(t, rt, runID)
	if run, _ := rt.RunView(runID); run.Status != RunDone {
		t.Fatalf("async command run = %#v, want done", run)
	}
	doneEvents := rt.EventHistory(EventFilter{RunID: runID, Types: []EventType{EventRunDone}})
	if len(doneEvents) != 1 {
		t.Fatalf("run_done events = %d, want 1", len(doneEvents))
	}
}

func TestManagerDoesNotDuplicateStreamedFinalText(t *testing.T) {
	bot := &testBot{}
	bot.run = func(_ string, callbacks RunCallbacks) (string, error) {
		callbacks.Text("hello")
		callbacks.Text(" world")
		callbacks.Step(StepEvent{Action: StepActionChat, Output: "hello world"})
		return "hello world", nil
	}
	rt := newTestRuntime(bot)

	runID, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "feishu"}, Text: "hi"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForRun(t, rt, runID)

	var streamed strings.Builder
	for _, ev := range rt.EventHistory(EventFilter{RunID: runID, Types: []EventType{EventAssistantDelta}}) {
		p, ok := ev.Payload.(DeltaPayload)
		if ok {
			streamed.WriteString(p.Delta)
		}
	}
	if got := streamed.String(); got != "hello world" {
		t.Fatalf("streamed assistant text = %q, want one copy of final text", got)
	}
}

func TestManagerIgnoresUnknownStepAction(t *testing.T) {
	bot := &testBot{}
	bot.run = func(_ string, callbacks RunCallbacks) (string, error) {
		callbacks.Step(StepEvent{Action: StepAction("unknown_action"), ToolName: "mystery", Output: "ignored"})
		callbacks.Step(StepEvent{Action: StepActionExecuteTool, ToolName: "read", ToolArgs: "path=a.go", Output: "ok"})
		return "done", nil
	}
	rt := newTestRuntime(bot)

	runID, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "run"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForRun(t, rt, runID)
	run, ok := rt.RunView(runID)
	if !ok {
		t.Fatal("RunView missing")
	}
	if len(run.Tools) != 1 {
		t.Fatalf("tools = %+v, want only known step action recorded", run.Tools)
	}
	if run.Tools[0].Name != "read" || run.Tools[0].Status != core.ToolDone {
		t.Fatalf("tool = %+v, want completed read tool", run.Tools[0])
	}
}

func TestManagerRejectsMessageWhileWaitingApproval(t *testing.T) {
	bot := &testBot{}
	rt := newTestRuntime(bot)
	bot.run = func(string, RunCallbacks) (string, error) {
		req := ConfirmRequest{
			ToolName: "shell",
			Args:     map[string]any{"command": "go test"},
			Kind:     ConfirmKindPermission,
			Response: make(chan ConfirmReply, 1),
		}
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

func TestManagerAbortPreservesAbortedRunAfterApprovalReturns(t *testing.T) {
	bot := &testBot{}
	rt := newTestRuntime(bot)
	runReturned := make(chan struct{})
	bot.run = func(string, RunCallbacks) (string, error) {
		defer close(runReturned)
		req := ConfirmRequest{
			ToolName: "shell",
			Args:     map[string]any{"command": "go test"},
			Kind:     ConfirmKindPermission,
			Response: make(chan ConfirmReply, 1),
		}
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
	select {
	case <-runReturned:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for bot run to return")
	}
	run, ok := rt.RunView(runID)
	if !ok {
		t.Fatal("RunView missing")
	}
	if run.Status != RunAborted {
		t.Fatalf("run status = %s, want %s; output=%q error=%q", run.Status, RunAborted, run.Output, run.Error)
	}
	if run.Output != "" {
		t.Fatalf("aborted run output = %q, want empty", run.Output)
	}
}

func TestManagerAbortRejectsPendingApproval(t *testing.T) {
	bot := &testBot{}
	rt := newTestRuntime(bot)
	bot.run = func(string, RunCallbacks) (string, error) {
		req := ConfirmRequest{
			ToolName: "shell",
			Args:     map[string]any{"command": "go test"},
			Kind:     ConfirmKindPermission,
			Response: make(chan ConfirmReply, 1),
		}
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

func TestManagerSteerRejectsInactiveRun(t *testing.T) {
	bot := &testBot{}
	rt := newTestRuntime(bot)
	started := make(chan struct{})
	release := make(chan struct{})
	bot.run = func(string, RunCallbacks) (string, error) {
		close(started)
		<-release
		return "ok", nil
	}

	runID, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "start"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for bot run to start")
	}
	if err := rt.Steer(context.Background(), RunID("run_missing"), Input{Source: SourceRef{Kind: "test"}, Text: "stale"}); err == nil {
		t.Fatal("Steer with inactive run succeeded")
	}
	if bot.steerCount() != 0 {
		t.Fatalf("steer count = %d, want 0", bot.steerCount())
	}
	close(release)
	waitForRun(t, rt, runID)
}

func TestManagerAbortRejectsInactiveRun(t *testing.T) {
	bot := &testBot{}
	rt := newTestRuntime(bot)
	started := make(chan struct{})
	release := make(chan struct{})
	bot.run = func(string, RunCallbacks) (string, error) {
		close(started)
		<-release
		return "ok", nil
	}

	runID, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "start"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for bot run to start")
	}
	if err := rt.Abort(context.Background(), RunID("run_missing")); err == nil {
		t.Fatal("Abort with inactive run succeeded")
	}
	if bot.abortCount() != 0 {
		t.Fatalf("abort count = %d, want 0", bot.abortCount())
	}
	close(release)
	waitForRun(t, rt, runID)
}

func TestManagerSteerRedactsSensitiveInputEvents(t *testing.T) {
	bot := &testBot{}
	rt := newTestRuntime(bot)
	started := make(chan struct{})
	release := make(chan struct{})
	bot.run = func(string, RunCallbacks) (string, error) {
		close(started)
		<-release
		return "ok", nil
	}

	runID, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "start"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for bot run to start")
	}
	err = rt.Steer(context.Background(), runID, Input{
		Source: SourceRef{Kind: "test"},
		Text:   "/connect telegram token 123456:super-secret",
	})
	if err != nil {
		t.Fatalf("Steer: %v", err)
	}

	var found bool
	for _, ev := range rt.EventHistory(EventFilter{RunID: runID, Types: []EventType{EventInputAccepted}}) {
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

func waitForStatus(t *testing.T, rt *Manager, status RunStatus) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if managerStatus(rt) == status {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for status %s, got %s", status, managerStatus(rt))
}

func managerStatus(rt *Manager) RunStatus {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.status
}

func TestManagerCustomRuntimeCommand(t *testing.T) {
	rt := newTestRuntime(&testBot{})
	rt.RegisterCommand("hello", func(_ context.Context, args []string) (string, error) {
		return "hello " + strings.Join(args, " "), nil
	})

	_, err := rt.Submit(context.Background(), Input{
		Source: SourceRef{Kind: "test"},
		Text:   "/hello world",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	var found bool
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if run, ok := rt.RunView(rt.currentRunID()); ok && run.Status == RunDone {
			found = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !found {
		t.Fatal("custom runtime command did not finish")
	}

	events := rt.EventHistory(EventFilter{})
	var sawMessage bool
	for _, ev := range events {
		if ev.Type == EventSystemMessage {
			if payload, ok := ev.Payload.(MessagePayload); ok && payload.Content == "hello world" {
				sawMessage = true
				break
			}
		}
	}
	if !sawMessage {
		t.Fatalf("did not see expected system message in events: %#v", events)
	}
}

func TestManagerCommandNamesIncludePrefixes(t *testing.T) {
	rt := newTestRuntime(&testBot{
		commands: []string{"/help", "$review", "model"},
	})

	got := rt.CommandNames()
	for _, want := range []string{"/connect", "/devices", "/disconnect", "/help", "/model", "$review"} {
		if !hasString(got, want) {
			t.Fatalf("CommandNames() missing %q in %v", want, got)
		}
	}
}

func TestManagerRuntimeCommandsRequireSlash(t *testing.T) {
	agentInput := make(chan string, 1)
	bot := &testBot{
		run: func(input string, _ RunCallbacks) (string, error) {
			agentInput <- input
			return "agent ran", nil
		},
	}
	rt := newTestRuntime(bot)
	rt.RegisterCommand("hello", func(_ context.Context, _ []string) (string, error) {
		return "runtime command ran", nil
	})

	runID, err := rt.Submit(context.Background(), Input{
		Source: SourceRef{Kind: "test"},
		Text:   "hello world",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForRun(t, rt, runID)

	select {
	case got := <-agentInput:
		if got != "hello world" {
			t.Fatalf("agent input = %q, want hello world", got)
		}
	default:
		t.Fatal("bare runtime command text should run the agent")
	}
}

func TestManagerCloseIsIdempotent(t *testing.T) {
	rt := newTestRuntime(&testBot{})
	rt.Close()
	rt.Close() // should not panic
}

func TestManagerCloseRejectsPendingApprovalAndBecomesTerminal(t *testing.T) {
	bot := &testBot{}
	replies := make(chan ConfirmReply, 1)
	bot.run = func(string, RunCallbacks) (string, error) {
		reply := bot.confirm(ConfirmRequest{
			ToolName: "shell",
			Args:     map[string]any{"command": "go test"},
			Response: make(chan ConfirmReply, 1),
		})
		replies <- reply
		return "", nil
	}
	rt := newTestRuntime(bot)

	if _, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "run"}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForStatus(t, rt, RunWaitingApproval)
	rt.Close()

	select {
	case reply := <-replies:
		if reply.Allowed {
			t.Fatalf("close approval reply = %+v, want rejection", reply)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not release pending approval")
	}
	if _, err := rt.Submit(context.Background(), Input{Source: SourceRef{Kind: "test"}, Text: "again"}); err == nil {
		t.Fatal("Submit after Close succeeded")
	}
	if _, err := rt.Subscribe(context.Background(), EventFilter{}); err == nil {
		t.Fatal("Subscribe after Close succeeded")
	}
}

func hasString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestManagerRecoversFromRunPanic(t *testing.T) {
	bot := &testBot{
		run: func(string, RunCallbacks) (string, error) {
			panic("intentional runner panic")
		},
	}
	rt := newTestRuntime(bot)

	_, err := rt.Submit(context.Background(), Input{
		Source: SourceRef{Kind: "test"},
		Text:   "trigger panic",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	var failedRun RunView
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if run, ok := rt.RunView(rt.currentRunID()); ok && run.Status == RunFailed {
			failedRun = run
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if failedRun.Status != RunFailed {
		t.Fatalf("expected run to fail after panic, got status %q", failedRun.Status)
	}
	if !strings.Contains(failedRun.Error, "panicked") {
		t.Fatalf("expected error to mention panic, got %q", failedRun.Error)
	}
}

func waitForRun(t *testing.T, rt *Manager, runID RunID) {
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
