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
	m.state.revision++
}

// AddAssistant persists the complete assistant message while charging only the
// reasoning that the active model contract will replay on its next request.
func (m *Manager) AddAssistant(message types.Message) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	message.Role = "assistant"
	m.state.ctx.Messages = append(m.state.ctx.Messages, message)
	m.state.tracker.AddEstimated(token.EstimateModelTokens([]types.Message{message}, m.state.reasoning))
	m.state.revision++
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
	if len(results) > 0 {
		m.state.revision++
	}
}

// Reset clears both active history and its compaction archive.
func (m *Manager) Reset() {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.clearLocked()
	m.state.ctx.Archive = ""
	m.state.ctx.Hints = ""
	m.state.runtimePolicy = ""
	m.state.tracker.Restore(token.State{})
	m.state.prefix.Reset()
	m.resetRuntimeContextLocked()
	m.state.compactCount = 0
	m.state.trimCount = 0
	m.state.revision++
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
		m.state.revision++
		m.state.tracker.RecordPrompt(m.totalTokenEstimate())
		m.restoreRuntimeContextLocked()
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
	m.state.revision++
	m.state.tracker.RecordPrompt(m.totalTokenEstimate())
	m.restoreRuntimeContextLocked()
	logger.Log("remove_messages: dropped %d messages [%d:%d] (total now %d)", n, startIdx, endIdx, len(m.state.ctx.Messages))
}

func (m *Manager) clearLocked() {
	m.state.ctx.Messages = make([]types.Message, 0)
	m.state.ctx.TodoItems = nil
}
