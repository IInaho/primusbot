package core

import (
	"time"

	"nekocode/protocol"
)

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

func (d ApprovalDecision) ConfirmReply() protocol.ConfirmReply {
	return protocol.ConfirmReply{
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

func (v ApprovalView) ToConfirmRequest() protocol.ConfirmRequest {
	return protocol.ConfirmRequest{
		ToolName:              v.ToolName,
		Args:                  v.Args,
		Kind:                  protocol.ConfirmKind(v.Kind),
		CanEscalatePermission: v.CanEscalatePermission,
	}
}

type QuestionStatus string

const (
	QuestionPending  QuestionStatus = "pending"
	QuestionAnswered QuestionStatus = "answered"
	QuestionRejected QuestionStatus = "rejected"
)

type QuestionView struct {
	ID         string                  `json:"id"`
	Questions  []protocol.QuestionItem `json:"questions"`
	Status     QuestionStatus          `json:"status"`
	CreatedAt  time.Time               `json:"created_at"`
	ResolvedAt *time.Time              `json:"resolved_at,omitempty"`
	Source     SourceRef               `json:"source"`
}

func (v QuestionView) ToQuestionRequest() protocol.QuestionRequest {
	return protocol.QuestionRequest{Questions: v.Questions}
}
