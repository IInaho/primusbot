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
// the archive plus recent messages in active history.
type replacementCompactor struct {
	summarizer Summarizer
	cfg        compressionConfig
	keepTurns  int
}

func newReplacementCompactor(summarizer Summarizer) *replacementCompactor {
	return &replacementCompactor{
		summarizer: summarizer,
		cfg:        defaultCompressionConfig,
		keepTurns:  3,
	}
}

func (s *replacementCompactor) shouldAutoCompact(history []types.Message, archive string, budget, estimate int) bool {
	level := s.currentLevel(history, archive, budget, estimate)
	return level == compactionRequired || level == compactionBlocking
}

func (s *replacementCompactor) summarize(history []types.Message, prevArchive string, budget int) (string, []types.Message, int, error) {
	if s.summarizer == nil || len(history) <= 2 {
		return prevArchive, history, 0, nil
	}
	keepStart := s.recentStart(history, budget)
	if keepStart <= 0 {
		return prevArchive, history, 0, nil
	}

	toSummarize := append([]types.Message(nil), history[:keepStart]...)
	recent := append([]types.Message(nil), history[keepStart:]...)
	rawSummary, err := s.summarizer(toSummarize, prevArchive)
	if err != nil {
		return "", nil, 0, fmt.Errorf("replacement compact: %w", err)
	}

	archive := formatCompactSummary(rawSummary)
	if len(strings.TrimSpace(archive)) < 50 {
		archive = "[Archive unavailable: summarizer output was malformed or too small; recent conversation was preserved.]"
	}

	logger.Log("replacement_compact: summarized %d msgs, kept %d recent msgs, archive_tokens=%d",
		len(toSummarize), len(recent), token.EstimateString(archive))
	return archive, recent, len(history) - len(recent), nil
}

func (s *replacementCompactor) currentLevel(history []types.Message, archive string, budget, estimate int) compactionLevel {
	if estimate <= 0 {
		estimate = token.EstimateTokens(history) + token.EstimateString(archive)
	}
	return classifyCompaction(s.effectiveBudget(budget)-estimate, s.effectiveConfig(budget))
}

func (s *replacementCompactor) recentStart(history []types.Message, budget int) int {
	start := userTurnBoundary(history, s.keepTurns)
	if start <= 0 {
		return start
	}

	maxRecentTokens := s.effectiveBudget(budget) / 3
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

func (s *replacementCompactor) effectiveBudget(budget int) int {
	if budget > 0 {
		return budget
	}
	return defaultBudget
}

func (s *replacementCompactor) effectiveConfig(configuredBudget int) compressionConfig {
	budget := s.effectiveBudget(configuredBudget)
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
