package contextmgr

import (
	"nekocode/bot/contextmgr/token"
)

type ContextReport struct {
	Budget          int
	SystemPrompt    int
	TodoText        int
	SkillList       int
	Memory          int
	Archive         int
	ToolDefTokens   int
	Messages        int
	HasArchive      bool
	Archived        int
	CompactCount    int
	TrimCount       int
	ToolDefCount    int
	UserMessages    int
	SysInjections   int
	AssistantMsgs   int
	ToolResults     int
	CacheHitTokens  int
	CacheMissTokens int
	CacheHitRatio   float64
	SubCount        int
	SubTokens       int
	SubCacheHit     int
	SubCacheMiss    int
	PrefixTurn      PrefixTurnStats
}

func (m *Manager) Report() ContextReport {
	runtimePrompt := m.buildLayer4()
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	r := ContextReport{}
	r.SystemPrompt = token.EstimateString(m.state.ctx.SystemPrompt) + token.EstimateString(runtimePrompt)
	r.TodoText = token.EstimateString(m.state.ctx.Todo)
	r.SkillList = token.EstimateString(m.state.ctx.Skills)
	r.Memory = token.EstimateString(m.state.ctx.Memory)
	r.Archive = token.EstimateString(m.state.ctx.Archive)

	for _, msg := range m.state.ctx.Messages {
		if msg.Content == clearedMarker {
			continue
		}
		switch msg.Role {
		case "user":
			if msg.Source == "system" {
				r.SysInjections++
			} else {
				r.UserMessages++
			}
		case "assistant":
			r.AssistantMsgs++
		case "tool":
			r.ToolResults++
		}
	}
	r.Messages = token.EstimateTokens(m.state.ctx.Messages)
	r.HasArchive = m.state.ctx.Archive != ""
	r.Archived = m.state.trimCount
	r.CompactCount = m.state.compactCount
	r.TrimCount = m.state.trimCount
	r.Budget = m.state.contextWindow
	r.CacheHitTokens, r.CacheMissTokens = m.state.tracker.CacheStats()
	r.CacheHitRatio = m.state.tracker.CacheHitRatio()
	sub := m.state.tracker.SubStats()
	r.SubCount = sub.Count
	r.SubTokens = sub.TotalTokens
	r.SubCacheHit = sub.CacheHitTokens
	r.SubCacheMiss = sub.CacheMissTokens
	r.PrefixTurn = m.state.prefix.TurnStats()
	return r
}
