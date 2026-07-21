package replacement

import (
	"strings"
	"testing"

	"nekocode/bot/contextmgr/compression"
	"nekocode/bot/contextmgr/context"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider/types"
)

func TestSummarizeReplacesHistoryWithArchiveAndRecentMessages(t *testing.T) {
	ctx := content.New("system")
	for i := 1; i <= 8; i++ {
		ctx.Messages = append(ctx.Messages,
			types.Message{Role: "user", Content: "question " + string(rune('0'+i))},
			types.Message{Role: "assistant", Content: "answer " + string(rune('0'+i))},
		)
	}
	budget := 64000
	tracker := &token.Tracker{}
	trimmed := 0
	s := New(Options{
		Ctx:           &ctx,
		ContextWindow: &budget,
		Tracker:       tracker,
		TrimCount:     &trimmed,
		Summarizer: func(msgs []types.Message, prev string) (string, error) {
			if len(msgs) == 0 {
				t.Fatal("expected messages to summarize")
			}
			return "<summary>This is a compacted project summary with enough detail to pass the quality floor.</summary>", nil
		},
		Cfg: compression.DefaultConfig,
	})

	if err := s.Summarize(); err != nil {
		t.Fatalf("Summarize() error: %v", err)
	}
	if ctx.CompactBoundary != 0 {
		t.Fatalf("CompactBoundary = %d, want 0", ctx.CompactBoundary)
	}
	if len(ctx.Messages) >= 16 {
		t.Fatalf("messages were not replaced: got %d", len(ctx.Messages))
	}
	if !strings.Contains(ctx.Archive, "compacted project summary") {
		t.Fatalf("archive not updated: %q", ctx.Archive)
	}
	if ctx.Messages[0].Content != "question 6" {
		t.Fatalf("first kept message = %q, want question 6", ctx.Messages[0].Content)
	}
	if trimmed != 10 {
		t.Fatalf("trimmed = %d, want 10", trimmed)
	}
}
