// message_user.go — UserMessageItem：用户消息渲染。
package message

import (
	"strings"

	"nekocode/interaction/tui/styles"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type UserMessageItem struct {
	content string
	sty     *styles.Styles
	cache   cachedRender
}

func NewUserMessageItem(sty *styles.Styles, content string) *UserMessageItem {
	return &UserMessageItem{content: content, sty: sty}
}

func (m *UserMessageItem) Render(width int) string {
	cw := fullMessageWidth(width)
	if m.cache.width == cw && m.cache.rendered != "" {
		return m.cache.rendered
	}
	bg := lipgloss.Color("#242424")
	bgOn := strings.TrimSuffix(lipgloss.NewStyle().Background(bg).Render(""), "\x1b[m")
	prefix := m.sty.Yellow.Bold(true).Background(bg).Render(" ›") + bgOn
	contentW := max(cw-4, 10)
	body := strings.TrimSpace(ansi.Strip(RenderMarkdown(strings.TrimSpace(m.content), contentW)))
	body = prefixUserBody(body, prefix)
	block := lipgloss.NewStyle().
		Width(cw).
		MaxWidth(cw).
		Padding(1, 1).
		Background(bg).
		Render(body)
	out := strings.TrimRight(block, "\n")
	m.cache.rendered = out
	m.cache.width = cw
	m.cache.height = strings.Count(out, "\n") + 1
	return out
}

func (m *UserMessageItem) Height(width int) int {
	cw := fullMessageWidth(width)
	if m.cache.height > 0 && m.cache.width == cw {
		return m.cache.height
	}
	lines := strings.Count(m.content, "\n") + 1
	return lines + 2
}

func prefixUserBody(body, prefix string) string {
	body = stripLeadingSpaces(body)
	lines := strings.Split(body, "\n")
	var out strings.Builder
	first := true
	for _, line := range lines {
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		if first {
			out.WriteString(prefix)
			out.WriteByte(' ')
			first = false
		} else {
			out.WriteString("  ")
		}
		out.WriteString(line)
	}
	return out.String()
}
