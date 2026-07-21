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
