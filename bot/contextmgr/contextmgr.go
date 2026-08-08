// Package contextmgr provides a layered context management system for LLM conversations.
package contextmgr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"nekocode/bot/calllog"
	"nekocode/bot/contextmgr/memory"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider"
	"nekocode/bot/provider/types"
)

type Manager struct {
	state *managerState

	// compactionMu serializes slow summary operations without blocking normal
	// context reads and writes for the duration of the model call.
	compactionMu sync.Mutex

	// runtimePrompt is evaluated for every Build and excluded from snapshots.
	runtimePrompt func() string

	usageMu       sync.RWMutex
	usageRecorder func(types.StreamUsage)
}

type managerState struct {
	mu sync.RWMutex

	ctx           contextContent
	contextWindow int
	tracker       *token.Tracker
	prefix        prefixTracker
	revision      uint64
	compactCount  int
	trimCount     int
	compressor    *replacementCompactor
	// Append-only projections remember their latest provider-visible value so
	// unchanged controller state does not add noise to the cached prefix.
	runtimeProjection appendProjection
	hintProjection    appendProjection
	// runtimePolicy is controller-owned, per-run policy state (for example
	// plan mode). BuildRequest folds it into the next tagged runtime snapshot.
	runtimePolicy string
}

type appendProjection struct {
	last string
	seen bool
}

func (p *appendProjection) changed(value string) bool {
	if p.seen && p.last == value {
		return false
	}
	p.last, p.seen = value, true
	return true
}

func (p *appendProjection) reset() { *p = appendProjection{} }

type Config struct {
	SystemPrompt       string
	ContextWindow      int
	AutoCompactPercent int
	Memory             *memory.File
	Summarizer         Summarizer
	CompactionModel    provider.LLM
	RuntimePrompt      func() string
}

var writeCompactionRecord = calllog.Write

func (m *Manager) makeSummarizer(ctx context.Context, client provider.LLM) Summarizer {
	return func(msgs []types.Message, prevSummary string) (string, error) {
		start := time.Now()
		resp, err := client.Chat(ctx, buildSummaryMessages(msgs, prevSummary), nil)
		var summary string
		if err == nil {
			if resp != nil && len(resp.Choices) > 0 {
				summary = resp.Choices[0].Message.Content
			}
			if summary == "" {
				err = fmt.Errorf("no response from summarizer")
			}
		}
		var usage types.StreamUsage
		if resp != nil {
			usage = resp.Usage
			usage.Normalize()
			m.recordLLMUsage(usage)
		}
		m.writeCompactionCall(client, usage, start, err)
		return summary, err
	}
}

func (m *Manager) writeCompactionCall(client provider.LLM, usage types.StreamUsage, start time.Time, callErr error) {
	rec := calllog.Record{TS: time.Now(), Source: "compaction", DurMs: time.Since(start).Milliseconds()}
	rec.SetUsage(usage)
	if source, ok := client.(interface{ RequestMeta() types.RequestMeta }); ok {
		meta := source.RequestMeta()
		rec.Model = meta.Model
		rec.Protocol = meta.Protocol
		rec.BaseURL = calllog.SafeBaseURL(meta.BaseURL)
		rec.RequestedEffort = meta.RequestedEffort
		rec.EffectiveEffort = meta.EffectiveEffort
	}
	rec.Err = calllog.ErrorSummary(callErr)
	writeCompactionRecord(rec)
}

func (m *Manager) recordLLMUsage(usage types.StreamUsage) {
	if usage.PromptTokens <= 0 && usage.CompletionTokens <= 0 {
		return
	}
	m.usageMu.RLock()
	recorder := m.usageRecorder
	m.usageMu.RUnlock()
	if recorder != nil {
		recorder(usage)
	}
}

// SetLLMUsageRecorder connects non-streaming compaction calls to the owning
// run's token meter. It does not affect the per-call JSONL record.
func (m *Manager) SetLLMUsageRecorder(recorder func(types.StreamUsage)) {
	m.usageMu.Lock()
	m.usageRecorder = recorder
	m.usageMu.Unlock()
}

func New(cfg Config) *Manager {
	ctx := newContextContent(cfg.SystemPrompt)
	if cfg.Memory != nil {
		ctx.Memory = cfg.Memory.Build()
	}
	m := &Manager{state: &managerState{
		ctx:           ctx,
		tracker:       &token.Tracker{},
		contextWindow: cfg.ContextWindow,
	}, runtimePrompt: cfg.RuntimePrompt}
	summarizer := cfg.Summarizer
	if summarizer == nil && cfg.CompactionModel != nil {
		summarizer = m.makeSummarizer(context.Background(), cfg.CompactionModel)
	}
	m.initCompressor(summarizer, cfg.AutoCompactPercent)
	return m
}

const defaultAutoCompactPercent = 80

func normalizeAutoCompactPercent(percent int) int {
	if percent < 1 || percent > 99 {
		return defaultAutoCompactPercent
	}
	return percent
}

func (m *Manager) initCompressor(summarizer Summarizer, autoCompactPercent int) {
	m.state.compressor = newReplacementCompactor(summarizer, autoCompactPercent)
}
