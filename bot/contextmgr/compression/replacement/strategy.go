package replacement

import (
	"fmt"
	"strings"

	"nekocode/bot/contextmgr/compression"
	ctxctx "nekocode/bot/contextmgr/context"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider/types"
	"nekocode/common/debug"
)

const defaultBudget = 64000

type Options struct {
	Ctx           *ctxctx.Content
	ContextWindow *int
	Tracker       *token.Tracker
	TrimCount     *int
	Summarizer    compression.Summarizer
	Cfg           compression.Config
	KeepTurns     int
}

// Strategy implements replacement-history compaction.
// It summarizes the old visible prefix, then replaces active history with
// the archive plus recent messages instead of retaining hidden pre-boundary
// messages in ctx.Messages.
type Strategy struct {
	ctx           *ctxctx.Content
	contextWindow *int
	tracker       *token.Tracker
	trimCount     *int
	summarizer    compression.Summarizer
	cfg           compression.Config
	keepTurns     int
}

func New(opts Options) *Strategy {
	cfg := opts.Cfg
	if cfg == (compression.Config{}) {
		cfg = compression.DefaultConfig
	}
	keepTurns := opts.KeepTurns
	if keepTurns <= 0 {
		keepTurns = 3
	}
	return &Strategy{
		ctx:           opts.Ctx,
		contextWindow: opts.ContextWindow,
		tracker:       opts.Tracker,
		trimCount:     opts.TrimCount,
		summarizer:    opts.Summarizer,
		cfg:           cfg,
		keepTurns:     keepTurns,
	}
}

func (s *Strategy) AutoCompactIfNeeded() (compression.Level, error) {
	level := s.currentLevel()
	if level == compression.LevelNormal || level == compression.LevelWarning || level == compression.LevelMicroCompact {
		return level, nil
	}
	if err := s.Summarize(); err != nil {
		return level, fmt.Errorf("auto compact failed: %w", err)
	}
	if s.currentLevel() == compression.LevelBlocking {
		used := s.estimateTokens()
		budget := s.effectiveBudget()
		return compression.LevelBlocking, fmt.Errorf("context full: %d tokens used of %d budget (only %d remaining)",
			used, budget, budget-used)
	}
	return compression.LevelCompact, nil
}

func (s *Strategy) NeedsSummarization() bool {
	if s.summarizer == nil || len(s.visibleHistory()) <= 20 {
		return false
	}
	return s.estimateTokens() > s.effectiveBudget()*8/10
}

func (s *Strategy) Summarize() error {
	if s.summarizer == nil || s.ctx == nil {
		return nil
	}
	history := s.visibleHistory()
	if len(history) <= 2 {
		return nil
	}

	keepStart := s.recentStart(history)
	if keepStart <= 0 {
		return nil
	}

	toSummarize := append([]types.Message(nil), history[:keepStart]...)
	recent := append([]types.Message(nil), history[keepStart:]...)
	prevArchive := s.ctx.Archive

	rawSummary, err := s.summarizer(toSummarize, prevArchive)
	if err != nil {
		return fmt.Errorf("replacement compact: %w", err)
	}

	archive := compression.FormatCompactSummary(rawSummary)
	if len(strings.TrimSpace(archive)) < 50 {
		archive = "[Archive unavailable: summarizer output was malformed or too small; recent conversation was preserved.]"
	}

	oldLen := len(s.ctx.Messages)
	s.ctx.Archive = archive
	s.ctx.Messages = recent
	s.ctx.CompactBoundary = 0
	if s.trimCount != nil && oldLen > len(recent) {
		*s.trimCount += oldLen - len(recent)
	}
	if s.tracker != nil {
		s.tracker.RecordUsage(s.visibleEstimatedTokens(), 0)
	}

	debug.Log("replacement_compact: summarized %d msgs, kept %d recent msgs, archive_tokens=%d",
		len(toSummarize), len(recent), token.EstimateString(archive))
	return nil
}

func (s *Strategy) SetSummarizer(summarizer compression.Summarizer) {
	s.summarizer = summarizer
}

func (s *Strategy) currentLevel() compression.Level {
	return compression.ClassifyLevel(s.effectiveBudget()-s.estimateTokens(), s.effectiveConfig())
}

func (s *Strategy) estimateTokens() int {
	est := s.visibleEstimatedTokens()
	if s.tracker != nil {
		if t := s.tracker.PromptEstimate(); t > est {
			return t
		}
	}
	return est
}

func (s *Strategy) visibleEstimatedTokens() int {
	return token.EstimateTokens(s.visibleHistory()) + token.EstimateString(s.ctx.Archive)
}

func (s *Strategy) visibleHistory() []types.Message {
	if s.ctx == nil {
		return nil
	}
	if s.ctx.CompactBoundary <= 0 {
		return s.ctx.Messages
	}
	if s.ctx.CompactBoundary >= len(s.ctx.Messages) {
		return nil
	}
	return s.ctx.Messages[s.ctx.CompactBoundary:]
}

func (s *Strategy) recentStart(history []types.Message) int {
	start := userTurnBoundary(history, s.keepTurns)
	if start <= 0 {
		return start
	}

	maxRecentTokens := s.effectiveBudget() / 3
	if maxRecentTokens < 2000 {
		maxRecentTokens = 2000
	}
	for token.EstimateTokens(history[start:]) > maxRecentTokens {
		next := nextUserBoundary(history, start+1)
		if next < 0 {
			break
		}
		start = next
	}
	return start
}

func userTurnBoundary(msgs []types.Message, turns int) int {
	if turns <= 0 {
		return len(msgs)
	}
	count := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			count++
			if count >= turns {
				return i
			}
		}
	}
	return 0
}

func nextUserBoundary(msgs []types.Message, start int) int {
	for i := start; i < len(msgs); i++ {
		if msgs[i].Role == "user" {
			return i
		}
	}
	return -1
}

func (s *Strategy) effectiveBudget() int {
	if s.contextWindow != nil && *s.contextWindow > 0 {
		return *s.contextWindow
	}
	return defaultBudget
}

func (s *Strategy) effectiveConfig() compression.Config {
	budget := s.effectiveBudget()
	if budget <= defaultBudget {
		return s.cfg
	}
	scale := float64(budget) / float64(defaultBudget)
	return compression.Config{
		WarningBuffer:      int(float64(s.cfg.WarningBuffer) * scale),
		MicroCompactBuffer: int(float64(s.cfg.MicroCompactBuffer) * scale),
		CompactBuffer:      int(float64(s.cfg.CompactBuffer) * scale),
		BlockingBuffer:     int(float64(s.cfg.BlockingBuffer) * scale),
	}
}
