package connect

import (
	"fmt"
	"strings"

	controlruntime "nekocode/runtime"
)

// Translate converts a runtime event into zero or more outbound intents.
// It is the single place where push policy lives: only previews, run
// outcomes, and pending/resolved decisions produce intents. Progress
// events (tool lifecycle, phases, todos, sub-agents, metrics, reasoning)
// never leave the process — they are dashboard state for the TUI, not
// chat messages; pushed to an IM they become stale noise in a permanent
// chat history.
func Translate(ev controlruntime.Event) []Intent {
	switch ev.Type {
	case controlruntime.EventAssistantDelta:
		p, ok := ev.Payload.(controlruntime.DeltaPayload)
		if !ok || p.Delta == "" {
			return nil
		}
		return []Intent{{Kind: IntentPreview, RunID: ev.RunID, Text: p.Delta}}

	case controlruntime.EventSystemMessage:
		// Command output (e.g. /connect, /model) is a terminal result, not
		// a progress event — it must reach the chat. Progress events are
		// intentionally dropped above/below; this one is the command's reply.
		p, ok := ev.Payload.(controlruntime.MessagePayload)
		if !ok || strings.TrimSpace(p.Content) == "" {
			return nil
		}
		return []Intent{{Kind: IntentSystem, RunID: ev.RunID, Text: p.Content}}

	case controlruntime.EventRunDone:
		p, ok := ev.Payload.(controlruntime.RunResult)
		if !ok {
			return nil
		}
		if strings.TrimSpace(p.Output) != "" {
			return []Intent{{Kind: IntentResult, RunID: ev.RunID, Text: p.Output}}
		}
		if p.Error != "" {
			return []Intent{{Kind: IntentFailed, RunID: ev.RunID, Text: "运行失败: " + p.Error}}
		}
		return nil

	case controlruntime.EventRunFailed:
		p, ok := ev.Payload.(controlruntime.RunResult)
		if !ok {
			return nil
		}
		text := "运行失败"
		if strings.TrimSpace(p.Error) != "" {
			text = "运行失败: " + p.Error
		}
		return []Intent{{Kind: IntentFailed, RunID: ev.RunID, Text: text}}

	case controlruntime.EventRunCancelled:
		return []Intent{{Kind: IntentStopped, RunID: ev.RunID, Text: "已停止"}}

	case controlruntime.EventApprovalRequested:
		p, ok := ev.Payload.(controlruntime.ApprovalView)
		if !ok || p.ID == "" {
			return nil
		}
		view := p
		return []Intent{{
			Kind:     IntentApproval,
			RunID:    ev.RunID,
			ID:       p.ID,
			Text:     ApprovalText(p),
			Actions:  ApprovalActions(p),
			Approval: &view,
		}}

	case controlruntime.EventApprovalResolved:
		p, ok := ev.Payload.(controlruntime.ApprovalView)
		if !ok || p.ID == "" {
			return nil
		}
		view := p
		return []Intent{{
			Kind:     IntentApprovalResolved,
			RunID:    ev.RunID,
			ID:       p.ID,
			Verdict:  ApprovalVerdict(p.Status),
			Approval: &view,
		}}

	case controlruntime.EventQuestionRequested:
		p, ok := ev.Payload.(controlruntime.QuestionView)
		if !ok || p.ID == "" || len(p.Questions) == 0 {
			return nil
		}
		view := p
		return []Intent{{
			Kind:     IntentQuestion,
			RunID:    ev.RunID,
			ID:       p.ID,
			Text:     QuestionText(p),
			Actions:  QuestionActions(p),
			Question: &view,
		}}

	case controlruntime.EventQuestionResolved:
		p, ok := ev.Payload.(controlruntime.QuestionView)
		if !ok || p.ID == "" {
			return nil
		}
		view := p
		return []Intent{{
			Kind:     IntentQuestionResolved,
			RunID:    ev.RunID,
			ID:       p.ID,
			Verdict:  "已回答",
			Question: &view,
		}}
	}
	return nil
}

// ApprovalActions is the canonical decision set for a pending approval,
// shared by inline keyboards, feishu cards, and slash-command help.
func ApprovalActions(p controlruntime.ApprovalView) []Action {
	actions := []Action{
		{ID: ActionOnce, Label: "批准一次"},
		{ID: ActionAlways, Label: "永久允许"},
		{ID: ActionReject, Label: "拒绝"},
	}
	if p.CanEscalatePermission {
		actions = append(actions, Action{ID: ActionEscalate, Label: "允许并授权"})
	}
	return actions
}

// QuestionActions returns the clickable decision set for a question: the
// options of a single selectable question plus dismiss (and confirm for
// multi-select). Free-form or multi-part questions return nil — they are
// answered with /answer text.
func QuestionActions(p controlruntime.QuestionView) []Action {
	if !QuestionSelectable(p) {
		return nil
	}
	q := p.Questions[0]
	actions := make([]Action, 0, len(q.Options)+2)
	for i, opt := range q.Options {
		actions = append(actions, Action{ID: fmt.Sprint(i), Label: opt.Label})
	}
	if q.Multiple {
		actions = append(actions, Action{ID: ActionConfirm, Label: "确认"})
	}
	actions = append(actions, Action{ID: ActionDismiss, Label: "忽略"})
	return actions
}

// QuestionSelectable reports whether a question can be answered by picking
// options (single question, at least one option) as opposed to free text.
func QuestionSelectable(p controlruntime.QuestionView) bool {
	return len(p.Questions) == 1 && len(p.Questions[0].Options) > 0
}

// ApprovalDecisionFor maps a canonical action ID to a runtime decision.
func ApprovalDecisionFor(action string) (controlruntime.ApprovalDecision, error) {
	switch action {
	case ActionOnce:
		return controlruntime.ApprovalDecision{Allowed: true}, nil
	case ActionAlways:
		return controlruntime.ApprovalDecision{Allowed: true, Remember: true}, nil
	case ActionEscalate:
		return controlruntime.ApprovalDecision{Allowed: true, AllowWithPermission: true}, nil
	case ActionReject:
		return controlruntime.ApprovalDecision{}, nil
	}
	return controlruntime.ApprovalDecision{}, fmt.Errorf("unknown approval action %q", action)
}

// VerdictForAction maps a canonical action ID to its human verdict text.
func VerdictForAction(action string) string {
	switch action {
	case ActionOnce:
		return "已批准"
	case ActionAlways:
		return "已永久允许"
	case ActionEscalate:
		return "已批准并授权"
	case ActionReject:
		return "已拒绝"
	}
	return "已处理"
}

// ApprovalVerdict maps a resolved approval status to canonical verdict text.
func ApprovalVerdict(s controlruntime.ApprovalStatus) string {
	switch s {
	case controlruntime.ApprovalApproved:
		return "已批准"
	case controlruntime.ApprovalRejected:
		return "已拒绝"
	}
	return "已处理"
}

// IsResolvedErr reports whether err means the approval/question was already
// resolved (here or on another surface) — i.e. the caller should show the
// terminal state rather than an error.
func IsResolvedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not pending") ||
		strings.Contains(msg, "already resolved") ||
		strings.Contains(msg, "not found")
}
