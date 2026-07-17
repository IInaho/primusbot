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
type ToolPayload = core.ToolPayload
type DonePayload = core.DonePayload

type ApprovalStatus = core.ApprovalStatus

const (
	ApprovalPending  = core.ApprovalPending
	ApprovalApproved = core.ApprovalApproved
	ApprovalRejected = core.ApprovalRejected
	ApprovalExpired  = core.ApprovalExpired
)

type ApprovalDecision = core.ApprovalDecision
type ApprovalView = core.ApprovalView
type Runtime = core.Runtime
type QueryRuntime = core.QueryRuntime
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
