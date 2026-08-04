// message_context_glyphs.go — /context 报告的条形字形上色。
package message

import (
	"strings"

	"nekocode/interaction/tui/styles"

	"charm.land/lipgloss/v2"
)

// contextGlyphStyles maps the context-report bar glyphs to palette colors.
// These glyphs are emitted only by the /context report, so coloring them at
// the presentation layer is unambiguous and keeps the command output plain
// (IM surfaces receive the same text without ANSI).
var contextGlyphStyles = map[rune]lipgloss.Style{
	'⛁': lipgloss.NewStyle().Foreground(lipgloss.Color(styles.Blue)),      // system / tools / messages
	'⛀': lipgloss.NewStyle().Foreground(lipgloss.Color(styles.Yellow)),    // todo / skills
	'⛶': lipgloss.NewStyle().Foreground(lipgloss.Color(styles.DiffGreen)), // free
	'⛂': lipgloss.NewStyle().Foreground(lipgloss.Color(styles.Primary)),   // cache
	'⛃': lipgloss.NewStyle().Foreground(lipgloss.Color(styles.SubColors[4])), // subagents
}

const contextGlyphs = "⛁⛀⛶⛂⛃"

// colorizeContextGlyphs wraps the context-report bar glyphs in color
// styles; text without those glyphs passes through untouched. lipgloss is
// ANSI-aware for width math, so the styled glyphs do not break wrapping.
func colorizeContextGlyphs(s string) string {
	if !strings.ContainsAny(s, contextGlyphs) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 64)
	for _, r := range s {
		if sty, ok := contextGlyphStyles[r]; ok {
			b.WriteString(sty.Render(string(r)))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
