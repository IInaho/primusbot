package exploration

import "testing"

func TestTrackerRecordAndReset(t *testing.T) {
	tracker := New()
	tracker.Record("read", nil)
	if got := tracker.Value(); got != MaxScore-readCost {
		t.Fatalf("score after read = %d, want %d", got, MaxScore-readCost)
	}

	tracker.Record("edit", map[string]any{"path": "main.go"})
	if got := tracker.Value(); got != MaxScore {
		t.Fatalf("score after edit = %d, want %d", got, MaxScore)
	}

	tracker.Reset()
	if got := tracker.Value(); got != MaxScore {
		t.Fatalf("score after reset = %d, want %d", got, MaxScore)
	}
}
