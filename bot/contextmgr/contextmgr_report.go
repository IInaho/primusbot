package contextmgr

import (
	"nekocode/bot/contextmgr/token"
)

type ContextReport struct {
	Budget          int
	SystemPrompt    int
	TodoText        int
	SkillList       int
	ToolDefTokens   int
	Messages        int
	Archived        int
	ClearedMarkers  int
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
}

func (m *Manager) Report() ContextReport {
	runtimePrompt := m.buildRuntimePrompt()
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	r := ContextReport{}
	r.SystemPrompt = token.EstimateString(m.state.ctx.SystemPrompt) + token.EstimateString(runtimePrompt)
	r.TodoText = token.EstimateString(m.state.ctx.Todo)
	r.SkillList = token.EstimateString(m.state.ctx.Skills)

	for i := m.state.ctx.CompactBoundary; i < len(m.state.ctx.Messages); i++ {
		msg := m.state.ctx.Messages[i]
		if msg.Content == clearedMarker {
			r.ClearedMarkers++
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
	r.Messages = token.EstimateTokens(m.state.ctx.Messages[m.state.ctx.CompactBoundary:])
	r.Archived = m.state.ctx.CompactBoundary
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
	return r
}
