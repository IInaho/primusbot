package session

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

func (r *SessionRuntime) Submit(ctx context.Context, input Input) (RunID, error) {
	if strings.TrimSpace(input.Text) == "" {
		return "", fmt.Errorf("runtime: empty input")
	}
	if input.Kind == "" {
		input.Kind = InputMessage
	}

	r.mu.Lock()
	if r.status == RunWaitingApproval || r.status == RunWaitingQuestion {
		runID := r.currentRun
		status := r.status
		r.mu.Unlock()
		return runID, fmt.Errorf("runtime: current run is %s; answer the pending approval or question first", status)
	}
	if r.status == RunRunning {
		runID := r.currentRun
		r.mu.Unlock()
		return runID, r.Steer(ctx, runID, input)
	}
	r.nextRun++
	runID := RunID(fmt.Sprintf("run_%d", r.nextRun))
	r.currentRun = runID
	r.status = RunRunning
	delete(r.aborted, runID)
	r.mu.Unlock()

	r.events.Publish(Event{
		RunID:  runID,
		Type:   EventInputAccepted,
		Source: input.Source,
		Payload: MessagePayload{
			Role:    "user",
			Content: RedactInputText(input.Text),
			Source:  input.Source,
			Sender:  input.Sender,
		},
	})

	go r.run(ctx, runID, input)
	return runID, nil
}

func (r *SessionRuntime) Steer(_ context.Context, runID RunID, input Input) error {
	r.mu.Lock()
	if runID == "" {
		runID = r.currentRun
	}
	if runID == "" || r.currentRun == "" {
		r.mu.Unlock()
		return fmt.Errorf("runtime: no active run")
	}
	if runID != r.currentRun {
		currentRun := r.currentRun
		r.mu.Unlock()
		return fmt.Errorf("runtime: run %s is not active; current run is %s", runID, currentRun)
	}
	if r.status != RunRunning {
		status := r.status
		r.mu.Unlock()
		return fmt.Errorf("runtime: current run is %s; cannot steer now", status)
	}
	if _, aborted := r.aborted[runID]; aborted {
		r.mu.Unlock()
		return fmt.Errorf("runtime: run %s is aborted", runID)
	}
	r.mu.Unlock()

	r.control.Steer(input.Text)
	r.events.Publish(Event{
		RunID:  runID,
		Type:   EventInputAccepted,
		Source: input.Source,
		Payload: MessagePayload{
			Role:    "user",
			Content: RedactInputText(input.Text),
			Source:  input.Source,
			Sender:  input.Sender,
		},
	})
	return nil
}

func (r *SessionRuntime) Abort(_ context.Context, runID RunID) error {
	runID, active, err := r.beginAbort(runID)
	if err != nil {
		return err
	}
	if !active {
		return nil
	}
	r.control.Abort()
	r.approvals.RejectAll()
	r.questions.RejectAll()
	r.events.Publish(Event{
		RunID:   runID,
		Type:    EventRunAborted,
		Source:  SourceRef{Kind: "runtime"},
		Payload: DonePayload{Error: "aborted"},
	})
	return nil
}

func (r *SessionRuntime) Approve(_ context.Context, approvalID string, decision ApprovalDecision) error {
	return r.approvals.Decide(approvalID, decision)
}

func (r *SessionRuntime) Answer(_ context.Context, questionID string, reply QuestionReply) error {
	return r.questions.Answer(questionID, reply)
}

func (r *SessionRuntime) Connect(ctx context.Context, name string, args []string) (string, error) {
	return r.connectors.Handle(ctx, append([]string{name}, args...))
}

func (r *SessionRuntime) Disconnect(name string) (string, error) {
	return r.connectors.Disconnect(name)
}

func (r *SessionRuntime) RegisterConnector(name string, factory ConnectorFactory) {
	r.connectors.Register(name, factory)
}

func (r *SessionRuntime) Close() {
	r.closeOnce.Do(func() {
		r.control.Close()
		if r.connectors != nil {
			_ = r.connectors.Close()
		}
		if r.recorder != nil {
			_ = r.recorder.Close()
		}
		if r.events != nil {
			r.events.Close()
		}
		if r.confirmDone != nil {
			close(r.confirmDone)
		}
	})
}

func (r *SessionRuntime) Stats() BotStats {
	return r.stats.Stats()
}

func (r *SessionRuntime) CommandNames() []string {
	seen := make(map[string]bool)
	names := make([]string, 0)
	for _, name := range r.catalog.CommandNames() {
		display := commandDisplayName(name)
		if display == "" || seen[display] {
			continue
		}
		seen[display] = true
		names = append(names, display)
	}
	for name := range r.runtimeCommands {
		display := commandDisplayName(name)
		if seen[display] {
			continue
		}
		seen[display] = true
		names = append(names, display)
	}
	sort.Strings(names)
	return names
}

func commandDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "$") {
		return name
	}
	return "/" + name
}

func (r *SessionRuntime) ProviderModel() (string, string) {
	return r.model.ProviderModel()
}

func (r *SessionRuntime) SessionMessages() []DisplayMessage {
	return r.messages.SessionMessages()
}
