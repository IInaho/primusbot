package contextmgr

import (
	"strings"
	"testing"

	"nekocode/bot/provider/types"
)

func TestSummarizeUsesDefaultCompressionStrategy(t *testing.T) {
	m := New(Config{
		SystemPrompt: "system",
		Summarizer: func(msgs []types.Message, prev string) (string, error) {
			return "<summary>Compacted context summary that replaces the hidden full conversation history.</summary>", nil
		},
	})
	for i := 1; i <= 8; i++ {
		m.Add("user", "old question")
		m.Add("assistant", "old answer")
	}

	if err := m.Summarize(); err != nil {
		t.Fatalf("Summarize() error: %v", err)
	}

	snap := m.Snapshot()
	if snap.CompactBoundary != 0 {
		t.Fatalf("CompactBoundary = %d, want 0", snap.CompactBoundary)
	}
	if len(snap.Messages) >= 16 {
		t.Fatalf("messages were not replaced: got %d", len(snap.Messages))
	}

	exported := m.Build()
	joined := joinContents(exported)
	if !strings.Contains(joined, "Compacted context summary") {
		t.Fatalf("exported context missing archive: %q", joined)
	}
}

func joinContents(msgs []types.Message) string {
	var b strings.Builder
	for _, msg := range msgs {
		b.WriteString(msg.Content)
		b.WriteByte('\n')
	}
	return b.String()
}
