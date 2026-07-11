package history

import (
	"nekocode/bot/contextmgr/compression"
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider/types"
	"nekocode/common/debug"
)

type Store struct {
	State *state.State
}

type ToolResultMsg struct {
	Message  types.Message
	ToolName string
}

func (h *Store) Add(role, content string, source ...string) {
	s := ""
	if len(source) > 0 {
		s = source[0]
	}
	h.State.Mu.Lock()
	defer h.State.Mu.Unlock()
	h.State.Ctx.Messages = append(h.State.Ctx.Messages, types.Message{Role: role, Content: content, Source: s})
	h.State.Tracker.AddNew(len(role) + len(content))
}

func (h *Store) AddAssistantResponse(content, reasoning string) {
	h.State.Mu.Lock()
	defer h.State.Mu.Unlock()
	h.State.Ctx.Messages = append(h.State.Ctx.Messages, types.Message{
		Role:             "assistant",
		Content:          content,
		ReasoningContent: reasoning,
	})
	h.State.Tracker.AddNew(len("assistant") + len(content) + len(reasoning))
}

func (h *Store) AddAssistantToolCall(content, reasoning string, toolCalls []types.ToolCall) {
	h.State.Mu.Lock()
	defer h.State.Mu.Unlock()
	h.State.Ctx.Messages = append(h.State.Ctx.Messages, types.Message{
		Role:             "assistant",
		Content:          content,
		ReasoningContent: reasoning,
		ToolCalls:        toolCalls,
	})
	tcBytes := 0
	for _, tc := range toolCalls {
		tcBytes += len(tc.ID) + len(tc.Function.Name) + len(tc.Function.Arguments)
	}
	h.State.Tracker.AddNew(len("assistant") + len(content) + len(reasoning) + tcBytes)
}

func (h *Store) AddToolResultsBatch(results []ToolResultMsg) {
	h.State.Mu.Lock()
	defer h.State.Mu.Unlock()
	for _, r := range results {
		role := "tool"
		if r.Message.ToolCallID == "" {
			role = "user"
		}
		content, _ := compression.BudgetResult(r.Message.Content, r.ToolName)
		h.State.Ctx.Messages = append(h.State.Ctx.Messages, types.Message{
			Role:       role,
			Content:    content,
			ToolCallID: r.Message.ToolCallID,
			IsError:    r.Message.IsError,
		})
		h.State.Tracker.AddNew(len(role) + len(content) + len(r.Message.ToolCallID))
	}
}

func (h *Store) Clear() {
	h.State.Mu.Lock()
	defer h.State.Mu.Unlock()
	h.clearLocked()
}

func (h *Store) TruncateTo(n int) {
	h.State.Mu.Lock()
	defer h.State.Mu.Unlock()
	if n < 0 {
		n = 0
	}
	if n < len(h.State.Ctx.Messages) {
		debug.Log("truncate_to: dropped %d messages (kept %d, was %d)", len(h.State.Ctx.Messages)-n, n, len(h.State.Ctx.Messages))
		h.State.Ctx.Messages = h.State.Ctx.Messages[:n]
	}
	if h.State.Ctx.CompactBoundary > n {
		h.State.Ctx.CompactBoundary = n
	}
}

func (h *Store) RemoveMessages(startIdx, endIdx int) {
	h.State.Mu.Lock()
	defer h.State.Mu.Unlock()
	if startIdx < 0 || endIdx >= len(h.State.Ctx.Messages) || startIdx > endIdx {
		return
	}
	n := endIdx - startIdx + 1
	h.State.Ctx.Messages = append(h.State.Ctx.Messages[:startIdx], h.State.Ctx.Messages[endIdx+1:]...)
	debug.Log("remove_messages: dropped %d messages [%d:%d] (total now %d)", n, startIdx, endIdx, len(h.State.Ctx.Messages))
	if h.State.Ctx.CompactBoundary > startIdx {
		if h.State.Ctx.CompactBoundary <= endIdx {
			h.State.Ctx.CompactBoundary = startIdx
		} else {
			h.State.Ctx.CompactBoundary -= n
		}
	}
}

func (h *Store) FreshStart() {
	h.State.Mu.Lock()
	defer h.State.Mu.Unlock()
	n := len(h.State.Ctx.Messages)
	h.clearLocked()
	debug.Log("fresh_start: clearing all %d messages", n)
	h.State.Ctx.Hints = ""
	h.State.Tracker.Restore(token.State{})
}

func (h *Store) clearLocked() {
	h.State.Ctx.Messages = make([]types.Message, 0)
	h.State.Ctx.CompactBoundary = 0
	h.State.Ctx.Todo = ""
	h.State.Ctx.TodoItems = nil
}
