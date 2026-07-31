package contextmgr

import (
	"strings"
	"testing"

	"nekocode/bot/contextmgr/token"
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
	state := &managerState{
		ctx:           ctx,
		contextWindow: budget,
		tracker:       &token.Tracker{},
	}
	s := newReplacementCompactor(state, func(msgs []types.Message, prev string) (string, error) {
		if len(msgs) == 0 {
			t.Fatal("expected messages to summarize")
		}
		return "<summary>This is a compacted project summary with enough detail to pass the quality floor.</summary>", nil
	})

	if err := s.Summarize(); err != nil {
		t.Fatalf("Summarize() error: %v", err)
	}
	if state.ctx.CompactBoundary != 0 {
		t.Fatalf("CompactBoundary = %d, want 0", state.ctx.CompactBoundary)
	}
	if len(state.ctx.Messages) >= 16 {
		t.Fatalf("messages were not replaced: got %d", len(state.ctx.Messages))
	}
	if !strings.Contains(state.ctx.Archive, "compacted project summary") {
		t.Fatalf("archive not updated: %q", state.ctx.Archive)
	}
	if state.ctx.Messages[0].Content != "question 6" {
		t.Fatalf("first kept message = %q, want question 6", state.ctx.Messages[0].Content)
	}
	if state.trimCount != 10 {
		t.Fatalf("trimmed = %d, want 10", state.trimCount)
	}
}
