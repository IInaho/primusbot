package contextmgr

import "testing"

func TestSnapshotRestore(t *testing.T) {
	first := New(Config{SystemPrompt: "test prompt"})
	first.SetContextWindow(50000)
	first.Add("user", "hello world")

	snap := first.Snapshot()

	second := New(Config{SystemPrompt: "test prompt"})
	second.Restore(snap)

	if got, want := len(second.Snapshot().Messages), len(first.Snapshot().Messages); got != want {
		t.Errorf("restored len = %d, want %d", got, want)
	}
	if _, budget := second.TokenUsage(); budget != 50000 {
		t.Errorf("restored budget = %d, want 50000", budget)
	}
}
