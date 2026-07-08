package budget

import "testing"

func TestConsumeCallCountsExploratoryShell(t *testing.T) {
	q := ToolQuota{MaxSlots: 1}
	if err := q.ConsumeCall("shell", map[string]any{"command": "cat README.md"}); err != nil {
		t.Fatalf("first exploratory shell command should fit quota: %v", err)
	}
	if err := q.ConsumeCall("shell", map[string]any{"command": "ls -la"}); err == nil {
		t.Fatal("second exploratory shell command should exceed quota")
	}
}

func TestConsumeCallDoesNotCountVerificationShell(t *testing.T) {
	q := ToolQuota{MaxSlots: 1}
	if err := q.ConsumeCall("shell", map[string]any{"command": "go test ./..."}); err != nil {
		t.Fatalf("verification shell command should not consume read quota: %v", err)
	}
	if q.Used != 0 {
		t.Fatalf("verification shell command consumed quota: %d", q.Used)
	}
}
