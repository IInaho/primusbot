// message_error.go — ErrorMessageItem：错误消息渲染（红色左侧竖条）。
package message

import (
	"strings"

	"nekocode/interaction/tui/styles"

	"charm.land/lipgloss/v2"
)

type ErrorMessageItem struct {
	content string
	sty     *styles.Styles
	cache   cachedRender
}

func NewErrorMessageItem(sty *styles.Styles, content string) *ErrorMessageItem {
	return &ErrorMessageItem{content: content, sty: sty}
}

func (m *ErrorMessageItem) Render(width int) string {
	cw := fullMessageWidth(width)
	if m.cache.width == cw && m.cache.rendered != "" {
		return m.cache.rendered
	}
	contentW := cw - barOverhead
	body := strings.TrimSpace(RenderMarkdown(strings.TrimSpace(m.content), contentW))
	header := m.sty.Red.Bold(true).Render("!")
	parts := []string{header, "", body}
	joined := lipgloss.JoinVertical(lipgloss.Left, parts...)
	out := thickLeftBar(stripLeadingSpaces(strings.TrimSpace(joined)), lipgloss.Color("#e06c75"), cw)
	m.cache.rendered = out
	m.cache.width = cw
	m.cache.height = strings.Count(out, "\n") + 1
	return out
}

func (m *ErrorMessageItem) Height(width int) int {
	cw := fullMessageWidth(width)
	if m.cache.height > 0 && m.cache.width == cw {
		return m.cache.height
	}
	return strings.Count(m.content, "\n") + 3
}
