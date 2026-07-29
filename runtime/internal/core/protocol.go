// Package core defines the stable runtime protocol shared by the public
// facade and internal services.
package core

import (
	"context"
	"time"
)

type RunID string

type InputKind string

const (
	InputMessage       InputKind = "message"
	InputCommand       InputKind = "command"
	InputApprovalReply InputKind = "approval_reply"
	InputQuestionReply InputKind = "question_reply"
)

type SourceRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}

type SenderRef struct {
	ID       string `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
	Display  string `json:"display,omitempty"`
}

type Input struct {
	ID        string    `json:"id,omitempty"`
	Kind      InputKind `json:"kind"`
	Source    SourceRef `json:"source"`
	Sender    SenderRef `json:"sender"`
	Text      string    `json:"text"`
	SessionID string    `json:"session_id,omitempty"`
	ReplyTo   string    `json:"reply_to,omitempty"`
}

type RunStatus string

const (
	RunIdle            RunStatus = "idle"
	RunRunning         RunStatus = "running"
	RunWaitingApproval RunStatus = "waiting_approval"
	RunWaitingQuestion RunStatus = "waiting_question"
	RunDone            RunStatus = "done"
	RunFailed          RunStatus = "failed"
	RunAborted         RunStatus = "aborted"
)

type EventType string

const (
	EventInputAccepted     EventType = "input_accepted"
	EventSystemMessage     EventType = "system_message"
	EventAssistantDelta    EventType = "assistant_delta"
	EventReasoningDelta    EventType = "reasoning_delta"
	EventPhaseChanged      EventType = "phase_changed"
	EventToolStarted       EventType = "tool_started"
	EventToolBlocked       EventType = "tool_blocked"
	EventToolPreview       EventType = "tool_preview"
	EventToolCompleted     EventType = "tool_completed"
	EventSubAgentStarted   EventType = "subagent_started"
	EventSubAgentEnded     EventType = "subagent_ended"
	EventTodosUpdated      EventType = "todos_updated"
	EventApprovalRequested EventType = "approval_requested"
	EventApprovalResolved  EventType = "approval_resolved"
	EventQuestionRequested EventType = "question_requested"
	EventQuestionResolved  EventType = "question_resolved"
	EventRunStarted        EventType = "run_started"
	EventRunDone           EventType = "run_done"
	EventRunFailed         EventType = "run_failed"
	EventRunAborted        EventType = "run_aborted"
	EventSessionResumed    EventType = "session_resumed"
	EventConnectorStatus   EventType = "connector_status"
)

type Event struct {
	ID        string    `json:"id"`
	RunID     RunID     `json:"run_id,omitempty"`
	Type      EventType `json:"type"`
	Source    SourceRef `json:"source"`
	Time      time.Time `json:"time"`
	Payload   any       `json:"payload,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
}

type EventFilter struct {
	RunID   RunID
	Types   []EventType
	Sources []string
}

type MessagePayload struct {
	Role    string    `json:"role"`
	Content string    `json:"content"`
	Source  SourceRef `json:"source"`
	Sender  SenderRef `json:"sender"`
}

type DeltaPayload struct {
	Delta string `json:"delta"`
	Done  bool   `json:"done,omitempty"`
}

type PhasePayload struct {
	Phase string `json:"phase"`
}

type ToolPayload struct {
	ToolName string `json:"tool_name"`
	CallID   string `json:"call_id,omitempty"`
	Args     string `json:"args,omitempty"`
	Output   string `json:"output,omitempty"`
	Preview  string `json:"preview,omitempty"`
	IsError  bool   `json:"is_error,omitempty"`
}

type DonePayload struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// Control is the application-facing interaction contract.
type Control interface {
	Submit(ctx context.Context, input Input) (RunID, error)
	Steer(ctx context.Context, runID RunID, input Input) error
	Abort(ctx context.Context, runID RunID) error
	Approve(ctx context.Context, approvalID string, decision ApprovalDecision) error
	Answer(ctx context.Context, questionID string, reply QuestionReply) error
	Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, error)
	Connect(ctx context.Context, name string, args []string) (string, error)
	Disconnect(name string) (string, error)
}

// ConnectorRuntime is the narrow host contract supplied to IM connectors.
type ConnectorRuntime interface {
	Submit(ctx context.Context, input Input) (RunID, error)
	Abort(ctx context.Context, runID RunID) error
	Approve(ctx context.Context, approvalID string, decision ApprovalDecision) error
	Answer(ctx context.Context, questionID string, reply QuestionReply) error
	Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, error)
	ReportConnectorStatus(payload ConnectorStatusPayload)
}
