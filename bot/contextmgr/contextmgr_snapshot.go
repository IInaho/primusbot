package contextmgr

import (
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider/types"
)

type ManagerSnapshot struct {
	SystemPrompt string
	Skills       string
	Archive      string
	Memory       string
	Hints        string
	Messages     []types.Message
	Budget       int
	Tracker      token.State
}

func (m *Manager) Snapshot() ManagerSnapshot {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	msgs := make([]types.Message, len(m.state.ctx.Messages))
	copy(msgs, m.state.ctx.Messages)
	return ManagerSnapshot{
		SystemPrompt: m.state.ctx.SystemPrompt,
		Skills:       m.state.ctx.Skills,
		Archive:      m.state.ctx.Archive,
		Memory:       m.state.ctx.Memory,
		Hints:        m.state.ctx.Hints,
		Messages:     msgs,
		Budget:       m.state.contextWindow,
		Tracker:      m.state.tracker.Snapshot(),
	}
}

func (m *Manager) Restore(snap ManagerSnapshot) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.ctx.SystemPrompt = snap.SystemPrompt
	m.state.ctx.Skills = snap.Skills
	m.state.ctx.Archive = snap.Archive
	m.state.ctx.Memory = snap.Memory
	m.state.ctx.Hints = snap.Hints
	m.state.runtimePolicy = ""
	m.state.ctx.Messages = append([]types.Message(nil), snap.Messages...)
	m.restoreRuntimeContextLocked()
	if m.state.tracker == nil {
		m.state.tracker = &token.Tracker{}
	}
	// Model limits and provider-calibrated usage belong to the active runtime,
	// not to persisted conversation content. A session may be resumed with a
	// different model, so retaining either can delay compaction past the new
	// model's actual context window. Sub-agent totals are session history and
	// remain valid independently of the active model.
	m.state.tracker.Restore(token.State{Sub: snap.Tracker.Sub})
	m.state.prefix.Reset()
	m.state.compactCount = 0
	m.state.trimCount = 0
	m.state.revision++
}
