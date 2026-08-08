package provider

import (
	"context"

	"nekocode/bot/provider/anthropic"
	"nekocode/bot/provider/openai"
	"nekocode/bot/provider/types"
)

// LLM is the contract every concrete provider must satisfy. Both
// anthropic.Client and openai.Client implement this interface (verified by
// the compile-time checks below).
type LLM interface {
	Chat(ctx context.Context, messages []types.Message, tools []types.ToolDef) (*types.Response, error)
	ChatStream(ctx context.Context, messages []types.Message, tools []types.ToolDef) (<-chan types.StreamToken, <-chan error)
	SetMaxTokens(n int)
	GetMaxTokens() int
	SetDisableThinking(disable bool)
	GetDisableThinking() bool
}

// Compile-time checks: both concrete clients must satisfy LLM.
var (
	_ LLM = (*anthropic.Client)(nil)
	_ LLM = (*openai.Client)(nil)
)

// Config identifies one model endpoint.
type Config struct {
	APIKey    string
	BaseURL   string
	Model     string
	Protocol  string
	Reasoning types.ReasoningSettings
}

// New creates an LLM client. Protocol may be "openai" or "anthropic".
func New(config Config) LLM {
	switch config.Protocol {
	case "anthropic":
		client := anthropic.New(config.APIKey, config.BaseURL, config.Model)
		client.SetReasoningSettings(config.Reasoning)
		return client
	default:
		client := openai.New(config.APIKey, config.BaseURL, config.Model)
		client.SetReasoningSettings(config.Reasoning)
		return client
	}
}
