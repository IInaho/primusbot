package tui

import (
	"strings"
	"testing"

	"nekocode/tui/components"
	"nekocode/tui/components/message"
	"nekocode/tui/styles"
)

// TestResizeMessagesStableWidthOnKeystroke simulates the per-keystroke flow:
// with overflowing content, resizeMessages must keep the message width stable
// (scrollbar-aware) so View() does not invalidate the render cache every frame.
func TestResizeMessagesStableWidthOnKeystroke(t *testing.T) {
	sty := styles.DefaultStyles()
	m := &Model{
		Width:       80,
		Height:      24,
		state:       stateReady,
		Messages:    components.NewMessages(80, 10, &sty),
		Header:      components.NewHeader(80, "prov", "model", "0"),
		Input:       components.NewInput(80),
		Suggestions: components.NewSuggestions(&sty),
		Scrollbar:   components.NewScrollbar(&sty),
	}

	// Add enough content to overflow the 10-line viewport so the scrollbar shows.
	big := strings.Repeat("paragraph of conversation text here\n\n", 40)
	m.Messages.AddMessage(message.ChatMessage{Role: "assistant", Content: big})

	// Prime: first resize computes the scrollbar-aware width.
	m.resizeMessages()
	w1 := m.Messages.Width()
	if w1 != 79 {
		t.Fatalf("expected scrollbar-aware width 79, got %d", w1)
	}

	// Simulate repeated keystrokes: each calls resizeMessages again.
	// Width must stay stable so SetSize does not clear the cache.
	for i := 0; i < 5; i++ {
		m.resizeMessages()
		if m.Messages.Width() != w1 {
			t.Fatalf("resizeMessages iteration %d: width drifted to %d (want %d)", i, m.Messages.Width(), w1)
		}
	}
}
