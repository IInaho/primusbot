package app

import (
	"nekocode/bot/contextmgr/memory"
	"nekocode/bot/view"
)

// api_memory.go — Bot API：记忆视图。

func (b *Bot) MemoryView(scope view.MemoryScope) view.MemoryView {
	b.mu.Lock()
	defer b.mu.Unlock()

	if scope == "" {
		scope = view.MemoryScopeProject
	}
	snap := b.ctxMgr.Snapshot()
	return view.NewMemoryView(scope, memory.DefaultPath(), snap.Memory)
}
