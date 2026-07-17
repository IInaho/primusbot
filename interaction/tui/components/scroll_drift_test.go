package components

import (
	"strings"
	"testing"
)

type fakeItem struct{ height int }

func (f fakeItem) Render(width int) string { return strings.Repeat("x\n", f.height-1) + "x" }
func (f fakeItem) Height(width int) int    { return f.height }

func newScrollList(items []int, gap int) *List {
	l := NewList()
	l.SetGap(gap)
	for _, h := range items {
		l.AppendItems(fakeItem{height: h})
	}
	return l
}

func TestScrollRoundTripBottomStaysStable(t *testing.T) {
	// Two 5-line items with a gap of 1 in a 3-line viewport. The classic
	// configuration that triggered the offsetLine/pixelsAbove drift.
	l := newScrollList([]int{5, 5}, 1)
	l.SetSize(80, 3)
	l.ScrollToBottom()
	wantBottom := l.pixelsAbove
	wantOffsetLine := l.offsetLine
	wantScrollPercent := l.ScrollPercent()
	t.Logf("initial bottom: pixelsAbove=%d offsetIdx=%d offsetLine=%d percent=%v",
		wantBottom, l.offsetIdx, wantOffsetLine, wantScrollPercent)
	if !l.AtBottom() {
		t.Fatalf("expected to start at bottom")
	}

	// Scroll all the way up, then back down several times. The bottom must
	// land on exactly the same offsets every round trip.
	for round := 0; round < 5; round++ {
		for !atTop(l) {
			l.ScrollBy(-3)
			if l.offsetIdx < 0 {
				l.ScrollToTop()
			}
		}
		if !atTop(l) {
			t.Fatalf("round %d: failed to reach top, pixelsAbove=%d", round, l.pixelsAbove)
		}
		// Now scroll back to the bottom in increments that overshoot the gap
		// so the clamp path is exercised — this is what used to leave
		// offsetLine pointing one line past the real bottom.
		for !l.AtBottom() {
			l.ScrollBy(3)
		}
		if l.pixelsAbove != wantBottom {
			t.Fatalf("round %d: pixelsAbove drifted to %d (want %d)", round, l.pixelsAbove, wantBottom)
		}
		if l.offsetLine != wantOffsetLine {
			t.Fatalf("round %d: offsetLine drifted to %d (want %d)", round, l.offsetLine, wantOffsetLine)
		}
		if p := l.ScrollPercent(); p != wantScrollPercent {
			t.Fatalf("round %d: scroll percent drifted to %v (want %v)", round, p, wantScrollPercent)
		}
	}
}

func atTop(l *List) bool { return l.offsetIdx == 0 && l.offsetLine == 0 }

func TestScrollNegativeClampSyncsToTop(t *testing.T) {
	// A single tall item — scrolling up past the start must put us at the
	// true top, both in pixelsAbove and offsetIdx/offsetLine.
	l := newScrollList([]int{20}, 1)
	l.SetSize(80, 5)
	l.ScrollBy(-100)
	if !atTop(l) {
		t.Fatalf("expected top, got pixelsAbove=%d offsetIdx=%d offsetLine=%d",
			l.pixelsAbove, l.offsetIdx, l.offsetLine)
	}
	if l.pixelsAbove != 0 || l.offsetIdx != 0 || l.offsetLine != 0 {
		t.Fatalf("top state out of sync: %d/%d/%d",
			l.pixelsAbove, l.offsetIdx, l.offsetLine)
	}
}
