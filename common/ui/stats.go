package ui

// BotStats carries runtime statistics from the bot to the UI.
type BotStats struct {
	PromptTokens, CompletionTokens int
	TurnPrompt, TurnCompletion     int
	ContextTokens, CompactCount    int
	Duration                       string
}

// ContextSegment describes one visible part of the active context window.
type ContextSegment struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Tokens int    `json:"tokens"`
	Tone   string `json:"tone"`
}

// ContextSnapshot is the structured context status consumed by UI surfaces.
type ContextSnapshot struct {
	Budget          int              `json:"budget"`
	Used            int              `json:"used"`
	Free            int              `json:"free"`
	PercentUsed     float64          `json:"percentUsed"`
	SystemPrompt    int              `json:"systemPrompt"`
	ToolDefTokens   int              `json:"toolDefTokens"`
	TodoText        int              `json:"todoText"`
	SkillList       int              `json:"skillList"`
	MessageTokens   int              `json:"messageTokens"`
	ToolDefCount    int              `json:"toolDefCount"`
	MessageCount    int              `json:"messageCount"`
	UserMessages    int              `json:"userMessages"`
	AssistantMsgs   int              `json:"assistantMsgs"`
	ToolResults     int              `json:"toolResults"`
	Archived        int              `json:"archived"`
	CompactCount    int              `json:"compactCount"`
	TrimCount       int              `json:"trimCount"`
	CacheHitTokens  int              `json:"cacheHitTokens"`
	CacheMissTokens int              `json:"cacheMissTokens"`
	CacheHitRatio   float64          `json:"cacheHitRatio"`
	SubCount        int              `json:"subCount"`
	SubTokens       int              `json:"subTokens"`
	SubCacheHit     int              `json:"subCacheHit"`
	SubCacheMiss    int              `json:"subCacheMiss"`
	Segments        []ContextSegment `json:"segments"`
}
