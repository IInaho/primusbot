// message_system.go — SystemMessageItem：系统消息渲染（蓝色左侧竖条）。
package message

import (
	"strings"

	"nekocode/interaction/tui/styles"

	"charm.land/lipgloss/v2"
)

type SystemMessageItem struct {
	content         string
	renderedContent string
	title           string
	sty             *styles.Styles
	cache           cachedRender
}

func NewSystemMessageItem(sty *styles.Styles, content string) *SystemMessageItem {
	return &SystemMessageItem{content: content, sty: sty}
}

func (m *SystemMessageItem) SetTitle(title string) {
	m.title = title
	m.cache = cachedRender{}
}

func (m *SystemMessageItem) SetRenderedContent(content string) {
	m.renderedContent = content
	m.cache = cachedRender{}
}

func (m *SystemMessageItem) Render(width int) string {
	cw := fullMessageWidth(width)
	if m.cache.width == cw && m.cache.rendered != "" {
		return m.cache.rendered
	}
	contentW := cw - barOverhead
	content := m.renderedContent
	if content == "" {
		content = RenderMarkdown(strings.TrimSpace(m.content), contentW)
	}
	header := m.sty.Blue.Bold(true).Render("▸")
	if m.title != "" {
		header += " " + m.sty.Blue.Bold(true).Render(m.title)
	}
	parts := []string{header, content}
	joined := lipgloss.JoinVertical(lipgloss.Left, parts...)
	out := thickLeftBar(stripLeadingSpaces(strings.TrimSpace(joined)), lipgloss.Color("#7a8ba0"), cw)
	m.cache.rendered = out
	m.cache.width = cw
	m.cache.height = strings.Count(out, "\n") + 1
	return out
}

func (m *SystemMessageItem) Height(width int) int {
	cw := fullMessageWidth(width)
	if m.cache.height > 0 && m.cache.width == cw {
		return m.cache.height
	}
	return strings.Count(m.content, "\n") + 2
}
