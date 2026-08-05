package types

import (
	nethttp "net/http"
	"sync"
	"time"

	utilhttp "nekocode/util/http"
)

var SharedHTTPClientTimeout = &nethttp.Client{
	Transport: utilhttp.SharedTransport,
	Timeout:   120 * time.Second,
}

// SharedHTTPStreamClient is used for streaming requests. The timeout is intentionally
// long (10 min) — streaming responses can take minutes of token generation.
var SharedHTTPStreamClient = &nethttp.Client{
	Transport: utilhttp.SharedTransport,
	Timeout:   10 * time.Minute,
}

type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	Name             string     `json:"name,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	IsError          bool       `json:"is_error,omitempty"`
	Source           string     `json:"source,omitempty"` // internal routing metadata; provider wire structs omit it
}

const MessageSourceVolatileTail = "volatile-tail"

type ToolCall struct {
	Index    int          `json:"index"`
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Response struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type StreamToken struct {
	Content          string
	ReasoningContent string
	ToolCallDelta    *ToolCallDelta
	Usage            *StreamUsage
	FinishReason     string
}

type ToolCallDelta struct {
	Index     int
	ID        string
	Name      string
	Arguments string
}

type StreamUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	CacheHitTokens   int
	CacheMissTokens  int
	// OpenAI standard: prompt_tokens_details.cached_tokens.
	PromptTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details,omitempty"`
	// DeepSeek also reports flat fields (kept for backward compatibility):
	// prompt_cache_hit_tokens / prompt_cache_miss_tokens.
	FlatCacheHit  *int `json:"prompt_cache_hit_tokens,omitempty"`
	FlatCacheMiss *int `json:"prompt_cache_miss_tokens,omitempty"`
}

// Normalize extracts cache fields from protocol-specific usage formats.
// Provider-reported values win over derived ones: the OpenAI-standard
// nested field wins for hit, DeepSeek's flat fields fill any gap, and a
// reported miss is trusted over the prompt-hit arithmetic.
func (u *StreamUsage) Normalize() {
	reportedHit := u.CacheHitTokens > 0
	if u.PromptTokensDetails != nil {
		u.CacheHitTokens = u.PromptTokensDetails.CachedTokens
		reportedHit = true
	} else if u.FlatCacheHit != nil {
		u.CacheHitTokens = *u.FlatCacheHit
		reportedHit = true
	}
	switch {
	case u.FlatCacheMiss != nil:
		u.CacheMissTokens = *u.FlatCacheMiss
	case reportedHit && u.PromptTokens > 0:
		u.CacheMissTokens = max(0, u.PromptTokens-u.CacheHitTokens)
	}
}

type ToolDef struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters"`
}

type Parameters struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string              `json:"type"`
	Description string              `json:"description,omitempty"`
	Enum        []string            `json:"enum,omitempty"`
	Items       *Property           `json:"items,omitempty"`
	Properties  map[string]Property `json:"properties,omitempty"`
	Required    []string            `json:"required,omitempty"`
}

// BaseClient holds common fields and setters shared by all LLM client implementations.
// Embed this in concrete clients to avoid duplicating struct fields and trivial methods.
type BaseClient struct {
	APIKey          string
	BaseURL         string
	Model           string
	MaxTokens       int
	Temperature     float64
	DisableThinking bool
	thinkingMu      sync.RWMutex // protects DisableThinking (subagent engine mutates concurrently)
	maxTokensMu     sync.RWMutex // protects MaxTokens (merge/summarize mutates concurrently)
}

func (c *BaseClient) SetMaxTokens(n int) {
	c.maxTokensMu.Lock()
	c.MaxTokens = n
	c.maxTokensMu.Unlock()
}
func (c *BaseClient) GetMaxTokens() int {
	c.maxTokensMu.RLock()
	defer c.maxTokensMu.RUnlock()
	return c.MaxTokens
}
func (c *BaseClient) SetDisableThinking(d bool) {
	c.thinkingMu.Lock()
	c.DisableThinking = d
	c.thinkingMu.Unlock()
}
func (c *BaseClient) GetDisableThinking() bool {
	c.thinkingMu.RLock()
	defer c.thinkingMu.RUnlock()
	return c.DisableThinking
}
