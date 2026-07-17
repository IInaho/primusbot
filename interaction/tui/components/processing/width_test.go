package processing

import (
	"testing"

	"nekocode/interaction/tui/styles"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestProcessingWidthMatchesMessageWidth(t *testing.T) {
	sty := styles.DefaultStyles()
	p := NewProcessingItem(&sty)
	p.SetStatusText("Thinking")
	p.AppendStreamText("some streaming output text here")

	for _, w := range []int{60, 80, 120, 200} {
		expected := styles.MessageWidth(w)
		clean := ansi.Strip(p.Render(w))
		got := 0
		for _, line := range splitLinesProc(clean) {
			if lw := lipgloss.Width(line); lw > got {
				got = lw
			}
		}
		if got != expected {
			t.Fatalf("width=%d: processing outer width %d, want %d\n%s", w, got, expected, clean)
		}
	}
}

func splitLinesProc(s string) []string {
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
