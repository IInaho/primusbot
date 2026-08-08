package contextmgr

import (
	"testing"

	"nekocode/bot/contextmgr/token"
)

func TestSnapshotRestore(t *testing.T) {
	first := New(Config{SystemPrompt: "test prompt"})
	first.ConfigureModel(ModelContext{Window: 1_000_000})
	first.Add("user", "hello world")
	first.state.tracker.RecordPrompt(10_000)
	first.state.tracker.RecordCache(9_000, 1_000)
	first.state.tracker.RecordSubagent(300, 200, 100)

	snap := first.Snapshot()

	second := New(Config{SystemPrompt: "test prompt"})
	second.ConfigureModel(ModelContext{Window: 128_000})
	second.state.tracker.RecordSubagent(999, 999, 0)
	second.Restore(snap)

	if got, want := len(second.Snapshot().Messages), len(first.Snapshot().Messages); got != want {
		t.Errorf("restored len = %d, want %d", got, want)
	}
	if budget := second.Status().Budget; budget != 128_000 {
		t.Errorf("restored budget = %d, want active model budget 128000", budget)
	}
	tracker := second.state.tracker.Snapshot()
	if tracker.CacheHitTokens != 0 || tracker.CacheMissTokens != 0 || tracker.LastPromptTokens != 0 {
		t.Errorf("restored provider token state = %+v, want reset", tracker)
	}
	if tracker.Sub != (token.SubStats{Count: 1, TotalTokens: 300, CacheHitTokens: 200, CacheMissTokens: 100}) {
		t.Errorf("restored sub-agent stats = %+v, want snapshot totals", tracker.Sub)
	}
}
