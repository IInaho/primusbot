package session

import (
	"context"
	"strings"
	"sync"

	"nekocode/runtime/internal/connectors"
)

type SessionRuntime struct {
	bot        RuntimeBot
	events     *EventBus
	approvals  *ApprovalBroker
	questions  *QuestionBroker
	connectors *connectors.Manager
	runs       *RunStore
	recorder   *EventRecorder

	mu         sync.Mutex
	currentRun RunID
	status     RunStatus
	nextRun    uint64
}

func NewSessionRuntime(b RuntimeBot) *SessionRuntime {
	events := NewEventBus()
	rt := &SessionRuntime{
		bot:    b,
		events: events,
		runs:   NewRunStore(0),
		status: RunIdle,
	}
	events.AddObserver(rt.runs.Record)
	rt.approvals = NewApprovalBroker(events, SourceRef{Kind: "runtime"}, rt.CurrentRunID)
	rt.questions = NewQuestionBroker(events, SourceRef{Kind: "runtime"}, rt.CurrentRunID)
	rt.connectors = connectors.NewManager(rt)
	rt.configureBot()
	return rt
}

func (r *SessionRuntime) Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, error) {
	return r.events.Subscribe(ctx, filter)
}

func (r *SessionRuntime) SubscribeReplay(ctx context.Context, filter EventFilter) (<-chan Event, error) {
	return r.events.SubscribeReplay(ctx, filter)
}

func (r *SessionRuntime) Publish(ev Event) {
	r.events.Publish(ev)
}

func (r *SessionRuntime) EnableEventRecording(baseDir string) error {
	if r.recorder != nil {
		return nil
	}
	if strings.TrimSpace(baseDir) == "" {
		baseDir = defaultEventRecorderBaseDir()
	}
	if events, err := LoadRecordedEvents(baseDir); err == nil && len(events) > 0 {
		for _, ev := range events {
			r.runs.Record(ev)
		}
		r.events.ImportHistory(events)
	}
	recorder, err := NewEventRecorder(baseDir)
	if err != nil {
		return err
	}
	r.recorder = recorder
	r.events.AddObserver(recorder.Record)
	return nil
}

func (r *SessionRuntime) EventRecordingSessionID() string {
	if r.recorder == nil {
		return ""
	}
	return r.recorder.SessionID()
}

func (r *SessionRuntime) CurrentRunID() RunID {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentRun
}

func (r *SessionRuntime) Status() RunStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *SessionRuntime) PendingApprovals() []ApprovalView {
	return r.approvals.Pending()
}

func (r *SessionRuntime) PendingQuestions() []QuestionView {
	return r.questions.Pending()
}

func (r *SessionRuntime) CurrentRunView() (RunView, bool) {
	return r.runs.CurrentRunView()
}

func (r *SessionRuntime) RunView(runID RunID) (RunView, bool) {
	return r.runs.RunView(runID)
}

func (r *SessionRuntime) ListRunViews(limit int) []RunView {
	return r.runs.ListRunViews(limit)
}

func (r *SessionRuntime) ArtifactView(runID RunID) (ArtifactView, bool) {
	return r.runs.ArtifactView(runID)
}

func (r *SessionRuntime) ConnectView() ConnectView {
	return r.connectors.View()
}

func (r *SessionRuntime) EventHistory(filter EventFilter) []Event {
	return r.events.History(filter)
}

func (r *SessionRuntime) setStatus(status RunStatus) {
	r.mu.Lock()
	r.status = status
	r.mu.Unlock()
}
