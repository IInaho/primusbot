package view

import ctxmgr "nekocode/bot/contextmgr"

func NewContextSnapshot(r ctxmgr.ContextReport) ContextSnapshot {
	used := r.SystemPrompt + r.ToolDefTokens + r.TodoText + r.SkillList + r.Messages
	free := max(r.Budget-used, 0)
	percentUsed := 0.0
	if r.Budget > 0 {
		percentUsed = min(float64(used)/float64(r.Budget), 1)
	}

	return ContextSnapshot{
		Budget:          r.Budget,
		Used:            used,
		Free:            free,
		PercentUsed:     percentUsed,
		SystemPrompt:    r.SystemPrompt,
		ToolDefTokens:   r.ToolDefTokens,
		TodoText:        r.TodoText,
		SkillList:       r.SkillList,
		MessageTokens:   r.Messages,
		ToolDefCount:    r.ToolDefCount,
		MessageCount:    r.UserMessages + r.AssistantMsgs + r.ToolResults,
		UserMessages:    r.UserMessages,
		AssistantMsgs:   r.AssistantMsgs,
		ToolResults:     r.ToolResults,
		Archived:        r.Archived,
		CompactCount:    r.CompactCount,
		TrimCount:       r.TrimCount,
		CacheHitTokens:  r.CacheHitTokens,
		CacheMissTokens: r.CacheMissTokens,
		CacheHitRatio:   r.CacheHitRatio,
		SubCount:        r.SubCount,
		SubTokens:       r.SubTokens,
		SubCacheHit:     r.SubCacheHit,
		SubCacheMiss:    r.SubCacheMiss,
		Segments: []ContextSegment{
			{Key: "system", Label: "系统提示", Tokens: r.SystemPrompt, Tone: "muted"},
			{Key: "tools", Label: "工具定义", Tokens: r.ToolDefTokens, Tone: "blue"},
			{Key: "todo", Label: "待办", Tokens: r.TodoText, Tone: "orange"},
			{Key: "skills", Label: "Skills", Tokens: r.SkillList, Tone: "yellow"},
			{Key: "messages", Label: "对话消息", Tokens: r.Messages, Tone: "violet"},
			{Key: "free", Label: "剩余", Tokens: free, Tone: "free"},
		},
	}
}
