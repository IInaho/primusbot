package runtime

import (
	commonview "nekocode/common/view"
	"nekocode/runtime/internal/core"
)

type RunID = core.RunID

type InputKind = core.InputKind

const (
	InputMessage       = core.InputMessage
	InputCommand       = core.InputCommand
	InputApprovalReply = core.InputApprovalReply
	InputQuestionReply = core.InputQuestionReply
)

type SourceRef = core.SourceRef
type SenderRef = core.SenderRef
type Input = core.Input

type RunStatus = core.RunStatus

const (
	RunIdle            = core.RunIdle
	RunRunning         = core.RunRunning
	RunWaitingApproval = core.RunWaitingApproval
	RunWaitingQuestion = core.RunWaitingQuestion
	RunDone            = core.RunDone
	RunFailed          = core.RunFailed
	RunAborted         = core.RunAborted
)

type EventType = core.EventType

const (
	EventInputAccepted     = core.EventInputAccepted
	EventSystemMessage     = core.EventSystemMessage
	EventAssistantDelta    = core.EventAssistantDelta
	EventReasoningDelta    = core.EventReasoningDelta
	EventPhaseChanged      = core.EventPhaseChanged
	EventToolStarted       = core.EventToolStarted
	EventToolBlocked       = core.EventToolBlocked
	EventToolPreview       = core.EventToolPreview
	EventToolCompleted     = core.EventToolCompleted
	EventSubAgentStarted   = core.EventSubAgentStarted
	EventSubAgentEnded     = core.EventSubAgentEnded
	EventTodosUpdated      = core.EventTodosUpdated
	EventApprovalRequested = core.EventApprovalRequested
	EventApprovalResolved  = core.EventApprovalResolved
	EventQuestionRequested = core.EventQuestionRequested
	EventQuestionResolved  = core.EventQuestionResolved
	EventRunStarted        = core.EventRunStarted
	EventRunDone           = core.EventRunDone
	EventRunFailed         = core.EventRunFailed
	EventRunAborted        = core.EventRunAborted
	EventSessionResumed    = core.EventSessionResumed
	EventConnectorStatus   = core.EventConnectorStatus
)

type Event = core.Event
type EventFilter = core.EventFilter
type MessagePayload = core.MessagePayload
type DeltaPayload = core.DeltaPayload
type PhasePayload = core.PhasePayload
type TodoItem = commonview.TodoItem
type BotStats = commonview.BotStats
type ContextSnapshot = commonview.ContextSnapshot
type MemoryScope = commonview.MemoryScope
type MemorySection = commonview.MemorySection
type MemoryView = commonview.MemoryView
type SessionMeta = commonview.SessionMeta
type DisplayMessage = commonview.DisplayMessage
type ConfigView = commonview.ConfigView
type SkillManagementView = commonview.SkillManagementView
type ToolPayload = core.ToolPayload
type DonePayload = core.DonePayload

type ApprovalStatus = core.ApprovalStatus

const (
	ApprovalPending  = core.ApprovalPending
	ApprovalApproved = core.ApprovalApproved
	ApprovalRejected = core.ApprovalRejected
	ApprovalExpired  = core.ApprovalExpired
)

type ConfirmKind = core.ConfirmKind

const (
	ConfirmKindPermission = core.ConfirmKindPermission
	ConfirmKindInstall    = core.ConfirmKindInstall
)

type ConfirmRequest = core.ConfirmRequest
type ConfirmReply = core.ConfirmReply
type ApprovalDecision = core.ApprovalDecision
type ApprovalView = core.ApprovalView
type Control = core.Control
type ConnectorRuntime = core.ConnectorRuntime
type RunView = core.RunView

type ToolStatus = core.ToolStatus

const (
	ToolRunning = core.ToolRunning
	ToolDone    = core.ToolDone
	ToolBlocked = core.ToolBlocked
)

type ToolView = core.ToolView
type ArtifactView = core.ArtifactView
type ArtifactItem = core.ArtifactItem
type ConnectorStatusPayload = core.ConnectorStatusPayload
type ConnectView = core.ConnectView
type ConnectorView = core.ConnectorView
type ConnectorDeviceView = core.ConnectorDeviceView

type QuestionStatus = core.QuestionStatus

const (
	QuestionPending  = core.QuestionPending
	QuestionAnswered = core.QuestionAnswered
	QuestionRejected = core.QuestionRejected
)

type QuestionView = core.QuestionView
type QuestionOption = core.QuestionOption
type QuestionItem = core.QuestionItem
type QuestionReply = core.QuestionReply
type QuestionRequest = core.QuestionRequest

type Query interface {
	CurrentRunView() (RunView, bool)
	RunView(runID RunID) (RunView, bool)
	ListRunViews(limit int) []RunView
	ArtifactView(runID RunID) (ArtifactView, bool)
	ConnectView() ConnectView
	Stats() BotStats
	ContextSnapshot() ContextSnapshot
	MemoryView(scope MemoryScope) MemoryView
	ListSessions() []SessionMeta
	SessionMessages() []DisplayMessage
	CommandNames() []string
	ProviderModel() (provider, model string)
}

type Management interface {
	SwitchModel(name string) (model, provider string, err error)
	ContextStatus() string
	ContextReport() string
	SelectSkill(name string) error
	ClearSelectedSkill()
	ConfigView() ConfigView
	ApplyConfig(cfg ConfigView) (ConfigView, error)
	SkillManagementView() SkillManagementView
	RefreshSkillManagement() SkillManagementView
	SetPluginEnabled(name string, enabled bool) (SkillManagementView, error)
	CWD() string
	ClearContext()
	CurrentSessionID() string
	SetSession(id string) error
	ResumeSession(id string) error
	ListSessions() []SessionMeta
	NewSession() (SessionMeta, error)
	DeleteSession(id string) error
}

// Client is the full application-facing view of one Manager instance.
type Client interface {
	Control
	Query
	Management
	Close()
}

const (
	MemoryScopeProject = commonview.MemoryScopeProject
)
