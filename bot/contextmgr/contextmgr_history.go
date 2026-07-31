package contextmgr

import (
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider/types"
	"nekocode/logger"
)

type ToolResultMsg struct {
	Message  types.Message
	ToolName string
}

func (m *Manager) Add(role, content string, source ...string) {
	s := ""
	if len(source) > 0 {
		s = source[0]
	}
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.ctx.Messages = append(m.state.ctx.Messages, types.Message{Role: role, Content: content, Source: s})
	m.state.tracker.AddNew(len(role) + len(content))
}

func (m *Manager) AddAssistantResponse(content, reasoning string) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.ctx.Messages = append(m.state.ctx.Messages, types.Message{
		Role:             "assistant",
		Content:          content,
		ReasoningContent: reasoning,
	})
	m.state.tracker.AddNew(len("assistant") + len(content) + len(reasoning))
}

func (m *Manager) AddAssistantToolCall(content, reasoning string, toolCalls []types.ToolCall) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.ctx.Messages = append(m.state.ctx.Messages, types.Message{
		Role:             "assistant",
		Content:          content,
		ReasoningContent: reasoning,
		ToolCalls:        toolCalls,
	})
	tcBytes := 0
	for _, tc := range toolCalls {
		tcBytes += len(tc.ID) + len(tc.Function.Name) + len(tc.Function.Arguments)
	}
	m.state.tracker.AddNew(len("assistant") + len(content) + len(reasoning) + tcBytes)
}

func (m *Manager) AddToolResultsBatch(results []ToolResultMsg) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	for _, r := range results {
		role := "tool"
		if r.Message.ToolCallID == "" {
			role = "user"
		}
		content, _ := budgetToolResult(r.Message.Content, r.ToolName)
		m.state.ctx.Messages = append(m.state.ctx.Messages, types.Message{
			Role:       role,
			Content:    content,
			ToolCallID: r.Message.ToolCallID,
			IsError:    r.Message.IsError,
		})
		m.state.tracker.AddNew(len(role) + len(content) + len(r.Message.ToolCallID))
	}
}

func (m *Manager) Clear() {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.clearLocked()
}

func (m *Manager) TruncateTo(n int) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	if n < 0 {
		n = 0
	}
	if n < len(m.state.ctx.Messages) {
		logger.Log("truncate_to: dropped %d messages (kept %d, was %d)", len(m.state.ctx.Messages)-n, n, len(m.state.ctx.Messages))
		m.state.ctx.Messages = m.state.ctx.Messages[:n]
	}
	if m.state.ctx.CompactBoundary > n {
		m.state.ctx.CompactBoundary = n
	}
}

func (m *Manager) RemoveMessages(startIdx, endIdx int) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	if startIdx < 0 || endIdx >= len(m.state.ctx.Messages) || startIdx > endIdx {
		return
	}
	n := endIdx - startIdx + 1
	m.state.ctx.Messages = append(m.state.ctx.Messages[:startIdx], m.state.ctx.Messages[endIdx+1:]...)
	logger.Log("remove_messages: dropped %d messages [%d:%d] (total now %d)", n, startIdx, endIdx, len(m.state.ctx.Messages))
	if m.state.ctx.CompactBoundary > startIdx {
		if m.state.ctx.CompactBoundary <= endIdx {
			m.state.ctx.CompactBoundary = startIdx
		} else {
			m.state.ctx.CompactBoundary -= n
		}
	}
}

func (m *Manager) FreshStart() {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	n := len(m.state.ctx.Messages)
	m.clearLocked()
	logger.Log("fresh_start: clearing all %d messages", n)
	m.state.ctx.Hints = ""
	m.state.tracker.Restore(token.State{})
}

func (m *Manager) clearLocked() {
	m.state.ctx.Messages = make([]types.Message, 0)
	m.state.ctx.CompactBoundary = 0
	m.state.ctx.Todo = ""
	m.state.ctx.TodoItems = nil
}
