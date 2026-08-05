package llmstream

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"nekocode/bot/calllog"
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
	Usage        types.StreamUsage
	Request      *types.RequestMeta
	FirstTokenAt time.Time
}

// mergeUsage folds one usage report into the accumulated totals. Providers
// split usage across chunks (anthropic reports prompt/cache at message_start
// and completion at message_delta), so the last report alone loses fields.
func mergeUsage(acc *types.StreamUsage, u *types.StreamUsage) {
	if u.PromptTokens > 0 {
		acc.PromptTokens = u.PromptTokens
	}
	if u.CompletionTokens > 0 {
		acc.CompletionTokens = u.CompletionTokens
	}
	if u.CacheHitTokens > 0 {
		acc.CacheHitTokens = u.CacheHitTokens
	}
	if u.CacheMissTokens > 0 {
		acc.CacheMissTokens = u.CacheMissTokens
	}
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
	// Source labels the call origin ("main", "synthesize", "subagent") in
	// the evidence log; Diagnostics reports the request's prefix fingerprint.
	Source      string
	Diagnostics func() calllog.PrefixDiag
}

var ansiRegex = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

// CallLLM executes a single LLM stream call and returns the result. Every
// call — success or failure — leaves one structured record in the evidence
// log (calllog.Write).
func CallLLM(client provider.LLM, opts LLMCallOptions) (*LLMCallResult, error) {
	start := time.Now()
	tokenCh, errCh := client.ChatStream(opts.Ctx, opts.Messages, opts.ToolDefs)
	if tokenCh == nil {
		err := fmt.Errorf("chat stream failed")
		select {
		case streamErr := <-errCh:
			err = streamErr
		default:
		}
		writeCallRecord(opts, &StreamResult{}, start, err)
		return nil, err
	}

	stream := StreamResult{}
	result, err := func() (*LLMCallResult, error) {
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
	}()
	writeCallRecord(opts, &stream, start, err)
	return result, err
}

// writeRecord and dumpBody are seams for tests.
var (
	writeRecord = calllog.Write
	dumpBody    = calllog.DumpBodyOnSevereMiss
)

// writeCallRecord assembles and appends the per-call evidence record,
// dumping the wire body when the call was a cache collapse.
func writeCallRecord(opts LLMCallOptions, stream *StreamResult, start time.Time, callErr error) {
	rec := calllog.Record{
		TS:               time.Now(),
		Source:           opts.Source,
		DurMs:            time.Since(start).Milliseconds(),
		PromptTokens:     stream.Usage.PromptTokens,
		CacheHitTokens:   stream.Usage.CacheHitTokens,
		CacheMissTokens:  stream.Usage.CacheMissTokens,
		CompletionTokens: stream.Usage.CompletionTokens,
	}
	if rec.Source == "" {
		rec.Source = "unknown"
	}
	if !stream.FirstTokenAt.IsZero() {
		rec.TTFTMs = stream.FirstTokenAt.Sub(start).Milliseconds()
	}
	if callErr != nil {
		rec.Err = callErr.Error()
	}
	if meta := stream.Request; meta != nil {
		rec.Model = meta.Model
		rec.Protocol = meta.Protocol
		rec.BaseURL = meta.BaseURL
		rec.BodySHA256 = meta.BodySHA256
		rec.BodyBytes = meta.BodyBytes
	}
	if opts.Diagnostics != nil {
		rec.PrefixDiag = opts.Diagnostics()
	}
	dumpBody(&rec, requestBody(stream))
	writeRecord(rec)
}

// requestBody returns the raw wire body for forensics, nil for streams that
// never reported their request.
func requestBody(stream *StreamResult) []byte {
	if stream.Request == nil {
		return nil
	}
	return stream.Request.Body
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
