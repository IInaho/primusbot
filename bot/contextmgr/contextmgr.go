// Package contextmgr provides a layered context management system for LLM conversations.
package contextmgr

import (
	"context"
	"fmt"
	"sync"

	"nekocode/bot/contextmgr/memory"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider"
	"nekocode/bot/provider/types"
)

type Manager struct {
	state *managerState
}

type compactor interface {
	AutoCompactIfNeeded() error
	NeedsSummarization() bool
	Summarize() error
	SetSummarizer(Summarizer)
}

type managerState struct {
	mu sync.RWMutex

	ctx             contextContent
	contextWindow   int
	tracker         *token.Tracker
	compactCount    int
	trimCount       int
	compressor      compactor
	compactionModel provider.LLM
}

type Config struct {
	SystemPrompt    string
	ContextWindow   int
	Memory          *memory.File
	Summarizer      Summarizer
	CompactionModel provider.LLM
}

func makeSummarizer(ctx context.Context, client provider.LLM) Summarizer {
	return func(msgs []types.Message, prevSummary string) (string, error) {
		prompt := buildSummaryPrompt(msgs, prevSummary)
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
	ctx := newContextContent(cfg.SystemPrompt)
	if cfg.Memory != nil {
		ctx.Memory = cfg.Memory.Build()
	}
	m := &Manager{state: &managerState{
		ctx:             ctx,
		tracker:         &token.Tracker{},
		contextWindow:   cfg.ContextWindow,
		compactionModel: cfg.CompactionModel,
	}}
	summarizer := cfg.Summarizer
	if summarizer == nil && cfg.CompactionModel != nil {
		summarizer = makeSummarizer(context.Background(), cfg.CompactionModel)
	}
	m.initCompressor(summarizer)
	return m
}

func (m *Manager) initCompressor(summarizer Summarizer) {
	m.state.compressor = newReplacementCompactor(m.state, summarizer)
}
