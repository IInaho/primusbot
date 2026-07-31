package contextmgr

import (
	"fmt"
	"strings"

	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider/types"
	"nekocode/logger"
)

// replacementCompactor implements replacement-history compaction.
// It summarizes the old visible prefix, then replaces active history with
// the archive plus recent messages instead of retaining hidden pre-boundary
// messages in ctx.Messages.
type replacementCompactor struct {
	state      *managerState
	summarizer Summarizer
	cfg        compressionConfig
	keepTurns  int
}

func newReplacementCompactor(state *managerState, summarizer Summarizer) *replacementCompactor {
	return &replacementCompactor{
		state:      state,
		summarizer: summarizer,
		cfg:        defaultCompressionConfig,
		keepTurns:  3,
	}
}

func (s *replacementCompactor) AutoCompactIfNeeded() error {
	level := s.currentLevel()
	if level == compactionNormal || level == compactionWarning || level == compactionMicro {
		return nil
	}
	if err := s.Summarize(); err != nil {
		return fmt.Errorf("auto compact failed: %w", err)
	}
	if s.currentLevel() == compactionBlocking {
		used := s.estimateTokens()
		budget := s.effectiveBudget()
		return fmt.Errorf("context full: %d tokens used of %d budget (only %d remaining)",
			used, budget, budget-used)
	}
	return nil
}

func (s *replacementCompactor) NeedsSummarization() bool {
	if s.summarizer == nil || len(s.visibleHistory()) <= 20 {
		return false
	}
	return s.estimateTokens() > s.effectiveBudget()*8/10
}

func (s *replacementCompactor) Summarize() error {
	if s.summarizer == nil || s.state == nil {
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
	prevArchive := s.state.ctx.Archive

	rawSummary, err := s.summarizer(toSummarize, prevArchive)
	if err != nil {
		return fmt.Errorf("replacement compact: %w", err)
	}

	archive := formatCompactSummary(rawSummary)
	if len(strings.TrimSpace(archive)) < 50 {
		archive = "[Archive unavailable: summarizer output was malformed or too small; recent conversation was preserved.]"
	}

	oldLen := len(s.state.ctx.Messages)
	s.state.ctx.Archive = archive
	s.state.ctx.Messages = recent
	s.state.ctx.CompactBoundary = 0
	if oldLen > len(recent) {
		s.state.trimCount += oldLen - len(recent)
	}
	if s.state.tracker != nil {
		s.state.tracker.RecordUsage(s.visibleEstimatedTokens(), 0)
	}

	logger.Log("replacement_compact: summarized %d msgs, kept %d recent msgs, archive_tokens=%d",
		len(toSummarize), len(recent), token.EstimateString(archive))
	return nil
}

func (s *replacementCompactor) SetSummarizer(summarizer Summarizer) {
	s.summarizer = summarizer
}

func (s *replacementCompactor) currentLevel() compactionLevel {
	return classifyCompaction(s.effectiveBudget()-s.estimateTokens(), s.effectiveConfig())
}

func (s *replacementCompactor) estimateTokens() int {
	est := s.visibleEstimatedTokens()
	if s.state.tracker != nil {
		if t := s.state.tracker.PromptEstimate(); t > est {
			return t
		}
	}
	return est
}

func (s *replacementCompactor) visibleEstimatedTokens() int {
	return token.EstimateTokens(s.visibleHistory()) + token.EstimateString(s.state.ctx.Archive)
}

func (s *replacementCompactor) visibleHistory() []types.Message {
	if s.state == nil {
		return nil
	}
	if s.state.ctx.CompactBoundary <= 0 {
		return s.state.ctx.Messages
	}
	if s.state.ctx.CompactBoundary >= len(s.state.ctx.Messages) {
		return nil
	}
	return s.state.ctx.Messages[s.state.ctx.CompactBoundary:]
}

func (s *replacementCompactor) recentStart(history []types.Message) int {
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

func (s *replacementCompactor) effectiveBudget() int {
	if s.state.contextWindow > 0 {
		return s.state.contextWindow
	}
	return defaultBudget
}

func (s *replacementCompactor) effectiveConfig() compressionConfig {
	budget := s.effectiveBudget()
	if budget <= defaultBudget {
		return s.cfg
	}
	scale := float64(budget) / float64(defaultBudget)
	return compressionConfig{
		warningBuffer:  int(float64(s.cfg.warningBuffer) * scale),
		microBuffer:    int(float64(s.cfg.microBuffer) * scale),
		compactBuffer:  int(float64(s.cfg.compactBuffer) * scale),
		blockingBuffer: int(float64(s.cfg.blockingBuffer) * scale),
	}
}
