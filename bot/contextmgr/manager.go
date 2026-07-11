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

	"nekocode/bot/contextmgr/compression"
	"nekocode/bot/contextmgr/compression/replacement"
	ctxctx "nekocode/bot/contextmgr/context"
	"nekocode/bot/contextmgr/internal/builder"
	compactctl "nekocode/bot/contextmgr/internal/compaction"
	"nekocode/bot/contextmgr/internal/history"
	"nekocode/bot/contextmgr/internal/report"
	"nekocode/bot/contextmgr/internal/settings"
	"nekocode/bot/contextmgr/internal/snapshot"
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/contextmgr/internal/usage"
	"nekocode/bot/contextmgr/memory"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider"
	"nekocode/bot/provider/types"
)

type Manager struct {
	state      *state.State
	history    *history.Store
	builder    *builder.Builder
	usage      *usage.Meter
	settings   *settings.Store
	snapshots  *snapshot.Store
	reports    *report.Builder
	compaction *compactctl.Controller
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
	m := assembleManager(&state.State{
		Ctx:           ctx,
		Tracker:       &token.Tracker{},
		ContextWindow: contextWindow,
		MergeClient:   mergeClient,
	})
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
	m := assembleManager(&state.State{
		Ctx:         ctx,
		Tracker:     &token.Tracker{},
		MergeClient: cfg.MergeClient,
	})
	m.initCompressor(cfg.Summarizer)
	return m
}

func assembleManager(state *state.State) *Manager {
	m := &Manager{state: state}
	m.history = &history.Store{State: state}
	m.builder = &builder.Builder{State: state}
	m.usage = &usage.Meter{State: state}
	m.settings = &settings.Store{State: state}
	m.snapshots = &snapshot.Store{State: state}
	m.reports = &report.Builder{State: state}
	m.compaction = &compactctl.Controller{State: state}
	return m
}

func (m *Manager) initCompressor(summarizer compression.Summarizer) {
	m.state.Compressor = replacement.New(replacement.Options{
		Ctx:           &m.state.Ctx,
		ContextWindow: &m.state.ContextWindow,
		Tracker:       m.state.Tracker,
		TrimCount:     &m.state.TrimCount,
		Summarizer:    summarizer,
		Cfg:           compression.DefaultConfig,
	})
}
