package components

import (
	"strings"
	"testing"

	"nekocode/interaction/tui/components/message"
	"nekocode/interaction/tui/styles"
)

func TestSetProcessingOffKeepsBottomWhenFollowing(t *testing.T) {
	sty := styles.DefaultStyles()
	m := NewMessages(80, 8, &sty)
	m.AddMessage(message.ChatMessage{
		Role:    "assistant",
		Content: strings.Repeat("command output line\n", 80),
	})
	m.GotoBottom()
	if !m.AtBottom() {
		t.Fatal("expected initial bottom")
	}

	m.SetProcessing(true)
	if !m.AtBottom() {
		t.Fatal("expected processing start to remain at bottom")
	}

	m.SetProcessing(false)
	if !m.AtBottom() {
		t.Fatal("expected processing removal to remain at bottom")
	}
}
