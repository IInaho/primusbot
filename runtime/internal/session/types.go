package session

import (
	"nekocode/runtime/internal/broker"
	"nekocode/runtime/internal/connectors"
	"nekocode/runtime/internal/core"
	"nekocode/runtime/internal/eventbus"
	"nekocode/runtime/internal/recording"
	"nekocode/runtime/internal/redaction"
	"nekocode/runtime/internal/runstore"
)

type AgentRunner interface {
	Run(input string, callbacks RunCallbacks) (string, error)
	ConfigureRuntime(callbacks ControlCallbacks)
}

type CommandExecutor interface {
	ExecuteCommand(input string) (string, CmdResult)
}

type SkillHintProvider interface {
	SkillHint() (string, bool)
}

type CommandCatalog interface {
	CommandNames() []string
}

type RunController interface {
	Steer(msg string)
	Abort()
	Close()
}

type StatsProvider interface {
	Stats() BotStats
}

type ModelInfoProvider interface {
	ProviderModel() (provider, model string)
}

type MessageProvider interface {
	SessionMessages() []DisplayMessage
}

type EventBus = eventbus.EventBus
type ApprovalBroker = broker.ApprovalBroker
type QuestionBroker = broker.QuestionBroker
type RunStore = runstore.RunStore
type EventRecorder = recording.EventRecorder
type ConnectorFactory = connectors.ConnectorFactory

type RunID = core.RunID
type InputKind = core.InputKind
type SourceRef = core.SourceRef
type SenderRef = core.SenderRef
type Input = core.Input
type RunStatus = core.RunStatus
type EventType = core.EventType
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
type DisplayMessage = core.DisplayMessage
type ToolPayload = core.ToolPayload
type DonePayload = core.DonePayload
type ConfirmKind = core.ConfirmKind
type ConfirmRequest = core.ConfirmRequest
type ConfirmReply = core.ConfirmReply
type ApprovalDecision = core.ApprovalDecision
type ApprovalView = core.ApprovalView
type RunView = core.RunView
type ArtifactView = core.ArtifactView
type ConnectView = core.ConnectView
type ConnectorStatusPayload = core.ConnectorStatusPayload
type QuestionView = core.QuestionView
type QuestionOption = core.QuestionOption
type QuestionItem = core.QuestionItem
type QuestionReply = core.QuestionReply
type QuestionRequest = core.QuestionRequest

const (
	InputMessage = core.InputMessage

	RunIdle            = core.RunIdle
	RunRunning         = core.RunRunning
	RunWaitingApproval = core.RunWaitingApproval
	RunWaitingQuestion = core.RunWaitingQuestion
	RunDone            = core.RunDone
	RunFailed          = core.RunFailed
	RunAborted         = core.RunAborted

	EventInputAccepted     = core.EventInputAccepted
	EventAssistantDelta    = core.EventAssistantDelta
	EventReasoningDelta    = core.EventReasoningDelta
	EventSystemMessage     = core.EventSystemMessage
	EventPhaseChanged      = core.EventPhaseChanged
	EventTodosUpdated      = core.EventTodosUpdated
	EventToolStarted       = core.EventToolStarted
	EventToolPreview       = core.EventToolPreview
	EventToolCompleted     = core.EventToolCompleted
	EventToolBlocked       = core.EventToolBlocked
	EventSubAgentStarted   = core.EventSubAgentStarted
	EventSubAgentEnded     = core.EventSubAgentEnded
	EventApprovalRequested = core.EventApprovalRequested
	EventQuestionRequested = core.EventQuestionRequested
	EventRunStarted        = core.EventRunStarted
	EventRunDone           = core.EventRunDone
	EventRunFailed         = core.EventRunFailed
	EventRunAborted        = core.EventRunAborted
	EventSessionResumed    = core.EventSessionResumed
	EventConnectorStatus   = core.EventConnectorStatus

	CmdNone           = core.CmdNone
	CmdHandled        = core.CmdHandled
	CmdConfirming     = core.CmdConfirming
	CmdSessionResumed = core.CmdSessionResumed

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

	ConfirmKindPermission = core.ConfirmKindPermission
	ConfirmKindInstall    = core.ConfirmKindInstall
)

func NewEventBus() *EventBus {
	return eventbus.NewEventBus()
}

func NewRunStore(limit int) *RunStore {
	return runstore.NewRunStore(limit)
}

func NewApprovalBroker(eventBus *EventBus, source SourceRef, runID func() RunID) *ApprovalBroker {
	return broker.NewApprovalBroker(eventBus, source, runID)
}

func NewQuestionBroker(eventBus *EventBus, source SourceRef, runID func() RunID) *QuestionBroker {
	return broker.NewQuestionBroker(eventBus, source, runID)
}

func NewEventRecorder(baseDir string) (*EventRecorder, error) {
	return recording.NewEventRecorder(baseDir)
}

func LoadRecordedEvents(baseDir string) ([]Event, error) {
	return recording.LoadRecordedEvents(baseDir)
}

func defaultEventRecorderBaseDir() string {
	return recording.DefaultBaseDir()
}

func RedactInputText(input string) string {
	return redaction.RedactInputText(input)
}
