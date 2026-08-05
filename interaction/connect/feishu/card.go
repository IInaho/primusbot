package feishu

import (
	"fmt"
	"strings"

	larkcallback "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"

	"nekocode/interaction/connect"
	controlruntime "nekocode/runtime"
)

// Approval card action values carry exactly these two fields (the approval
// ID passes through verbatim; nothing user-supplied is ever concatenated
// into it). The decision is a canonical connect.Action* ID, shared with the
// slash commands and the other channels.
const (
	valueKeyID      = "approval_id"
	valueKeyAction  = "decision"
	valueKeyCommand = "command_token"
)

func commandMenuCard(prompt *connect.MenuPrompt) map[string]any {
	buttons := make([]any, 0, len(prompt.Choices))
	for _, choice := range prompt.Choices {
		buttons = append(buttons, map[string]any{
			"tag": "button", "text": map[string]any{"tag": "plain_text", "content": menuChoiceButtonText(choice)},
			"type": "default", "value": map[string]interface{}{valueKeyCommand: choice.Token},
		})
	}
	body := menuCardBody(prompt)
	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{"template": "blue", "title": map[string]any{"tag": "plain_text", "content": prompt.Title}},
		"elements": []any{
			map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": body}},
			map[string]any{"tag": "action", "actions": buttons},
		},
	}
}

// menuCardBody renders the card body accompanying the choice buttons. Unlike
// connect.FormatMenu — the fallback for button-less transports — it does not
// repeat the choices as a numbered list: the buttons already list them, and
// descriptions ride in the button text.
func menuCardBody(prompt *connect.MenuPrompt) string {
	if len(prompt.Choices) > 0 {
		return fmt.Sprintf("共 %d 项，点击下方按钮选择。", len(prompt.Choices))
	}
	empty := strings.TrimSpace(prompt.Empty)
	if empty == "" {
		empty = "没有可选项"
	}
	return empty
}

// menuChoiceButtonText packs label + description into one button line so the
// buttons carry all the information the old text list did.
func menuChoiceButtonText(choice connect.MenuChoice) string {
	text := choice.Label
	if choice.Description != "" {
		text += " — " + choice.Description
	}
	return connect.TruncateRunes(text, 40)
}

func commandMenuResultCard(content string) map[string]any {
	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"elements": []any{
			map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": content}},
		},
	}
}

// maxMarkdownRunes bounds markdown card content. The documented card size
// limit is 30 KB for the whole payload; at up to 4 bytes per rune in JSON,
// 6000 runes stays safely below it with room for the card envelope.
const maxMarkdownRunes = 6000

// markdownCard builds a card JSON 2.0 message around a single markdown
// component, which renders close to GFM (headings, lists, tables, code
// blocks) — run results are LLM-produced markdown and would lose all
// formatting as a plain text message.
func markdownCard(content string) map[string]any {
	return map[string]any{
		"schema": "2.0",
		"config": map[string]any{"wide_screen_mode": true},
		"body": map[string]any{
			"elements": []any{
				map[string]any{"tag": "markdown", "content": content},
			},
		},
	}
}

// approvalCard builds the interactive approval card (msg_type=interactive):
// a header with the tool name, a summary body, and the decision buttons
// whose action values carry the approval ID and decision. The fourth
// "allow and escalate" button only appears when the approval view allows
// permission escalation.
func approvalCard(p controlruntime.ApprovalView) map[string]any {
	buttons := []any{
		cardButton("批准一次", "default", p.ID, connect.ActionOnce),
		cardButton("永久允许", "primary", p.ID, connect.ActionAlways),
		cardButton("拒绝", "danger", p.ID, connect.ActionReject),
	}
	if p.CanEscalatePermission {
		buttons = append(buttons, cardButton("允许并授权", "primary", p.ID, connect.ActionEscalate))
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
	template := "green"
	if decision == connect.ActionReject {
		template = "red"
	}
	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"template": template,
			"title":    map[string]any{"tag": "plain_text", "content": connect.VerdictForAction(decision) + ": " + p.ToolName},
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
// text approval rendering (connect.ApprovalText).
func approvalSummary(p controlruntime.ApprovalView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**工具**：%s", p.ToolName)
	if p.Kind != "" {
		fmt.Fprintf(&b, "\n**类型**：%s", p.Kind)
	}
	if cmd, ok := p.Args["command"].(string); ok && cmd != "" {
		fmt.Fprintf(&b, "\n**命令**：\n```\n%s\n```", connect.TruncateRunes(cmd, 600))
		return b.String()
	}
	if path, ok := p.Args["path"].(string); ok && path != "" {
		fmt.Fprintf(&b, "\n**路径**：`%s`", path)
	}
	if preview, ok := p.Args["_preview"].(string); ok && preview != "" {
		fmt.Fprintf(&b, "\n**预览**：\n```\n%s\n```", connect.TruncateRunes(preview, 900))
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
	if _, err := connect.ApprovalDecisionFor(dec); err != nil {
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
