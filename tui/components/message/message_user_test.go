package message

import (
	"strings"
	"testing"

	"nekocode/tui/styles"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestUserMessageUsesPromptStyle(t *testing.T) {
	sty := styles.DefaultStyles()
	m := NewUserMessageItem(&sty, "Please update the README.")

	clean := ansi.Strip(m.Render(80))
	if strings.Contains(clean, "You") || strings.Contains(clean, "▐") {
		t.Fatalf("user message should not render a label or left rail:\n%s", clean)
	}
	if !strings.Contains(clean, "› Please update the README.") {
		t.Fatalf("user message should render with prompt marker:\n%s", clean)
	}
	if w := lipgloss.Width(clean); w != 80 {
		t.Fatalf("user message background should span the full viewport, width=%d:\n%s", w, clean)
	}
}

func TestUserMessageContinuationAlignsWithPromptText(t *testing.T) {
	sty := styles.DefaultStyles()
	m := NewUserMessageItem(&sty, "first line\n\nsecond line")

	clean := ansi.Strip(m.Render(80))
	if !strings.Contains(clean, "\n   second line") {
		t.Fatalf("user continuation line should align after the prompt marker:\n%s", clean)
	}
}
