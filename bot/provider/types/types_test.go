package types

import (
	"testing"

	"nekocode/bot/reasoning"
)

func TestReasoningForRequestReplaysSignatureOnlyBlock(t *testing.T) {
	message := Message{Role: "assistant", ReasoningSignature: "sig", ToolCalls: []ToolCall{{ID: "call-1"}}}
	content, replay := ReasoningForRequest(message, ReasoningSettings{Replay: reasoning.ReplaySigned})
	if !replay || content != "" {
		t.Fatalf("signature-only thinking = %q, %v; want empty content replayed", content, replay)
	}
}
