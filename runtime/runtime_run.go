package runtime

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type runLease struct {
	mu          sync.Mutex
	drained     chan struct{}
	drainOnce   sync.Once
	interaction sync.Mutex
	active      bool
	inFlight    int
}

func newRunLease() *runLease {
	return &runLease{active: true, drained: make(chan struct{})}
}

func (l *runLease) emit(fn func()) {
	l.guard(fn)
}

func (l *runLease) guard(fn func()) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	if !l.active {
		l.mu.Unlock()
		return false
	}
	l.inFlight++
	l.mu.Unlock()

	defer func() {
		l.mu.Lock()
		l.inFlight--
		l.signalDrainedLocked()
		l.mu.Unlock()
	}()
	fn()
	return true
}

func (l *runLease) deactivate() {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.active = false
	l.signalDrainedLocked()
	l.mu.Unlock()
}

func (l *runLease) wait(ctx context.Context) error {
	if l == nil {
		return nil
	}
	select {
	case <-l.drained:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *runLease) signalDrainedLocked() {
	if !l.active && l.inFlight == 0 {
		l.drainOnce.Do(func() { close(l.drained) })
	}
}

type runHost struct {
	runtime *Manager
	runID   RunID
	lease   *runLease
}

func (h runHost) Text(delta string) {
	h.lease.emit(func() {
		h.runtime.events.Publish(Event{
			RunID: h.runID, Type: EventAssistantDelta, Source: SourceRef{Kind: "bot"},
			Payload: DeltaPayload{Delta: delta},
		})
	})
}

func (h runHost) Reason(delta string) {
	h.lease.emit(func() {
		h.runtime.events.Publish(Event{
			RunID: h.runID, Type: EventReasoningDelta, Source: SourceRef{Kind: "bot"},
			Payload: DeltaPayload{Delta: delta},
		})
	})
}

func (h runHost) Tool(event ToolEvent) {
	h.lease.emit(func() { h.runtime.publishTool(h.runID, event) })
}

func (h runHost) SubAgent(event SubAgentEvent) {
	h.lease.emit(func() { h.runtime.publishSubAgent(h.runID, event) })
}

func (h runHost) Phase(phase string) {
	h.lease.emit(func() {
		h.runtime.events.Publish(Event{
			RunID: h.runID, Type: EventPhaseChanged, Source: SourceRef{Kind: "bot"},
			Payload: PhasePayload{Phase: phase},
		})
	})
}

func (h runHost) Todos(items []TodoItem) {
	h.lease.emit(func() {
		h.runtime.events.Publish(Event{
			RunID: h.runID, Type: EventTodosUpdated, Source: SourceRef{Kind: "bot"},
			Payload: items,
		})
	})
}

func (h runHost) system(message string, source SourceRef) {
	if strings.TrimSpace(message) == "" {
		return
	}
	h.lease.emit(func() {
		h.runtime.events.Publish(Event{
			RunID: h.runID, Type: EventSystemMessage, Source: source,
			Payload: MessagePayload{Role: "system", Content: message, Source: source},
		})
	})
}

func (h runHost) Confirm(request ConfirmRequest) ConfirmReply {
	h.lease.interaction.Lock()
	defer h.lease.interaction.Unlock()

	var wait func() ConfirmReply
	var waiting bool
	ok := h.lease.guard(func() {
		wait = h.runtime.approvals.Register(request)
		if wait != nil {
			waiting = h.runtime.beginWaiting(h.runID, RunWaitingApproval)
		}
	})
	if !ok || wait == nil {
		return ConfirmReply{Allowed: false}
	}
	reply := wait()
	if waiting {
		h.runtime.resumeRun(h.runID, RunWaitingApproval)
	}
	return reply
}

func (h runHost) Ask(request QuestionRequest) QuestionReply {
	h.lease.interaction.Lock()
	defer h.lease.interaction.Unlock()

	var wait func() QuestionReply
	var waiting bool
	ok := h.lease.guard(func() {
		wait = h.runtime.questions.Register(request)
		if wait != nil {
			waiting = h.runtime.beginWaiting(h.runID, RunWaitingQuestion)
		}
	})
	if !ok || wait == nil {
		return QuestionReply{Rejected: true}
	}
	reply := wait()
	if waiting {
		h.runtime.resumeRun(h.runID, RunWaitingQuestion)
	}
	return reply
}

func (r *Manager) run(ctx context.Context, runID RunID, input Input, lease *runLease) {
	stopMetricsUpdates := func() {}
	defer func() {
		stopMetricsUpdates()
		if recovered := recover(); recovered != nil {
			r.finishRun(runID, "", fmt.Errorf("runtime: run %s panicked: %v", runID, recovered))
		}
		r.endRun(runID)
	}()

	host := runHost{runtime: r, runID: runID, lease: lease}
	if !lease.guard(func() {
		r.events.Publish(Event{RunID: runID, Type: EventRunStarted, Source: SourceRef{Kind: "runtime"}})
	}) {
		return
	}
	stopMetricsUpdates = r.startMetricsUpdates(runID, lease)

	if r.handleRuntimeCommand(ctx, runID, input, host) {
		return
	}

	agentInput, proceed := r.handleBotCommand(ctx, runID, input.Text, host)
	if !proceed {
		return
	}

	result, err := r.runner.Run(ctx, agentInput, host)
	r.finishRun(runID, result, err)
}

func (r *Manager) handleBotCommand(ctx context.Context, runID RunID, input string, host runHost) (string, bool) {
	if r.commander == nil {
		return input, true
	}
	sessionID := ""
	if r.sessions != nil {
		sessionID = r.sessions.CurrentSessionID()
	}
	result, err := r.commander.ExecuteCommand(ctx, input, host)
	if err != nil {
		r.finishRun(runID, "", err)
		return "", false
	}
	if result.Output != "" {
		host.system(result.Output, SourceRef{Kind: "bot"})
	}
	if r.sessions != nil {
		nextSessionID := r.sessions.CurrentSessionID()
		if sessionID != nextSessionID {
			host.lease.emit(func() {
				r.events.Publish(Event{
					RunID: runID, Type: EventSessionChanged, Source: SourceRef{Kind: "bot"},
					Payload: SessionPayload{ID: nextSessionID},
				})
			})
		}
	}
	switch result.Action {
	case CommandIgnored:
		return input, true
	case CommandContinue:
		if strings.TrimSpace(result.AgentInput) != "" {
			return result.AgentInput, true
		}
		return input, true
	default:
		r.finishRun(runID, "", nil)
		return "", false
	}
}

func (r *Manager) handleRuntimeCommand(ctx context.Context, runID RunID, input Input, host runHost) bool {
	fields := strings.Fields(strings.TrimSpace(input.Text))
	if len(fields) == 0 {
		return false
	}
	head := strings.ToLower(fields[0])
	if !strings.HasPrefix(head, "/") {
		return false
	}
	r.mu.Lock()
	handler, ok := r.runtimeCommands[strings.TrimPrefix(head, "/")]
	r.mu.Unlock()
	if !ok {
		return false
	}
	response, err := handler(ctx, fields[1:])
	if err != nil {
		response = err.Error()
	}
	host.system(response, SourceRef{Kind: "runtime"})
	r.finishRun(runID, "", nil)
	return true
}

func (r *Manager) finishRun(runID RunID, output string, err error) {
	payload := RunResult{Output: output}
	eventType := EventRunDone
	status := RunDone
	if err != nil {
		eventType = EventRunFailed
		status = RunFailed
		payload.Error = err.Error()
	}

	r.mu.Lock()
	if r.closed || r.currentRun != runID {
		r.mu.Unlock()
		return
	}
	if _, cancelled := r.cancelled[runID]; cancelled {
		r.mu.Unlock()
		return
	}
	switch r.status {
	case RunRunning, RunWaitingApproval, RunWaitingQuestion:
	default:
		r.mu.Unlock()
		return
	}
	r.status = status
	cancel := r.cancelRun
	r.cancelRun = nil
	r.runContext = nil
	lease := r.runLease
	r.runLease = nil
	r.mu.Unlock()

	lease.deactivate()
	if cancel != nil {
		cancel()
	}
	_ = lease.wait(context.Background())
	r.approvals.RejectAll()
	r.questions.RejectAll()
	r.publishMetrics(runID)
	r.events.Publish(Event{RunID: runID, Type: eventType, Source: SourceRef{Kind: "bot"}, Payload: payload})

}

func (r *Manager) endRun(runID RunID) {
	r.mu.Lock()
	if r.currentRun != runID {
		r.mu.Unlock()
		return
	}
	status := r.status
	cancelDone := r.cancelDone
	r.mu.Unlock()

	if status == RunCancelled && cancelDone != nil {
		<-cancelDone
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.currentRun != runID {
		return
	}
	if r.status == RunDone || r.status == RunFailed || r.status == RunCancelled {
		r.status = RunIdle
		r.runContext = nil
	}
	delete(r.cancelled, runID)
	if r.runDone != nil {
		close(r.runDone)
		r.runDone = nil
	}
}

func (r *Manager) publishTool(runID RunID, event ToolEvent) {
	var eventType EventType
	payload := ToolPayload{
		ToolName: event.Name, CallID: event.CallID, Args: event.Args,
		Output: event.Output, Preview: event.Preview, IsError: event.IsError,
		SubAgentID: event.SubAgentID, SubAgentColor: event.SubAgentColor,
	}
	switch event.Kind {
	case ToolEventStarted:
		eventType = EventToolStarted
	case ToolEventBlocked:
		eventType = EventToolBlocked
	case ToolEventPreview:
		eventType = EventToolPreview
	case ToolEventCompleted:
		eventType = EventToolCompleted
	default:
		return
	}
	r.events.Publish(Event{RunID: runID, Type: eventType, Source: SourceRef{Kind: "bot"}, Payload: payload})
	r.publishMetrics(runID)
}

func (r *Manager) publishSubAgent(runID RunID, event SubAgentEvent) {
	var eventType EventType
	switch event.Kind {
	case SubAgentEventStarted:
		eventType = EventSubAgentStarted
	case SubAgentEventEnded:
		eventType = EventSubAgentEnded
	default:
		return
	}
	r.events.Publish(Event{
		RunID: runID, Type: eventType, Source: SourceRef{Kind: "bot"},
		Payload: SubAgentPayload{ID: event.ID, Type: event.Type, Color: event.Color},
	})
	r.publishMetrics(runID)
}
