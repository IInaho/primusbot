package runtime

import (
	"context"
	"fmt"
	"strings"
)

func (r *Manager) Submit(ctx context.Context, input Input) (RunID, error) {
	if strings.TrimSpace(input.Text) == "" {
		return "", fmt.Errorf("runtime: empty input")
	}
	if input.Kind == "" {
		input.Kind = InputMessage
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return "", fmt.Errorf("runtime: closed")
	}
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

func (r *Manager) Steer(_ context.Context, runID RunID, input Input) error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return fmt.Errorf("runtime: closed")
	}
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

	r.backend.Steer(input.Text)
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

func (r *Manager) Abort(_ context.Context, runID RunID) error {
	if err := r.ensureOpen(); err != nil {
		return err
	}
	runID, active, err := r.beginAbort(runID)
	if err != nil {
		return err
	}
	if !active {
		return nil
	}
	r.backend.Abort()
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

func (r *Manager) Approve(_ context.Context, approvalID string, decision ApprovalDecision) error {
	if err := r.ensureOpen(); err != nil {
		return err
	}
	return r.approvals.Decide(approvalID, decision)
}

func (r *Manager) Answer(_ context.Context, questionID string, reply QuestionReply) error {
	if err := r.ensureOpen(); err != nil {
		return err
	}
	return r.questions.Answer(questionID, reply)
}

func (r *Manager) Close() {
	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.closed = true
		runID := r.currentRun
		active := r.status == RunRunning || r.status == RunWaitingApproval || r.status == RunWaitingQuestion
		if active {
			r.aborted[runID] = struct{}{}
			r.status = RunAborted
		}
		r.mu.Unlock()

		if r.connectors != nil {
			_ = r.connectors.Close()
		}
		if active {
			r.backend.Abort()
		}
		r.approvals.Close()
		r.questions.Close()
		if active {
			r.events.Publish(Event{
				RunID:   runID,
				Type:    EventRunAborted,
				Source:  SourceRef{Kind: "runtime"},
				Payload: DonePayload{Error: "runtime closed"},
			})
		}
		r.backend.Close()
		if r.recorder != nil {
			_ = r.recorder.Close()
		}
		if r.events != nil {
			r.events.Close()
		}
	})
}
