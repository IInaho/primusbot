package feishu

import (
	"fmt"
	"strings"

	controlruntime "nekocode/runtime"
)

// renderEvent maps a runtime event to an outbound plain-text message.
// Streaming events (assistant/reasoning deltas) are handled separately by
// the core stream buffer; "" means the event produces no message.
func renderEvent(ev controlruntime.Event) string {
	switch ev.Type {
	case controlruntime.EventToolCompleted:
		p, ok := ev.Payload.(controlruntime.ToolPayload)
		if !ok {
			return ""
		}
		line := "- " + p.ToolName
		if args := truncateRunes(strings.TrimSpace(p.Args), 80); args != "" {
			line += " " + args
		}
		if p.IsError {
			line += " (failed)"
		}
		return line
	case controlruntime.EventToolBlocked:
		p, ok := ev.Payload.(controlruntime.ToolPayload)
		if !ok {
			return ""
		}
		return fmt.Sprintf("- %s (blocked: %s)", p.ToolName, truncateRunes(p.Preview, 120))
	case controlruntime.EventApprovalRequested:
		p, ok := ev.Payload.(controlruntime.ApprovalView)
		if !ok || p.ID == "" {
			return ""
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Approval needed: %s", p.ToolName)
		if cmd, ok := p.Args["command"].(string); ok && cmd != "" {
			fmt.Fprintf(&b, "\n%s", truncateRunes(cmd, 600))
		} else if path, ok := p.Args["path"].(string); ok && path != "" {
			fmt.Fprintf(&b, "\n%s", path)
		}
		fmt.Fprintf(&b, "\nReply /approve %s or /reject %s", p.ID, p.ID)
		return b.String()
	case controlruntime.EventQuestionRequested:
		p, ok := ev.Payload.(controlruntime.QuestionView)
		if !ok || len(p.Questions) == 0 {
			return ""
		}
		var b strings.Builder
		b.WriteString("Question from NekoCode:")
		for _, q := range p.Questions {
			fmt.Fprintf(&b, "\n- %s", q.Question)
			for _, opt := range q.Options {
				fmt.Fprintf(&b, "\n    · %s", opt.Label)
			}
		}
		b.WriteString("\n(Answer in the TUI — in-feishu answers land in phase 2.)")
		return b.String()
	case controlruntime.EventRunFailed:
		p, ok := ev.Payload.(controlruntime.DonePayload)
		if !ok {
			return ""
		}
		return "Run failed: " + truncateRunes(p.Error, 600)
	default:
		return ""
	}
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
