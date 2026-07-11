package snapshot

import (
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider/types"
)

type Snapshot struct {
	SystemPrompt    string
	Skills          string
	Archive         string
	Memory          string
	Hints           string
	CompactBoundary int
	Messages        []types.Message
	Budget          int
	Tracker         token.State
}

type Store struct {
	State *state.State
}

func (s *Store) Snapshot() Snapshot {
	s.State.Mu.RLock()
	defer s.State.Mu.RUnlock()
	msgs := make([]types.Message, len(s.State.Ctx.Messages))
	copy(msgs, s.State.Ctx.Messages)
	return Snapshot{
		SystemPrompt:    s.State.Ctx.SystemPrompt,
		Skills:          s.State.Ctx.Skills,
		Archive:         s.State.Ctx.Archive,
		Memory:          s.State.Ctx.Memory,
		Hints:           s.State.Ctx.Hints,
		CompactBoundary: s.State.Ctx.CompactBoundary,
		Messages:        msgs,
		Budget:          s.State.ContextWindow,
		Tracker:         s.State.Tracker.Snapshot(),
	}
}

func (s *Store) Restore(snap Snapshot) {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	s.State.Ctx.SystemPrompt = snap.SystemPrompt
	s.State.Ctx.Skills = snap.Skills
	s.State.Ctx.Archive = snap.Archive
	s.State.Ctx.Memory = snap.Memory
	s.State.Ctx.Hints = snap.Hints
	s.State.Ctx.CompactBoundary = snap.CompactBoundary
	s.State.Ctx.Messages = snap.Messages
	s.State.ContextWindow = snap.Budget
	if s.State.Tracker == nil {
		s.State.Tracker = &token.Tracker{}
	}
	s.State.Tracker.Restore(snap.Tracker)
}
