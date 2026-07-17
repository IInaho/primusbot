// scrollbar.go — 垂直滚动条指示器。
package components

import (
	"strings"

	"nekocode/interaction/tui/styles"

	"charm.land/lipgloss/v2"
)

// Scrollbar renders a vertical scroll indicator as an independent component.
// When content fits in the viewport it returns an empty string.
type Scrollbar struct {
	totalHeight    int
	viewportHeight int
	scrollPercent  float64
	sty            *styles.Styles

	cachedView  string
	cachedDirty bool
}

func NewScrollbar(sty *styles.Styles) *Scrollbar {
	return &Scrollbar{sty: sty, cachedDirty: true}
}

func (s *Scrollbar) Update(totalHeight, viewportHeight int, scrollPercent float64) {
	if s.totalHeight == totalHeight && s.viewportHeight == viewportHeight && s.scrollPercent == scrollPercent {
		return
	}
	s.totalHeight = totalHeight
	s.viewportHeight = viewportHeight
	s.scrollPercent = scrollPercent
	s.cachedDirty = true
}

func (s *Scrollbar) View() string {
	if s.totalHeight <= s.viewportHeight || s.viewportHeight <= 0 {
		return ""
	}

	if !s.cachedDirty {
		return s.cachedView
	}

	thumbSize := max(1, s.viewportHeight*s.viewportHeight/s.totalHeight)
	thumbPos := 0
	trackSpace := s.viewportHeight - thumbSize
	if trackSpace > 0 {
		thumbPos = min(trackSpace, int(float64(trackSpace)*s.scrollPercent))
	}

	var sb strings.Builder
	for i := 0; i < s.viewportHeight; i++ {
		if i > 0 {
			sb.WriteString("\n")
		}
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString(s.sty.Scrollbar.Thumb.Render(styles.HeavyVert))
		} else {
			sb.WriteString(s.sty.Scrollbar.Track.Render(styles.Vertical))
		}
	}

	s.cachedView = lipgloss.NewStyle().Width(1).Render(sb.String())
	s.cachedDirty = false
	return s.cachedView
}

// ViewHorizontal renders a single-line horizontal scroll indicator across the
// given width. A thumb segment sized proportionally to viewport/total sits at
// a position proportional to scrollPercent; the rest is track. Returns "" when
// content fits the viewport (no scrolling needed).
func (s *Scrollbar) ViewHorizontal(width int) string {
	if s.totalHeight <= s.viewportHeight || s.viewportHeight <= 0 || width <= 0 {
		return ""
	}
	thumbSize := max(1, width*s.viewportHeight/s.totalHeight)
	if thumbSize > width {
		thumbSize = width
	}
	trackSpace := width - thumbSize
	thumbPos := 0
	if trackSpace > 0 {
		thumbPos = min(trackSpace, int(float64(trackSpace)*s.scrollPercent))
	}

	// Slim thumb on a thin rule. Track color matches the header bottom border
	// (#333333) so the bar reads as an extension of the header line; thumb
	// uses the teal accent from the "深夜书房" palette.
	trackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))
	thumbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4ec9b0"))

	left := trackStyle.Render(strings.Repeat("─", thumbPos))
	thumb := thumbStyle.Render(strings.Repeat("─", thumbSize))
	right := trackStyle.Render(strings.Repeat("─", trackSpace-thumbPos))
	return left + thumb + right
}
