package llmstream

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"nekocode/bot/extension/tool/runtime/core"
	"nekocode/bot/provider"
	"nekocode/bot/provider/types"
	"nekocode/logger"
)

// StreamCallbacks holds per-token callbacks for ConsumeStream.
type StreamCallbacks struct {
	OnText      func(delta string)
	OnReasoning func(delta string)
	OnPhase     func(phase string)
	AddTokens   func(prompt, completion int)
	RecordUsage func(prompt, completion int)
	RecordCache func(hit, miss int)
}

// StreamResult accumulates the output of consuming a ChatStream.
type StreamResult struct {
	TextBuf      strings.Builder
	ReasoningBuf strings.Builder
	TcAccum      map[int]*ToolAccum
	LastUsage    *types.StreamUsage
}

// ToolAccum accumulates incremental tool call deltas for a single tool call.
type ToolAccum struct {
	ID   string
	Name string
	Args strings.Builder
}

// LLMCallResult holds the result of a single LLM stream call.
type LLMCallResult struct {
	Text      string
	Reasoning string
	ToolCalls []core.ToolCallItem
}

// LLMCallOptions configures a single LLM call.
type LLMCallOptions struct {
	Ctx       context.Context
	Messages  []types.Message
	ToolDefs  []types.ToolDef
	Callbacks StreamCallbacks
	CheckDone func() bool
}

var ansiRegex = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

// CallLLM executes a single LLM stream call and returns the result.
func CallLLM(client provider.LLM, opts LLMCallOptions) (*LLMCallResult, error) {
	tokenCh, errCh := client.ChatStream(opts.Ctx, opts.Messages, opts.ToolDefs)
	if tokenCh == nil {
		select {
		case err := <-errCh:
			return nil, err
		default:
			return nil, fmt.Errorf("chat stream failed")
		}
	}

	stream := StreamResult{}
	if err := ConsumeStream(tokenCh, &stream, opts.Callbacks, opts.CheckDone); err != nil {
		go func() { <-errCh }()
		return nil, err
	}

	if opts.CheckDone != nil && opts.CheckDone() {
		go func() { <-errCh }()
		return nil, context.Canceled
	}

	if err := <-errCh; err != nil {
		return nil, err
	}

	return &LLMCallResult{
		Text:      ansiRegex.ReplaceAllString(stream.TextBuf.String(), ""),
		Reasoning: stream.ReasoningBuf.String(),
		ToolCalls: stream.CollectToolCalls(),
	}, nil
}

// CallLLMWithRetry calls CallLLM, rebuilding the call options on every
// retryable failure via buildOptions.
func CallLLMWithRetry(ctx context.Context, client provider.LLM, buildOptions func() LLMCallOptions) (*LLMCallResult, error) {
	var result *LLMCallResult
	err := WithRetry(ctx, func() error {
		var err error
		result, err = CallLLM(client, buildOptions())
		return err
	})
	return result, err
}

// WithRetry runs fn under the provider-default retry policy, logging each
// retryable attempt.
func WithRetry(ctx context.Context, fn func() error) error {
	var attempt int
	return provider.Retry(ctx, provider.DefaultRetryConfig, func() error {
		err := fn()
		if err != nil && provider.IsRetryable(err) {
			attempt++
			logger.Log("retry %d: %v", attempt, err)
		}
		return err
	})
}
