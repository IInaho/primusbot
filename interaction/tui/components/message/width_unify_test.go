package message

import (
	"testing"

	"nekocode/interaction/tui/styles"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestAllMessageTypesShareWidth(t *testing.T) {
	sty := styles.DefaultStyles()
	content := "hello world this is a test message with some text"

	cases := []struct {
		name   string
		render func(int) string
	}{
		{"user", func(w int) string { return NewUserMessageItem(&sty, content).Render(w) }},
		{"assistant", func(w int) string { return NewAssistantMessageItem(&sty, content).Render(w) }},
		{"system", func(w int) string { return NewSystemMessageItem(&sty, content).Render(w) }},
		{"error", func(w int) string { return NewErrorMessageItem(&sty, content).Render(w) }},
	}

	widths := []int{60, 80, 120, 200}
	for _, w := range widths {
		expected := styles.MessageWidth(w)
		for _, c := range cases {
			clean := ansi.Strip(c.render(w))
			got := 0
			for _, line := range splitLines(clean) {
				if lw := lipgloss.Width(line); lw > got {
					got = lw
				}
			}
			if got != expected {
				t.Fatalf("width=%d %s: outer width %d, want %d\n%s", w, c.name, got, expected, clean)
			}
		}
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
