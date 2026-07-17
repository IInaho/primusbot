package session

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"nekocode/runtime/view"
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
	runID := RunID(fmt.Sprintf("run_%d", atomic.AddUint64(&r.nextRun, 1)))
	r.currentRun = runID
	r.status = RunRunning
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
	if runID == "" {
		runID = r.CurrentRunID()
	}
	if runID == "" {
		return fmt.Errorf("runtime: no active run")
	}
	r.bot.Steer(input.Text)
	r.events.Publish(Event{
		RunID:  runID,
		Type:   EventInputAccepted,
		Source: input.Source,
		Payload: MessagePayload{
			Role:    "user",
			Content: input.Text,
			Source:  input.Source,
			Sender:  input.Sender,
		},
	})
	return nil
}

func (r *SessionRuntime) Abort(_ context.Context, runID RunID) error {
	if runID == "" {
		runID = r.CurrentRunID()
	}
	if runID == "" {
		return nil
	}
	r.bot.Abort()
	r.setStatus(RunAborted)
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

func (r *SessionRuntime) Answer(_ context.Context, questionID string, reply view.QuestionReply) error {
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
	r.bot.Close()
}

func (r *SessionRuntime) ExecuteCommand(input string) (string, view.CmdResult) {
	return r.bot.ExecuteCommand(input)
}

func (r *SessionRuntime) SkillHint() (string, bool) {
	return r.bot.SkillHint()
}

func (r *SessionRuntime) Stats() view.BotStats {
	return r.bot.Stats()
}

func (r *SessionRuntime) CommandNames() []string {
	names := append([]string(nil), r.bot.CommandNames()...)
	names = append(names, "connect", "disconnect", "devices")
	return names
}

func (r *SessionRuntime) ProviderModel() (string, string) {
	return r.bot.ProviderModel()
}

func (r *SessionRuntime) SessionMessages() []view.DisplayMessage {
	return r.bot.SessionMessages()
}
