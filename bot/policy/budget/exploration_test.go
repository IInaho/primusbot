package budget

import "testing"

func TestExplorationRecordCallUsesShellSemantics(t *testing.T) {
	tracker := NewExplorationTracker()

	tracker.RecordCall("shell", map[string]any{"command": "go test ./bot/..."})
	if tracker.Score != MaxScore {
		t.Fatalf("verification shell command should not reduce exploration score: got %d", tracker.Score)
	}

	tracker.RecordCall("shell", map[string]any{"command": "rg -n Foo bot"})
	if tracker.Score >= MaxScore {
		t.Fatalf("exploratory shell command should reduce exploration score: got %d", tracker.Score)
	}
}
