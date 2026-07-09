package legacy

import "nekocode/bot/contextmgr/compression"

type Level = compression.Level

const (
	LevelNormal       = compression.LevelNormal
	LevelWarning      = compression.LevelWarning
	LevelMicroCompact = compression.LevelMicroCompact
	LevelCompact      = compression.LevelCompact
	LevelBlocking     = compression.LevelBlocking
)

type Config = compression.Config
type Summarizer = compression.Summarizer

var DefaultConfig = compression.DefaultConfig

func ClassifyLevel(remaining int, cfg Config) Level {
	return compression.ClassifyLevel(remaining, cfg)
}

func classifyLevel(remaining int, cfg Config) Level {
	return ClassifyLevel(remaining, cfg)
}

const ClearedMarker = compression.ClearedMarker
