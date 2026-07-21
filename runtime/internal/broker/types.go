package broker

import "nekocode/runtime/internal/core"

type RunID = core.RunID
type SourceRef = core.SourceRef
type Event = core.Event
type EventType = core.EventType
type ConfirmKind = core.ConfirmKind
type ConfirmRequest = core.ConfirmRequest
type ConfirmReply = core.ConfirmReply
type ApprovalDecision = core.ApprovalDecision
type ApprovalStatus = core.ApprovalStatus
type ApprovalView = core.ApprovalView
type QuestionStatus = core.QuestionStatus
type QuestionView = core.QuestionView
type QuestionReply = core.QuestionReply
type QuestionRequest = core.QuestionRequest

const (
	EventApprovalRequested = core.EventApprovalRequested
	EventApprovalResolved  = core.EventApprovalResolved
	EventQuestionRequested = core.EventQuestionRequested
	EventQuestionResolved  = core.EventQuestionResolved

	ConfirmKindPermission = core.ConfirmKindPermission
	ConfirmKindInstall    = core.ConfirmKindInstall

	ApprovalPending  = core.ApprovalPending
	ApprovalApproved = core.ApprovalApproved
	ApprovalRejected = core.ApprovalRejected

	QuestionPending  = core.QuestionPending
	QuestionAnswered = core.QuestionAnswered
	QuestionRejected = core.QuestionRejected
)
