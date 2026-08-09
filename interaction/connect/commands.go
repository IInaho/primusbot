package connect

import (
	"context"
	"strings"

	controlruntime "nekocode/runtime"
)

// CommandHandler parses the text commands shared by IM connectors
// (/stop, /approve, /always, /reject, and — when Questions is set —
// /answer and /dismiss) and drives the runtime accordingly.
// Channels compose it and fall through to their own commands (or message
// submission) when Handle reports handled=false.
type CommandHandler struct {
	RT controlruntime.ConnectorRuntime

	// Questions enables /answer and /dismiss when set. Channels feed it
	// from the IntentQuestion / IntentQuestionResolved intents their sink
	// receives.
	Questions *QuestionTracker
}

// Handle executes a shared command. handled is false when text is not one
// of the shared commands — the channel should then try its own commands or
// submit the text as a message. The returned reply is meant to be sent back
// to the chat ("" means send nothing, e.g. after a successful /stop reply
// is already included).
func (h CommandHandler) Handle(ctx context.Context, text string) (reply string, handled bool) {
	switch {
	case text == "/stop":
		_ = h.RT.CancelRun(ctx, "")
		return "已请求停止。", true
	case strings.HasPrefix(text, "/approve "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/approve "))
		err := h.RT.DecideApproval(ctx, id, controlruntime.ApprovalDecision{Allowed: true})
		return errReply("已批准。", err), true
	case strings.HasPrefix(text, "/always "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/always "))
		err := h.RT.DecideApproval(ctx, id, controlruntime.ApprovalDecision{Allowed: true, Remember: true})
		return errReply("已记住并允许。", err), true
	case strings.HasPrefix(text, "/reject "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/reject "))
		err := h.RT.DecideApproval(ctx, id, controlruntime.ApprovalDecision{})
		return errReply("已拒绝。", err), true
	case h.Questions != nil && (text == "/answer" || strings.HasPrefix(text, "/answer ")):
		id, answer := ParseAnswerCommand(text)
		if answer == "" {
			return "用法: /answer <回答内容>", true
		}
		if id == "" && h.Questions.LastID() == "" {
			return "当前没有待回答的问题。", true
		}
		reply, resolvedID, err := h.Questions.BuildReply(id, answer)
		if err == nil {
			err = h.RT.AnswerQuestion(ctx, resolvedID, reply)
		}
		if err != nil {
			return errReply("", err), true
		}
		h.Questions.Remove(resolvedID)
		return "答案已发送。", true
	case h.Questions != nil && (text == "/dismiss" || strings.HasPrefix(text, "/dismiss ")):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/dismiss"))
		resolvedID, err := h.Questions.Reject(id)
		if err == nil {
			err = h.RT.AnswerQuestion(ctx, resolvedID, controlruntime.QuestionReply{Rejected: true})
		}
		if err != nil {
			return errReply("", err), true
		}
		h.Questions.Remove(resolvedID)
		return "已忽略。", true
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
