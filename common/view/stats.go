package view

import (
	"time"

	"nekocode/util/duration"
)

type BotStatsInput struct {
	PromptTokens     int
	CompletionTokens int
	TurnPrompt       int
	TurnCompletion   int
	ContextTokens    int
	CompactCount     int
	Duration         time.Duration
}

func NewBotStats(in BotStatsInput) BotStats {
	return BotStats{
		PromptTokens:     in.PromptTokens,
		CompletionTokens: in.CompletionTokens,
		TurnPrompt:       in.TurnPrompt,
		TurnCompletion:   in.TurnCompletion,
		ContextTokens:    in.ContextTokens,
		CompactCount:     in.CompactCount,
		Duration:         duration.FormatDuration(in.Duration),
	}
}
