package contextmgr

import (
	"nekocode/bot/contextmgr/compression"
	"nekocode/bot/contextmgr/internal/history"
	"nekocode/bot/contextmgr/internal/report"
	"nekocode/bot/contextmgr/internal/snapshot"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider"
	"nekocode/bot/provider/types"
	"nekocode/bot/todo"
)

type ToolResultMsg struct {
	Message  types.Message
	ToolName string
}

// ManagerSnapshot captures the full context manager state for session persistence.
type ManagerSnapshot struct {
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

type ContextReport struct {
	Budget          int
	SystemPrompt    int
	TodoText        int
	SkillList       int
	ToolDefTokens   int
	Messages        int
	Archived        int
	ClearedMarkers  int
	CompactCount    int
	TrimCount       int
	ToolDefCount    int
	UserMessages    int
	SysInjections   int
	AssistantMsgs   int
	ToolResults     int
	CacheHitTokens  int
	CacheMissTokens int
	CacheHitRatio   float64
	SubCount        int
	SubTokens       int
	SubCacheHit     int
	SubCacheMiss    int
}

type BarSegment struct {
	Size int
	Kind string
}

func (m *Manager) Build() []types.Message {
	return m.builder.Build()
}

func (m *Manager) Add(role, content string, source ...string) {
	m.history.Add(role, content, source...)
}

func (m *Manager) AddAssistantResponse(content, reasoning string) {
	m.history.AddAssistantResponse(content, reasoning)
}

func (m *Manager) AddAssistantToolCall(content, reasoning string, toolCalls []types.ToolCall) {
	m.history.AddAssistantToolCall(content, reasoning, toolCalls)
}

func (m *Manager) AddToolResultsBatch(results []ToolResultMsg) {
	m.history.AddToolResultsBatch(toHistoryToolResults(results))
}

func (m *Manager) Clear() {
	m.history.Clear()
}

func (m *Manager) TruncateTo(n int) {
	m.history.TruncateTo(n)
}

func (m *Manager) RemoveMessages(startIdx, endIdx int) {
	m.history.RemoveMessages(startIdx, endIdx)
}

func (m *Manager) FreshStart() {
	m.history.FreshStart()
}

func (m *Manager) SetSystemPrompt(s string) {
	m.settings.SetSystemPrompt(s)
}

func (m *Manager) SetSkillList(s string) {
	m.settings.SetSkillList(s)
}

func (m *Manager) SetHints(s string) {
	m.settings.SetHints(s)
}

func (m *Manager) SetContextWindow(budget int) {
	m.settings.SetContextWindow(budget)
}

func (m *Manager) SetTodos(items []todo.Item) {
	m.settings.SetTodos(items)
}

func (m *Manager) AllTasksDone() bool {
	return m.settings.AllTasksDone()
}

func (m *Manager) HasTasks() bool {
	return m.settings.HasTasks()
}

func (m *Manager) Len() int {
	return m.usage.Len()
}

func (m *Manager) Stats() (int, int, bool) {
	return m.usage.Stats()
}

func (m *Manager) RecordUsage(prompt, completion int) {
	m.usage.RecordUsage(prompt, completion)
}

func (m *Manager) RecordCache(hit, miss int) {
	m.usage.RecordCache(hit, miss)
}

func (m *Manager) ResetCache() {
	m.usage.ResetCache()
}

func (m *Manager) TokenUsage() (int, int) {
	return m.usage.TokenUsage()
}

func (m *Manager) RecordSubagent(tokens, hit, miss int) {
	m.usage.RecordSubagent(tokens, hit, miss)
}

func (m *Manager) CacheStats() (hit, miss int) {
	return m.usage.CacheStats()
}

func (m *Manager) Snapshot() ManagerSnapshot {
	return ManagerSnapshot(m.snapshots.Snapshot())
}

func (m *Manager) Restore(s ManagerSnapshot) {
	m.snapshots.Restore(snapshot.Snapshot(s))
}

func (m *Manager) AutoCompactIfNeeded() (compression.Level, error) {
	return m.compaction.AutoCompactIfNeeded()
}

func (m *Manager) NeedsSummarization() bool {
	return m.compaction.NeedsSummarization()
}

func (m *Manager) CompactStats() (compactCount, trimCount int) {
	return m.compaction.CompactStats()
}

func (m *Manager) Summarize() error {
	return m.compaction.Summarize()
}

func (m *Manager) SetCompressionStrategy(strategy compression.Strategy) {
	m.compaction.SetCompressionStrategy(strategy)
}

func (m *Manager) SetSummarizer(summarizer compression.Summarizer) {
	m.compaction.SetSummarizer(summarizer)
}

func (m *Manager) SetMergeClient(client provider.LLM) {
	m.compaction.SetMergeClient(client)
}

func (m *Manager) MergeClient() provider.LLM {
	return m.compaction.MergeClient()
}

func (m *Manager) Report() ContextReport {
	return ContextReport(m.reports.Report())
}

// FormatContextReport renders a context report as a styled string.
func FormatContextReport(r ContextReport) string {
	return report.Format(report.Report(r))
}

func BuildBar(total int, segments []BarSegment, width int) string {
	return report.BuildBar(total, toReportSegments(segments), width)
}

func toHistoryToolResults(results []ToolResultMsg) []history.ToolResultMsg {
	out := make([]history.ToolResultMsg, len(results))
	for i, r := range results {
		out[i] = history.ToolResultMsg{Message: r.Message, ToolName: r.ToolName}
	}
	return out
}

func toReportSegments(segments []BarSegment) []report.BarSegment {
	out := make([]report.BarSegment, len(segments))
	for i, s := range segments {
		out[i] = report.BarSegment{Size: s.Size, Kind: s.Kind}
	}
	return out
}
