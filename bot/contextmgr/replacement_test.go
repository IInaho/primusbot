package contextmgr

import (
	"strings"
	"testing"

	"nekocode/bot/provider/types"
)

func TestSummarizeReplacesHistoryWithArchiveAndRecentMessages(t *testing.T) {
	ctx := newContextContent("system")
	for i := 1; i <= 8; i++ {
		ctx.Messages = append(ctx.Messages,
			types.Message{Role: "user", Content: "question " + string(rune('0'+i))},
			types.Message{Role: "assistant", Content: "answer " + string(rune('0'+i))},
		)
	}
	budget := 64000
	s := newReplacementCompactor(func(msgs []types.Message, prev string) (string, error) {
		if len(msgs) == 0 {
			t.Fatal("expected messages to summarize")
		}
		return "<summary>This is a compacted project summary with enough detail to pass the quality floor.</summary>", nil
	})

	archive, recent, trimmed, err := s.summarize(ctx.Messages, "", budget)
	if err != nil {
		t.Fatalf("Summarize() error: %v", err)
	}
	if len(recent) >= 16 {
		t.Fatalf("messages were not replaced: got %d", len(recent))
	}
	if !strings.Contains(archive, "compacted project summary") {
		t.Fatalf("archive not updated: %q", archive)
	}
	if recent[0].Content != "question 6" {
		t.Fatalf("first kept message = %q, want question 6", recent[0].Content)
	}
	if trimmed != 10 {
		t.Fatalf("trimmed = %d, want 10", trimmed)
	}
}
