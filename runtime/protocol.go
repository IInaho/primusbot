// Package runtime provides the interaction control layer above the Bot core.
package runtime

import "nekocode/runtime/internal/core"

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
type CmdResult = core.CmdResult
type StepAction = core.StepAction
type StepEvent = core.StepEvent
type RunCallbacks = core.RunCallbacks
type ControlCallbacks = core.ControlCallbacks
type TodoItem = core.TodoItem
type BotStats = core.BotStats
type ContextSegment = core.ContextSegment
type ContextSnapshot = core.ContextSnapshot
type MemoryScope = core.MemoryScope
type MemorySection = core.MemorySection
type MemoryView = core.MemoryView
type SessionMeta = core.SessionMeta
type DisplayBlock = core.DisplayBlock
type ImageRef = core.ImageRef
type DisplayMessage = core.DisplayMessage
type ConfigView = core.ConfigView
type ModelConfig = core.ModelConfig
type ImageGenConfig = core.ImageGenConfig
type MCPServerConfig = core.MCPServerConfig
type PermissionsConfig = core.PermissionsConfig
type SandboxConfig = core.SandboxConfig
type WorkspaceConfig = core.WorkspaceConfig
type SkillManagementView = core.SkillManagementView
type SkillView = core.SkillView
type PluginView = core.PluginView
type MCPServerView = core.MCPServerView
type ToolPayload = core.ToolPayload
type DonePayload = core.DonePayload

const (
	CmdNone           = core.CmdNone
	CmdHandled        = core.CmdHandled
	CmdConfirming     = core.CmdConfirming
	CmdSessionResumed = core.CmdSessionResumed
)

const (
	StepActionChat           = core.StepActionChat
	StepActionThink          = core.StepActionThink
	StepActionToolStart      = core.StepActionToolStart
	StepActionToolBlocked    = core.StepActionToolBlocked
	StepActionToolPreview    = core.StepActionToolPreview
	StepActionExecuteTool    = core.StepActionExecuteTool
	StepActionSubToolStart   = core.StepActionSubToolStart
	StepActionSubExecuteTool = core.StepActionSubExecuteTool
	StepActionSubAgentStart  = core.StepActionSubAgentStart
	StepActionSubAgentEnd    = core.StepActionSubAgentEnd
)

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
type Runtime = core.Runtime
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

type QueryRuntime interface {
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

type ManagementRuntime interface {
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

const (
	MemoryScopeProject = core.MemoryScopeProject
)
