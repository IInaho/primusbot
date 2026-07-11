package state

import (
	"sync"

	"nekocode/bot/contextmgr/compression"
	ctxctx "nekocode/bot/contextmgr/context"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider"
)

type State struct {
	Mu sync.RWMutex

	Ctx           ctxctx.Content
	ContextWindow int
	Tracker       *token.Tracker
	CompactCount  int
	TrimCount     int
	Compressor    compression.Strategy
	MergeClient   provider.LLM
}
