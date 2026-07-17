package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestInputWrapsLongTextWithoutHidingPrefix(t *testing.T) {
	in := NewInput(32)
	initialHeight := in.Height()
	in.SetValue("alpha-beta-gamma-delta-epsilon-zeta-eta-theta-iota-kappa")
	in.SetCursorEnd()

	view := in.View()
	clean := ansi.Strip(view)
	if !strings.Contains(clean, "alpha-beta") {
		t.Fatalf("wrapped input should keep the prefix visible:\n%s", clean)
	}
	if !strings.Contains(clean, "kappa") {
		t.Fatalf("wrapped input should keep the suffix visible:\n%s", clean)
	}
	if strings.Contains(clean, "alpha-beta-gamma-delta-epsilon-zeta-eta-theta-iota-kappa") {
		t.Fatalf("input should wrap instead of staying on one line:\n%s", clean)
	}
	if in.Height() <= initialHeight {
		t.Fatalf("input height should grow for wrapped text, got %d <= %d", in.Height(), initialHeight)
	}
	for _, line := range strings.Split(view, "\n") {
		if w := ansi.StringWidth(line); w > 32 {
			t.Fatalf("input line width = %d, want <= 32:\n%s", w, clean)
		}
	}
}
