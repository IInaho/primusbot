package contextmgr

import (
	"context"

	"nekocode/bot/provider"
)

func (m *Manager) AutoCompactIfNeeded() error {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	if m.state.compressor != nil {
		return m.state.compressor.AutoCompactIfNeeded()
	}
	return nil
}

func (m *Manager) NeedsSummarization() bool {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	if m.state.compressor != nil {
		return m.state.compressor.NeedsSummarization()
	}
	return false
}

func (m *Manager) Summarize() error {
	if m.state.compressor == nil {
		return nil
	}
	m.state.mu.Lock()
	prevArchive := m.state.ctx.Archive
	if err := m.state.compressor.Summarize(); err != nil {
		m.state.mu.Unlock()
		return err
	}
	newArchive := m.state.ctx.Archive
	compactionModel := m.state.compactionModel
	m.state.mu.Unlock()

	if prevArchive != "" && newArchive != "" && compactionModel != nil {
		merged := mergeSummaries(context.Background(), compactionModel, prevArchive, newArchive)

		m.state.mu.Lock()
		m.state.ctx.Archive = merged
		m.state.mu.Unlock()
	}
	return nil
}

func (m *Manager) SetCompactionModel(model provider.LLM) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	m.state.compactionModel = model
	if m.state.compressor != nil {
		var summarizer Summarizer
		if model != nil {
			summarizer = makeSummarizer(context.Background(), model)
		}
		m.state.compressor.SetSummarizer(summarizer)
	}
}
