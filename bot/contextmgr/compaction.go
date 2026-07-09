package contextmgr

import (
	"context"

	"nekocode/bot/contextmgr/compression"
)

func (m *Manager) AutoCompactIfNeeded() (compression.Level, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Compressor != nil {
		return m.Compressor.AutoCompactIfNeeded()
	}
	return compression.LevelNormal, nil
}

func (m *Manager) NeedsSummarization() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.Compressor != nil {
		return m.Compressor.NeedsSummarization()
	}
	return false
}

func (m *Manager) CompactStats() (compactCount, trimCount int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.CompactCount, m.TrimCount
}

func (m *Manager) Summarize() error {
	if m.Compressor == nil {
		return nil
	}
	m.mu.Lock()
	prevArchive := m.ctx.Archive
	if err := m.Compressor.Summarize(); err != nil {
		m.mu.Unlock()
		return err
	}
	newArchive := m.ctx.Archive
	m.mu.Unlock()

	if prevArchive != "" && newArchive != "" && m.MergeClient != nil {
		merged := compression.MergeSummaries(context.Background(), m.MergeClient, prevArchive, newArchive)

		m.mu.Lock()
		m.ctx.Archive = merged
		m.mu.Unlock()
	}
	return nil
}

func (m *Manager) SetCompressionStrategy(strategy compression.Strategy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Compressor = strategy
}

func (m *Manager) SetSummarizer(summarizer compression.Summarizer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Compressor != nil {
		m.Compressor.SetSummarizer(summarizer)
	}
}
