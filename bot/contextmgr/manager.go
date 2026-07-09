// Package contextmgr provides a layered context management system for LLM conversations.
//
// Design rationale: the sub-packages (compact, context, memory, token) are
// organized by domain responsibility rather than as a monolithic module.
// Each sub-package has its own test suite and can be evolved independently.
// This is intentional — aggressive merging would destroy test isolation
// without measurable benefit.
package contextmgr

import (
	"context"
	"fmt"
	"sync"

	"nekocode/bot/contextmgr/compression"
	"nekocode/bot/contextmgr/compression/replacement"
	ctxctx "nekocode/bot/contextmgr/context"
	"nekocode/bot/contextmgr/memory"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider"
	"nekocode/bot/provider/types"
)

type Manager struct {
	mu            sync.RWMutex
	ctx           ctxctx.Content
	ContextWindow int
	Tracker       *token.Tracker
	CompactCount  int
	TrimCount     int
	mem           *memory.File
	Compressor    compression.Strategy
	MergeClient   provider.LLM // for independent merge archive sessions
}

type Config struct {
	SystemPrompt string
	Memory       *memory.File
	Summarizer   compression.Summarizer
	MergeClient  provider.LLM
}

// NewSub creates a lightweight Manager for subagents.
// Compression is only enabled when mergeClient is non-nil.
func NewSub(systemPrompt string, contextWindow int, mergeClient provider.LLM) *Manager {
	ctx := ctxctx.New(systemPrompt)
	m := &Manager{
		ctx:           ctx,
		Tracker:       &token.Tracker{},
		ContextWindow: contextWindow,
	}
	if mergeClient != nil {
		mergeCtx := context.Background()
		m.initCompressor(MakeSummarizer(mergeCtx, mergeClient))
	}
	return m
}

// MakeSummarizer creates a Summarizer func from an LLM client.
// The provided context is used for LLM calls, enabling cancellation.
func MakeSummarizer(ctx context.Context, client provider.LLM) compression.Summarizer {
	return func(msgs []types.Message, prevSummary string) (string, error) {
		prompt := compression.BuildPrompt(msgs, prevSummary)
		resp, err := client.Chat(ctx, []types.Message{{Role: "user", Content: prompt}}, nil)
		if err != nil {
			return "", err
		}
		if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != "" {
			return resp.Choices[0].Message.Content, nil
		}
		return "", fmt.Errorf("no response from summarizer")
	}
}

func New(cfg Config) *Manager {
	ctx := ctxctx.New(cfg.SystemPrompt)
	if cfg.Memory != nil {
		ctx.Memory = cfg.Memory.Build()
	}
	m := &Manager{
		ctx:         ctx,
		Tracker:     &token.Tracker{},
		mem:         cfg.Memory,
		MergeClient: cfg.MergeClient,
	}
	m.initCompressor(cfg.Summarizer)
	return m
}

func (m *Manager) initCompressor(summarizer compression.Summarizer) {
	m.Compressor = replacement.New(replacement.Options{
		Ctx:           &m.ctx,
		ContextWindow: &m.ContextWindow,
		Tracker:       m.Tracker,
		TrimCount:     &m.TrimCount,
		Summarizer:    summarizer,
		Cfg:           compression.DefaultConfig,
	})
}
