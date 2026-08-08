package tui

import (
	"strings"
	"testing"

	"nekocode/interaction/tui/components"
	"nekocode/interaction/tui/components/message"
	"nekocode/interaction/tui/styles"
)

// TestResizeMessagesStableWidthOnKeystroke simulates the per-keystroke flow:
// with overflowing content, resizeMessages must keep the message dimensions
// stable (horizontal-scrollbar-aware: width stays full, height shrinks by 1
// for the bar) so View() does not invalidate the render cache every frame.
func TestResizeMessagesStableWidthOnKeystroke(t *testing.T) {
	sty := styles.DefaultStyles()
	m := &Model{
		Width:       80,
		Height:      24,
		state:       stateReady,
		Messages:    components.NewMessages(80, 10, &sty),
		Header:      components.NewHeader(80, "0"),
		Input:       components.NewInput(80),
		Suggestions: components.NewSuggestions(&sty),
		Scrollbar:   components.NewScrollbar(&sty),
	}

	// Add enough content to overflow the viewport so the horizontal scrollbar shows.
	big := strings.Repeat("paragraph of conversation text here\n\n", 40)
	m.Messages.AddMessage(message.ChatMessage{Role: "assistant", Content: big})

	// Prime: first resize reserves 1 row for the horizontal scrollbar.
	m.resizeMessages()
	w1 := m.Messages.Width()
	h1 := m.Messages.Height()
	if w1 != 80 {
		t.Fatalf("expected full width 80 (horizontal scrollbar takes height, not width), got %d", w1)
	}

	// Simulate repeated keystrokes: each calls resizeMessages again.
	// Dimensions must stay stable so SetSize does not clear the cache.
	for i := 0; i < 5; i++ {
		m.resizeMessages()
		if m.Messages.Width() != w1 {
			t.Fatalf("resizeMessages iteration %d: width drifted to %d (want %d)", i, m.Messages.Width(), w1)
		}
		if m.Messages.Height() != h1 {
			t.Fatalf("resizeMessages iteration %d: height drifted to %d (want %d)", i, m.Messages.Height(), h1)
		}
	}
}

func TestResizeMessagesKeepsBottomWhenFollowing(t *testing.T) {
	sty := styles.DefaultStyles()
	m := &Model{
		Width:       80,
		Height:      24,
		state:       stateReady,
		Messages:    components.NewMessages(80, 10, &sty),
		Header:      components.NewHeader(80, "0"),
		Input:       components.NewInput(80),
		Suggestions: components.NewSuggestions(&sty),
		ConfirmBar:  components.NewConfirmBar(&sty),
		QuestionBar: components.NewQuestionBar(&sty),
		Scrollbar:   components.NewScrollbar(&sty),
	}

	big := strings.Repeat("remote output line\n", 80)
	m.Messages.AddMessage(message.ChatMessage{Role: "assistant", Content: big})
	m.Messages.GotoBottom()
	if !m.Messages.AtBottom() {
		t.Fatal("expected initial bottom")
	}

	m.state = stateProcessing
	m.resizeMessages()
	if !m.Messages.AtBottom() {
		t.Fatal("resize while following should remain at bottom")
	}
}
