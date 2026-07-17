package broker

import "nekocode/runtime/internal/core"

type RunID = core.RunID
type SourceRef = core.SourceRef
type Event = core.Event
type EventType = core.EventType
type ApprovalDecision = core.ApprovalDecision
type ApprovalStatus = core.ApprovalStatus
type ApprovalView = core.ApprovalView
type QuestionStatus = core.QuestionStatus
type QuestionView = core.QuestionView

const (
	EventApprovalRequested = core.EventApprovalRequested
	EventApprovalResolved  = core.EventApprovalResolved
	EventQuestionRequested = core.EventQuestionRequested
	EventQuestionResolved  = core.EventQuestionResolved

	ApprovalPending  = core.ApprovalPending
	ApprovalApproved = core.ApprovalApproved
	ApprovalRejected = core.ApprovalRejected

	QuestionPending  = core.QuestionPending
	QuestionAnswered = core.QuestionAnswered
	QuestionRejected = core.QuestionRejected
)
