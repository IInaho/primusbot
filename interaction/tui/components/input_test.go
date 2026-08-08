package components

import (
	"strings"
	"testing"

	"nekocode/interaction/tui/styles"

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

func TestInputFooterShowsModelReasoningEffort(t *testing.T) {
	in := NewInput(120)
	in.SetPermissionMode("manual")
	in.SetModel("deepseek/deepseek-v4-flash")
	in.SetReasoningEffort("medium")
	clean := ansi.Strip(in.View())
	want := "Follow: Auto · Perm: Manual · Effort: medium · Model: deepseek/deepseek-v4-flash"
	if !strings.Contains(clean, want) {
		t.Fatalf("footer = %q, want %q", clean, want)
	}
	if !strings.Contains(in.View(), styles.TealStyle.Render("Manual")) || !strings.Contains(in.View(), styles.YellowStyle.Render("medium")) {
		t.Fatalf("footer semantic colors were not applied:\n%s", in.View())
	}
}

func TestInputFooterShowsAutoForUnsetEffortAndFitsNarrowWidth(t *testing.T) {
	in := NewInput(42)
	in.SetModel("deepseek/deepseek-v4-flash")
	view := in.View()
	if !strings.Contains(ansi.Strip(view), "E:Auto") {
		t.Fatalf("unset effort was not shown as Auto:\n%s", ansi.Strip(view))
	}
	for _, line := range strings.Split(view, "\n") {
		if width := ansi.StringWidth(line); width > 42 {
			t.Fatalf("footer line width = %d, want <= 42:\n%s", width, ansi.Strip(view))
		}
	}
}
