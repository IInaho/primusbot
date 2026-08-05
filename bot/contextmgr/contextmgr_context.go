package contextmgr

import (
	"context"

	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider"
	"nekocode/bot/provider/types"
	"nekocode/logger"
	"nekocode/protocol"
)

// Build assembles the full request context in cache-stability order:
//
//	Layer 0  system prompt + skill list (immutable within a session)
//	Layer 1  long-term memory
//	Layer 2  compaction archive
//	Layer 3  message history (append-only)
//	Layer 4  runtime environment (volatile: date, processes, roots)
//	Layer 5  todos + hints (volatile tail)
//
// Layers 4-5 ride the tail: their content may change every turn, and a
// change there only costs the tail itself, never the cached prefix.
func (m *Manager) Build() []types.Message {
	runtimePrompt := m.buildLayer4()
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	return m.buildLocked(runtimePrompt, false)
}

// BuildRequest assembles one model request and records its cache-relevant
// shape in the same state snapshot. Model callers should use this instead of
// pairing Build and prefix observation manually.
func (m *Manager) BuildRequest(toolDefs []types.ToolDef) []types.Message {
	runtimePrompt := m.buildLayer4()
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	out := m.buildLocked(runtimePrompt, true)
	system := m.state.ctx.BuildLayer0()
	system = append(system, m.state.ctx.BuildLayer1()...)
	history := m.state.ctx.BuildLayer2()
	history = append(history, m.buildLayer3()...)
	m.state.prefix.Observe(buildPrefixShape(system, history, toolDefs))
	return out
}

func (m *Manager) buildLocked(runtimePrompt string, preserveSource bool) []types.Message {
	out := m.state.ctx.BuildLayer0()
	out = append(out, m.state.ctx.BuildLayer1()...)
	out = append(out, m.state.ctx.BuildLayer2()...)
	out = append(out, m.buildLayer3()...)
	if runtimePrompt != "" {
		out = append(out, types.Message{Role: "system", Content: runtimePrompt, Source: types.MessageSourceVolatileTail})
	}
	out = append(out, m.state.ctx.BuildLayer5()...)
	if !preserveSource {
		for i := range out {
			out[i].Source = ""
		}
	}
	return out
}

// buildLayer3 returns the visible (post-compaction-boundary) history with
// orphaned tool calls/results filtered out.
func (m *Manager) buildLayer3() []types.Message {
	return m.filterValidMessages(m.state.ctx.Messages)
}

// buildLayer4 renders the runtime environment block via the registered
// provider ("" when none is set).
func (m *Manager) buildLayer4() string {
	provider := m.runtimePrompt
	if provider == nil {
		return ""
	}
	return provider()
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

// ModelContext contains the context settings that change together when the
// active model changes.
type ModelContext struct {
	Window          int
	CompactionModel provider.LLM
}

// ConfigureModel updates model-dependent context settings atomically.
func (m *Manager) ConfigureModel(cfg ModelContext) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	if cfg.Window > 0 {
		m.state.contextWindow = cfg.Window
	}
	m.state.tracker.ResetCache()
	m.state.prefix.Reset()
	if m.state.compressor != nil {
		var summarizer Summarizer
		if cfg.CompactionModel != nil {
			summarizer = makeSummarizer(context.Background(), cfg.CompactionModel)
		}
		m.state.compressor.summarizer = summarizer
	}
}

func (m *Manager) SetTodos(items []protocol.TodoItem) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.ctx.LoadTodos(items)
}

func (m *Manager) Len() int {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	return len(m.state.ctx.Messages)
}

type Status struct {
	Messages     int
	Tokens       int
	Budget       int
	HasArchive   bool
	CacheHit     int
	CacheMiss    int
	HasTasks     bool
	TasksDone    bool
	CompactCount int
}

// Status returns one coherent snapshot of context occupancy and session usage.
func (m *Manager) Status() Status {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	hit, miss := m.state.tracker.CacheStats()
	return Status{
		Messages: len(m.state.ctx.Messages), Tokens: m.estimatedTokens(),
		Budget: m.state.contextWindow, HasArchive: m.state.ctx.Archive != "",
		CacheHit: hit, CacheMiss: miss,
		HasTasks: m.state.ctx.HasTasks(), TasksDone: m.state.ctx.AllTasksDone(),
		CompactCount: m.state.compactCount,
	}
}

func (m *Manager) RecordUsage(prompt int) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.tracker.RecordPrompt(prompt)
}

// BeginModelTurn starts cache diagnostics for one user conversation. It keeps
// the prior request shape so the next request can still detect prefix changes.
func (m *Manager) BeginModelTurn() {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.prefix.BeginTurn()
}

func (m *Manager) RecordCache(hit, miss int) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.tracker.RecordCache(hit, miss)
	diagnosis := m.state.prefix.RecordCache(hit, miss)
	if miss > 0 {
		logger.Log("prefix cache miss: tokens=%d changed=%v", miss, diagnosis.Parts)
	}
}

func (m *Manager) RecordSubagent(tokens, hit, miss int) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.tracker.RecordSubagent(tokens, hit, miss)
}

func (m *Manager) totalTokenEstimate() int {
	return token.EstimateString(m.state.ctx.SystemPrompt) +
		token.EstimateString(m.state.ctx.Skills) +
		token.EstimateString(m.state.ctx.Memory) +
		token.EstimateString(m.state.ctx.Archive) +
		token.EstimateTokens(m.state.ctx.Messages) +
		token.EstimateString(m.state.ctx.Todo) +
		token.EstimateString(m.state.ctx.Hints)
}

func (m *Manager) estimatedTokens() int {
	estimated := m.totalTokenEstimate()
	if calibrated := m.state.tracker.PromptEstimate(); calibrated > estimated {
		return calibrated
	}
	return estimated
}
