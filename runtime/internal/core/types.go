// Package core defines runtime protocol types shared by the runtime facade and
// internal runtime services.
package core

import (
	"context"
	"time"

	commonview "nekocode/common/view"
)

// ---------------------------------------------------------------------------
// Runtime-specific protocol types
// ---------------------------------------------------------------------------

type RunID string

type InputKind string

const (
	InputMessage       InputKind = "message"
	InputCommand       InputKind = "command"
	InputApprovalReply InputKind = "approval_reply"
	InputQuestionReply InputKind = "question_reply"
)

type SourceRef struct {
	Kind string `json:"kind"` // tui, gui, telegram, web, sdk
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

type CmdResult = commonview.CmdResult

const (
	CmdNone           = commonview.CmdNone
	CmdHandled        = commonview.CmdHandled
	CmdConfirming     = commonview.CmdConfirming
	CmdSessionResumed = commonview.CmdSessionResumed
)

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

type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
	ApprovalExpired  ApprovalStatus = "expired"
)

type ApprovalDecision struct {
	Allowed             bool `json:"allowed"`
	Remember            bool `json:"remember,omitempty"`
	AllowWithPermission bool `json:"allow_with_permission,omitempty"`
}

func (d ApprovalDecision) ConfirmReply() ConfirmReply {
	return ConfirmReply{
		Allowed:             d.Allowed,
		Remember:            d.Remember,
		AllowWithPermission: d.AllowWithPermission,
	}
}

type ApprovalView struct {
	ID                    string         `json:"id"`
	ToolName              string         `json:"tool_name"`
	Args                  map[string]any `json:"args,omitempty"`
	ArgsHash              string         `json:"args_hash,omitempty"`
	ToolCallHash          string         `json:"tool_call_hash,omitempty"`
	Kind                  string         `json:"kind,omitempty"`
	CanEscalatePermission bool           `json:"can_escalate_permission,omitempty"`
	Status                ApprovalStatus `json:"status"`
	CreatedAt             time.Time      `json:"created_at"`
	ResolvedAt            *time.Time     `json:"resolved_at,omitempty"`
	Source                SourceRef      `json:"source"`
	Metadata              map[string]any `json:"metadata,omitempty"`
}

func (v ApprovalView) ToConfirmRequest() ConfirmRequest {
	return ConfirmRequest{
		ToolName:              v.ToolName,
		Args:                  v.Args,
		Kind:                  ConfirmKind(v.Kind),
		CanEscalatePermission: v.CanEscalatePermission,
	}
}

type Runtime interface {
	Submit(ctx context.Context, input Input) (RunID, error)
	Steer(ctx context.Context, runID RunID, input Input) error
	Abort(ctx context.Context, runID RunID) error
	Approve(ctx context.Context, approvalID string, decision ApprovalDecision) error
	Answer(ctx context.Context, questionID string, reply QuestionReply) error
	Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, error)
	Connect(ctx context.Context, name string, args []string) (string, error)
	Disconnect(name string) (string, error)
	// Publish injects an event into the runtime event stream (e.g. connector status).
	Publish(ev Event)
}

type RunView struct {
	ID         RunID          `json:"id"`
	Status     RunStatus      `json:"status"`
	Source     SourceRef      `json:"source"`
	Sender     SenderRef      `json:"sender"`
	Input      string         `json:"input,omitempty"`
	Phase      string         `json:"phase,omitempty"`
	StartedAt  time.Time      `json:"started_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	Tools      []ToolView     `json:"tools,omitempty"`
	Approvals  []ApprovalView `json:"approvals,omitempty"`
	Questions  []QuestionView `json:"questions,omitempty"`
	Output     string         `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	EventCount int            `json:"event_count"`
}

type ToolStatus string

const (
	ToolRunning ToolStatus = "running"
	ToolDone    ToolStatus = "done"
	ToolBlocked ToolStatus = "blocked"
)

