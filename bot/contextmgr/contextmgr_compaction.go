package contextmgr

import (
	"fmt"

	"nekocode/bot/provider/types"
)

type compactionSnapshot struct {
	history  []types.Message
	archive  string
	budget   int
	estimate int
	revision uint64
}

// AutoCompactIfNeeded compacts only when the context has crossed the automatic
// threshold. The model call runs without holding the context state lock.
func (m *Manager) AutoCompactIfNeeded() (bool, error) {
	return m.compact(false)
}

// Summarize compacts the current history regardless of its token occupancy.
// It reports whether any messages were actually replaced.
func (m *Manager) Summarize() (bool, error) {
	return m.compact(true)
}

func (m *Manager) compact(force bool) (bool, error) {
	m.compactionMu.Lock()
	defer m.compactionMu.Unlock()

	snap, compressor := m.compactionInput()
	if compressor == nil {
		return false, nil
	}
	if !force && !compressor.shouldAutoCompact(snap.history, snap.archive, snap.budget, snap.estimate) {
		return false, nil
	}

	archive, recent, trimmed, err := compressor.summarize(snap.history, snap.archive, snap.budget)
	if err != nil {
		if !force {
			return false, fmt.Errorf("auto compact failed: %w", err)
		}
		return false, err
	}
	if trimmed == 0 {
		if !force && compressor.currentLevel(snap.history, snap.archive, snap.budget, snap.estimate) == compactionBlocking {
			return false, contextFullError(snap.estimate, snap.budget)
		}
		return false, nil
	}

	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	if m.state.revision != snap.revision {
		return false, fmt.Errorf("context changed while compacting; summary was not applied")
	}
	m.state.ctx.Archive = archive
	m.state.ctx.Messages = recent
	m.state.revision++
	m.state.compactCount++
	m.state.trimCount += trimmed
	m.state.tracker.RecordPrompt(m.totalTokenEstimate())

	if !force && compressor.currentLevel(recent, archive, snap.budget, m.estimatedTokens()) == compactionBlocking {
		return true, contextFullError(m.estimatedTokens(), snap.budget)
	}
	return true, nil
}

func contextFullError(used, budget int) error {
	if budget <= 0 {
		budget = defaultBudget
	}
	return fmt.Errorf("context full: %d tokens used of %d budget (only %d remaining)",
		used, budget, budget-used)
}

func (m *Manager) compactionInput() (compactionSnapshot, *replacementCompactor) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	history := append([]types.Message(nil), m.state.ctx.Messages...)
	return compactionSnapshot{
		history: history, archive: m.state.ctx.Archive,
		budget: m.state.contextWindow, estimate: m.estimatedTokens(),
		revision: m.state.revision,
	}, cloneCompactor(m.state.compressor)
}

func cloneCompactor(src *replacementCompactor) *replacementCompactor {
	if src == nil {
		return nil
	}
	copy := *src
	return &copy
}
