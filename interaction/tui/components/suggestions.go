// suggestions.go — input suggestions panel, shown below the input box.
package components

import (
	"fmt"
	"strings"

	"nekocode/interaction/tui/styles"
)

const maxVisibleSuggestions = 5

type Suggestions struct {
	items       []string
	selectedIdx int
	scrollOff   int // first visible item index
	visible     bool
	sty         *styles.Styles
}

func NewSuggestions(sty *styles.Styles) *Suggestions {
	return &Suggestions{sty: sty}
}

func (s *Suggestions) Refresh(prefix string, commands []string) {
	s.items = nil
	s.selectedIdx = 0
	s.scrollOff = 0
	s.visible = false

	commandPrefix := suggestionPrefix(prefix)
	if commandPrefix == "" {
		return
	}

	p := strings.TrimPrefix(prefix, commandPrefix)
	for _, name := range commands {
		display := commandDisplayName(name)
		if !strings.HasPrefix(display, commandPrefix) {
			continue
		}
		if strings.HasPrefix(strings.TrimPrefix(display, commandPrefix), p) {
			s.items = append(s.items, display)
		}
	}
	if len(s.items) == 1 && s.items[0] == prefix {
		return
	}
	if len(s.items) > 0 {
		s.visible = true
	}
}

func suggestionPrefix(value string) string {
	if strings.HasPrefix(value, "/") {
		return "/"
	}
	if strings.HasPrefix(value, "$") {
		return "$"
	}
	return ""
}

func commandDisplayName(name string) string {
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "$") {
		return name
	}
	return "/" + name
}

func (s *Suggestions) Accept() string {
	if !s.visible || len(s.items) == 0 {
		return ""
	}
	val := s.items[s.selectedIdx]
	s.visible = false
	return val
}

func (s *Suggestions) Cycle(delta int) {
	if !s.visible || len(s.items) == 0 {
		return
	}
	s.selectedIdx += delta
	if s.selectedIdx < 0 {
		s.selectedIdx = 0
	}
	if s.selectedIdx >= len(s.items) {
		s.selectedIdx = len(s.items) - 1
	}
	// Keep selected item visible.
	if s.selectedIdx < s.scrollOff {
		s.scrollOff = s.selectedIdx
	}
	if s.selectedIdx >= s.scrollOff+maxVisibleSuggestions {
		s.scrollOff = s.selectedIdx - maxVisibleSuggestions + 1
	}
}

func (s *Suggestions) Visible() bool { return s.visible }
func (s *Suggestions) Hide()         { s.visible = false; s.scrollOff = 0; s.selectedIdx = 0 }

func (s *Suggestions) Height() int {
	if !s.visible || len(s.items) == 0 {
		return 0
	}
	n := len(s.items)
	if n > maxVisibleSuggestions {
		n = maxVisibleSuggestions
	}
	return n + 1 // +1 for the header line
}

func (s *Suggestions) View(width int) string {
	if !s.visible || len(s.items) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(s.sty.Subtle.Render("── suggestions ──"))
	end := s.scrollOff + maxVisibleSuggestions
	hasMore := end < len(s.items)
	if end > len(s.items) {
		end = len(s.items)
	}
	for i := s.scrollOff; i < end; i++ {
		b.WriteByte('\n')
		if i == s.selectedIdx {
			fmt.Fprintf(&b, "%s", s.sty.Primary.Bold(true).Render("> "+s.items[i]))
		} else {
			fmt.Fprintf(&b, "%s", s.sty.Muted.Render("  "+s.items[i]))
		}
	}
	if s.scrollOff > 0 || hasMore {
		fmt.Fprintf(&b, "\n%s", s.sty.Subtle.Render("  ... more ..."))
	}
	return b.String()
}
