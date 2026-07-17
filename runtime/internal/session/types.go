package session

import (
	"nekocode/runtime/internal/botcore"
	"nekocode/runtime/internal/broker"
	"nekocode/runtime/internal/connectors"
	"nekocode/runtime/internal/core"
	"nekocode/runtime/internal/eventbus"
	"nekocode/runtime/internal/recording"
	"nekocode/runtime/internal/redaction"
	"nekocode/runtime/internal/runstore"
)

type RuntimeBot = botcore.RuntimeBot
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
type ToolPayload = core.ToolPayload
type DonePayload = core.DonePayload
type ApprovalDecision = core.ApprovalDecision
type ApprovalView = core.ApprovalView
type RunView = core.RunView
type ArtifactView = core.ArtifactView
type ConnectView = core.ConnectView
type ConnectorStatusPayload = core.ConnectorStatusPayload
type QuestionView = core.QuestionView

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
