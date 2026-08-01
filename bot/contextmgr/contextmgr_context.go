package contextmgr

import (
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider/types"
	"nekocode/protocol"
)

func (m *Manager) Build() []types.Message {
	runtimePrompt := m.buildRuntimePrompt()
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	out := m.state.ctx.BuildLayer0()
	out = append(out, m.state.ctx.BuildLayer0Mem()...)
	out = append(out, m.state.ctx.BuildLayer05()...)
	if runtimePrompt != "" {
		out = append(out, types.Message{Role: "system", Content: runtimePrompt})
	}
	out = append(out, m.filterValidMessages(m.visibleHistory())...)
	out = append(out, m.state.ctx.BuildLayer2()...)

	for i := range out {
		out[i].Source = ""
	}
	return out
}

// SetRuntimePromptProvider sets an ephemeral system-message provider. The
// provider is called for every model-context build and its output is excluded
// from snapshots so resumed sessions always see the current environment.
func (m *Manager) SetRuntimePromptProvider(provider func() string) {
	m.runtimeMu.Lock()
	m.runtimePrompt = provider
	m.runtimeMu.Unlock()
}

func (m *Manager) buildRuntimePrompt() string {
	m.runtimeMu.RLock()
	provider := m.runtimePrompt
	m.runtimeMu.RUnlock()
	if provider == nil {
		return ""
	}
	return provider()
}

func (m *Manager) visibleHistory() []types.Message {
	if m.state.ctx.CompactBoundary <= 0 {
		return m.state.ctx.Messages
	}
	if m.state.ctx.CompactBoundary >= len(m.state.ctx.Messages) {
		return nil
	}
	return m.state.ctx.Messages[m.state.ctx.CompactBoundary:]
}

func (m *Manager) filterValidMessages(kept []types.Message) []types.Message {
	hasResult := map[string]bool{}
	for _, msg := range kept {
		if msg.Role == "tool" && msg.ToolCallID != "" {
			hasResult[msg.ToolCallID] = true
		}
	}
	validAsst := map[int]bool{}
	for i, msg := range kept {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			ok := true
			for _, tc := range msg.ToolCalls {
				if tc.ID != "" && !hasResult[tc.ID] {
					ok = false
					break
				}
			}
			if ok {
				validAsst[i] = true
			}
		}
	}
	validIDs := map[string]bool{}
	filtered := make([]types.Message, 0, len(kept))
	for i, msg := range kept {
		if msg.Content == "" && msg.Role != "system" {
			msg.Content = "."
		}
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			if !validAsst[i] {
				continue
			}
			for _, tc := range msg.ToolCalls {
				if tc.ID != "" {
					validIDs[tc.ID] = true
				}
			}
		}
		if msg.Role == "tool" && (msg.ToolCallID == "" || !validIDs[msg.ToolCallID]) {
			continue
		}
		filtered = append(filtered, msg)
	}
	return filtered
}

func (m *Manager) SetSystemPrompt(systemPrompt string) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.ctx.SystemPrompt = systemPrompt
}

func (m *Manager) SetSkillList(skillList string) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.ctx.Skills = skillList
}

func (m *Manager) SetHints(hints string) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.ctx.Hints = hints
}

func (m *Manager) SetContextWindow(budget int) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	if budget > 0 {
		m.state.contextWindow = budget
	}
}

func (m *Manager) SetTodos(items []protocol.TodoItem) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.ctx.LoadTodos(items)
}

func (m *Manager) AllTasksDone() bool {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	return m.state.ctx.AllTasksDone()
}

func (m *Manager) HasTasks() bool {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	return m.state.ctx.HasTasks()
}

func (m *Manager) Len() int {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	n := len(m.state.ctx.Messages)
	if m.state.ctx.CompactBoundary > 0 && m.state.ctx.CompactBoundary < n {
		return n - m.state.ctx.CompactBoundary
	}
	return n
}

func (m *Manager) Stats() (int, int, bool) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	return len(m.state.ctx.Messages), m.totalTokenEstimate(), m.state.ctx.Archive != ""
}

func (m *Manager) RecordUsage(prompt, completion int) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.tracker.RecordUsage(prompt, completion)
}

func (m *Manager) RecordCache(hit, miss int) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.tracker.RecordCache(hit, miss)
}

func (m *Manager) ResetCache() {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.tracker.ResetCache()
}

func (m *Manager) TokenUsage() (int, int) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	return m.totalTokenEstimate(), m.state.contextWindow
}

func (m *Manager) RecordSubagent(tokens, hit, miss int) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.tracker.RecordSubagent(tokens, hit, miss)
}

func (m *Manager) CacheStats() (hit, miss int) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	return m.state.tracker.CacheStats()
}

func (m *Manager) visibleMessages() []types.Message {
	visible := m.state.ctx.Messages
	if m.state.ctx.CompactBoundary > 0 && m.state.ctx.Archive != "" && m.state.ctx.CompactBoundary < len(visible) {
		visible = visible[m.state.ctx.CompactBoundary:]
	}
	return visible
}

func (m *Manager) totalTokenEstimate() int {
	return token.EstimateTokens(m.visibleMessages()) + token.EstimateString(m.state.ctx.Archive)
}
