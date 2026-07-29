package core

import (
	"context"
	"strings"

	controlruntime "nekocode/runtime"
)

// CommandHandler parses the text commands shared by IM connectors
// (/stop, /help, /approve, /always, /reject) and drives the runtime
// accordingly.
// Channels compose it and fall through to their own commands (or message
// submission) when Handle reports handled=false.
type CommandHandler struct {
	RT controlruntime.ConnectorRuntime

	// Help is the channel-specific help text returned for /help.
	Help string
}

// Handle executes a shared command. handled is false when text is not one
// of the shared commands — the channel should then try its own commands or
// submit the text as a message. The returned reply is meant to be sent back
// to the chat ("" means send nothing, e.g. after a successful /stop reply
// is already included).
func (h CommandHandler) Handle(ctx context.Context, text string) (reply string, handled bool) {
	switch {
	case text == "/stop":
		_ = h.RT.Abort(ctx, "")
		return "已请求停止。", true
	case text == "/help":
		return h.Help, true
	case strings.HasPrefix(text, "/approve "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/approve "))
		err := h.RT.Approve(ctx, id, controlruntime.ApprovalDecision{Allowed: true})
		return errReply("已批准。", err), true
	case strings.HasPrefix(text, "/always "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/always "))
		err := h.RT.Approve(ctx, id, controlruntime.ApprovalDecision{Allowed: true, Remember: true})
		return errReply("已永久允许。", err), true
	case strings.HasPrefix(text, "/reject "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/reject "))
		err := h.RT.Approve(ctx, id, controlruntime.ApprovalDecision{})
		return errReply("已拒绝。", err), true
	default:
		return "", false
	}
}

func errReply(ok string, err error) string {
	if err != nil {
		return "错误: " + err.Error()
	}
	return ok
}
