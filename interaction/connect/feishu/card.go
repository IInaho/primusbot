package feishu

import (
	"fmt"
	"strings"

	larkcallback "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"

	controlruntime "nekocode/runtime"
)

// Approval card actions: the button value carries exactly these two fields
// (the approval ID passes through verbatim; nothing user-supplied is ever
// concatenated into it).
const (
	decisionApprove  = "approve"  // allow once
	decisionRemember = "remember" // allow and remember (TUI: always allow)
	decisionReject   = "reject"   // deny
	decisionEscalate = "escalate" // allow and pre-authorize permission escalation
	valueKeyID       = "approval_id"
	valueKeyAction   = "decision"
)

// approvalDecisionFor maps a card decision to the runtime decision model.
func approvalDecisionFor(decision string) (controlruntime.ApprovalDecision, error) {
	switch decision {
	case decisionApprove:
		return controlruntime.ApprovalDecision{Allowed: true}, nil
	case decisionRemember:
		return controlruntime.ApprovalDecision{Allowed: true, Remember: true}, nil
	case decisionReject:
		return controlruntime.ApprovalDecision{Allowed: false}, nil
	case decisionEscalate:
		return controlruntime.ApprovalDecision{Allowed: true, AllowWithPermission: true}, nil
	default:
		return controlruntime.ApprovalDecision{}, fmt.Errorf("unknown decision %q", decision)
	}
}

// verdict renders the post-decision verdict text and header template for
// each of the four decision semantics.
func verdict(decision string) (text, template string) {
	switch decision {
	case decisionRemember:
		return "已永久允许", "green"
	case decisionReject:
		return "已拒绝", "red"
	case decisionEscalate:
		return "已批准并授权", "green"
	default:
		return "已批准", "green"
	}
}

// approvalCard builds the interactive approval card (msg_type=interactive):
// a header with the tool name, a summary body, and the decision buttons
// whose action values carry the approval ID and decision. The fourth
// "allow and escalate" button only appears when the approval view allows
// permission escalation.
func approvalCard(p controlruntime.ApprovalView) map[string]any {
	buttons := []any{
		cardButton("批准一次", "default", p.ID, decisionApprove),
		cardButton("永久允许", "primary", p.ID, decisionRemember),
		cardButton("拒绝", "danger", p.ID, decisionReject),
	}
	if p.CanEscalatePermission {
		buttons = append(buttons, cardButton("允许并授权", "primary", p.ID, decisionEscalate))
	}
	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"template": "blue",
			"title":    map[string]any{"tag": "plain_text", "content": "需要审批: " + p.ToolName},
		},
		"elements": []any{
			map[string]any{
				"tag":  "div",
				"text": map[string]any{"tag": "lark_md", "content": approvalSummary(p)},
			},
			map[string]any{
				"tag":     "action",
				"actions": buttons,
			},
		},
	}
}

func cardButton(text, style, approvalID, decision string) map[string]any {
	return map[string]any{
		"tag":   "button",
		"text":  map[string]any{"tag": "plain_text", "content": text},
		"type":  style,
		"value": cardActionValue(approvalID, decision),
	}
}

// resolvedCard renders the post-decision replacement card: same summary, a
// verdict line, and no buttons (prevents repeat clicks).
func resolvedCard(p controlruntime.ApprovalView, decision string) map[string]any {
	text, template := verdict(decision)
	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"template": template,
			"title":    map[string]any{"tag": "plain_text", "content": text + ": " + p.ToolName},
		},
		"elements": []any{
			map[string]any{
				"tag":  "div",
				"text": map[string]any{"tag": "lark_md", "content": approvalSummary(p)},
			},
		},
	}
}

// approvalSummary renders the one-block tool detail, mirroring the plain
// text approval rendering in render.go.
func approvalSummary(p controlruntime.ApprovalView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**工具**：%s", p.ToolName)
	if p.Kind != "" {
		fmt.Fprintf(&b, "\n**类型**：%s", p.Kind)
	}
	if cmd, ok := p.Args["command"].(string); ok && cmd != "" {
		fmt.Fprintf(&b, "\n**命令**：\n```\n%s\n```", truncateRunes(cmd, 600))
		return b.String()
	}
	if path, ok := p.Args["path"].(string); ok && path != "" {
		fmt.Fprintf(&b, "\n**路径**：`%s`", path)
	}
	if preview, ok := p.Args["_preview"].(string); ok && preview != "" {
		fmt.Fprintf(&b, "\n**预览**：\n```\n%s\n```", truncateRunes(preview, 900))
	}
	return b.String()
}

// cardActionValue encodes the button action value.
func cardActionValue(approvalID, decision string) map[string]interface{} {
	return map[string]interface{}{valueKeyID: approvalID, valueKeyAction: decision}
}

// decodeCardActionValue extracts and validates the approval ID and decision
// from a card action value.
func decodeCardActionValue(value map[string]interface{}) (approvalID, decision string, err error) {
	id, _ := value[valueKeyID].(string)
	if id == "" {
		return "", "", fmt.Errorf("card action missing %q", valueKeyID)
	}
	dec, _ := value[valueKeyAction].(string)
	if _, err := approvalDecisionFor(dec); err != nil {
		return "", "", fmt.Errorf("card action has unknown decision %q", dec)
	}
	return id, dec, nil
}

// toastResponse builds a toast-only card action response.
func toastResponse(toastType, content string) *larkcallback.CardActionTriggerResponse {
	return &larkcallback.CardActionTriggerResponse{
		Toast: &larkcallback.Toast{Type: toastType, Content: content},
	}
}

// isAlreadyResolvedErr reports whether an Approve error means the approval
// was already decided (double click, or handled via the text command).
func isAlreadyResolvedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "already resolved") || strings.Contains(msg, "not pending")
}
