package config

import "strings"

// knownContextWindows maps lowercase model-name substrings to their context
// window sizes. There is no standard provider API to query a model's context
// window, so this table covers the common models; it is only a fallback —
// an explicit context_window (per-model or global) always wins.
// First match wins, so list more specific names before their prefixes.
// Values verified against provider announcements as of 2026-08.
var knownContextWindows = []struct {
	substr string
	window int
}{
	// DeepSeek: V4 generation (Pro/Flash) is 1M; the retired deepseek-chat /
	// deepseek-reasoner endpoints were aliases of V4-Flash.
	{"deepseek-v4", 1048576},
	{"deepseek-chat", 1048576},
	{"deepseek-reasoner", 1048576},
	// Anthropic: Opus 4.6+ / Sonnet 4.6+ / Sonnet 5 / Fable are 1M;
	// Haiku and older generations are 200K.
	{"claude-haiku", 200000},
	{"claude-opus-4-6", 1048576},
	{"claude-opus-4-7", 1048576},
	{"claude-opus-4-8", 1048576},
	{"claude-sonnet-4-6", 1048576},
	{"claude-sonnet-5", 1048576},
	{"claude-fable", 1048576},
	{"claude", 200000},
	// OpenAI: 5.5/5.6 (Sol/Terra/Luna) are 1.05M; 5/5.1/5.2 are 400K.
	{"gpt-4.1", 1047576},
	{"gpt-4o", 128000},
	{"gpt-4-turbo", 128000},
	{"gpt-5.6", 1050000},
	{"gpt-5.5", 1050000},
	{"gpt-5", 400000},
	{"o1-", 200000},
	{"o3-", 200000},
	{"o4-mini", 200000},
	// Zhipu GLM: 5.x and 4.6 are 200K; older 4.x is 128K.
	{"glm-5", 200000},
	{"glm-4.6", 200000},
	{"glm-4", 131072},
	// Moonshot / Kimi: K3 is 1M; K2.x is 256K; legacy moonshot-v1 varies.
	{"kimi-k3", 1048576},
	{"moonshot-v1-8k", 8192},
	{"moonshot-v1-32k", 32768},
	{"moonshot-v1-128k", 131072},
	{"kimi", 262144},
	// Alibaba Qwen
	{"qwen3", 131072},
	{"qwen2.5", 131072},
	{"qwq", 131072},
	// Google
	{"gemini", 1048576},
	// xAI: Grok 4.1 Fast up to 2M; 4.5 is 500K; 4/4.1 is 256K; 3 is 128K.
	{"grok-4.1-fast", 2097152},
	{"grok-4.5", 512000},
	{"grok-4", 262144},
	{"grok-3", 131072},
	// Meta: Llama 4 Scout 10M, Maverick 1M; 3.x is 128K.
	{"llama-4-scout", 10485760},
	{"llama-4", 1048576},
	{"llama-3.1", 131072},
	{"llama-3.3", 131072},
	// MiniMax
	{"minimax-text-01", 1048576},
	// Mistral
	{"mistral-large", 131072},
}

// KnownContextWindow reports the built-in context window for a model ID
// (matched case-insensitively by substring, so "deepseek/deepseek-v4-pro"
// style prefixed IDs also hit). ok is false for unknown models.
func KnownContextWindow(model string) (window int, ok bool) {
	lower := strings.ToLower(model)
	for _, e := range knownContextWindows {
		if strings.Contains(lower, e.substr) {
			return e.window, true
		}
	}
	return 0, false
}
