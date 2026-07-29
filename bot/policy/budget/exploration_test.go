package budget

import "testing"

func TestExplorationRecordCallUsesShellSemantics(t *testing.T) {
	tracker := NewExplorationTracker()

	tracker.RecordCall("shell", map[string]any{"command": "go test ./bot/..."})
	if got := tracker.Score.Load(); got != MaxScore {
		t.Fatalf("verification shell command should not reduce exploration score: got %d", got)
	}

	tracker.RecordCall("shell", map[string]any{"command": "rg -n Foo bot"})
	if got := tracker.Score.Load(); got >= MaxScore {
		t.Fatalf("exploratory shell command should reduce exploration score: got %d", got)
	}
}