type ToolView struct {
	Name       string     `json:"name"`
	Args       string     `json:"args,omitempty"`
	Preview    string     `json:"preview,omitempty"`
	Output     string     `json:"output,omitempty"`
	Status     ToolStatus `json:"status"`
	IsError    bool       `json:"is_error,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type ArtifactView struct {
	RunID   RunID          `json:"run_id"`
	Diffs   []ArtifactItem `json:"diffs,omitempty"`
	Patches []ArtifactItem `json:"patches,omitempty"`
	Reviews []ArtifactItem `json:"reviews,omitempty"`
	Results []ArtifactItem `json:"results,omitempty"`
	Errors  []ArtifactItem `json:"errors,omitempty"`
	Events  int            `json:"events"`
}

type ArtifactItem struct {
	Kind      string    `json:"kind"`
	ToolName  string    `json:"tool_name,omitempty"`
	Title     string    `json:"title,omitempty"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type ConnectorStatusPayload struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ConnectView struct {
	Connectors []ConnectorView `json:"connectors"`
}

type ConnectorView struct {
	Name        string                `json:"name"`
	Registered  bool                  `json:"registered"`
	Initialized bool                  `json:"initialized"`
	Configured  bool                  `json:"configured"`
	Running     bool                  `json:"running"`
	Status      string                `json:"status"`
	Message     string                `json:"message,omitempty"`
	Devices     []ConnectorDeviceView `json:"devices,omitempty"`
	Metadata    map[string]any        `json:"metadata,omitempty"`
}

type ConnectorDeviceView struct {
	ID       string `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
	Display  string `json:"display,omitempty"`
	LastSeen int64  `json:"last_seen,omitempty"`
	PairedAt int64  `json:"paired_at,omitempty"`
}

type QuestionStatus string

const (
	QuestionPending  QuestionStatus = "pending"
	QuestionAnswered QuestionStatus = "answered"
	QuestionRejected QuestionStatus = "rejected"
)

type QuestionView struct {
	ID         string         `json:"id"`
	Questions  []QuestionItem `json:"questions"`
	Status     QuestionStatus `json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
	ResolvedAt *time.Time     `json:"resolved_at,omitempty"`
	Source     SourceRef      `json:"source"`
}

func (v QuestionView) ToQuestionRequest() QuestionRequest {
	return QuestionRequest{Questions: v.Questions}
}

// ---------------------------------------------------------------------------
// Shared UI-facing DTO types (aliased from common/view)
// ---------------------------------------------------------------------------

type StepAction = commonview.StepAction
type StepEvent = commonview.StepEvent

const (
	StepActionChat           = commonview.StepActionChat
	StepActionThink          = commonview.StepActionThink
	StepActionToolStart      = commonview.StepActionToolStart
	StepActionToolBlocked    = commonview.StepActionToolBlocked
	StepActionToolPreview    = commonview.StepActionToolPreview
	StepActionExecuteTool    = commonview.StepActionExecuteTool
	StepActionSubToolStart   = commonview.StepActionSubToolStart
	StepActionSubExecuteTool = commonview.StepActionSubExecuteTool
	StepActionSubAgentStart  = commonview.StepActionSubAgentStart
	StepActionSubAgentEnd    = commonview.StepActionSubAgentEnd
)

type RunCallbacks = commonview.RunCallbacks

type ControlCallbacks = commonview.ControlCallbacks

type TodoItem = commonview.TodoItem

type BotStats = commonview.BotStats

type ContextSegment = commonview.ContextSegment

type ContextSnapshot = commonview.ContextSnapshot

type MemoryScope = commonview.MemoryScope

const (
	MemoryScopeProject = commonview.MemoryScopeProject
)

type MemorySection = commonview.MemorySection

type MemoryView = commonview.MemoryView

type SessionMeta = commonview.SessionMeta

type DisplayBlock = commonview.DisplayBlock

type ImageRef = commonview.ImageRef

type DisplayMessage = commonview.DisplayMessage

type ConfigView = commonview.ConfigView

type ModelConfig = commonview.ModelConfig

type ImageGenConfig = commonview.ImageGenConfig

type MCPServerConfig = commonview.MCPServerConfig

type PermissionsConfig = commonview.PermissionsConfig

type SandboxConfig = commonview.SandboxConfig

type WorkspaceConfig = commonview.WorkspaceConfig

type SkillManagementView = commonview.SkillManagementView

type SkillView = commonview.SkillView

type PluginView = commonview.PluginView

type MCPServerView = commonview.MCPServerView

type ConfirmKind = commonview.ConfirmKind

const (
	ConfirmKindPermission = commonview.ConfirmKindPermission
	ConfirmKindInstall    = commonview.ConfirmKindInstall
)

type ConfirmRequest = commonview.ConfirmRequest

type ConfirmReply = commonview.ConfirmReply

type QuestionOption = commonview.QuestionOption

type QuestionItem = commonview.QuestionItem

type QuestionReply = commonview.QuestionReply

type QuestionRequest = commonview.QuestionRequest
