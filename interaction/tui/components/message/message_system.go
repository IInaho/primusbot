// message_system.go — SystemMessageItem：系统消息渲染（灰色圆点 + 缩进，与对话块格式统一）。
package message

import (
	"strings"

	"nekocode/interaction/tui/styles"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
	contentW := max(cw-4, 10)
	content := m.renderedContent
	if content == "" {
		content = RenderMarkdown(strings.TrimSpace(m.content), contentW)
	}
	content = colorizeContextGlyphs(content)
	if m.title != "" {
		content = m.title + "\n" + content
	}
	sepW := cw
	separator := m.sty.Border.Render(strings.Repeat("─", sepW))
	body := lipgloss.NewStyle().Width(cw).MaxWidth(cw).Render(renderSystemBody(content, m.sty))
	out := separator + "\n" + body
	m.cache.rendered = out
	m.cache.width = cw
	m.cache.height = strings.Count(out, "\n") + 1
	return out
}

// renderSystemBody: 灰色圆点 + 缩进, 与 assistant 正文 (•) 格式统一, 仅颜色不同。
func renderSystemBody(body string, sty *styles.Styles) string {
	body = stripLeadingSpaces(body)
	lines := strings.Split(body, "\n")
	prefix := "  " + sty.Muted.Render("•") + " "
	continuation := "    "

	var out strings.Builder
	bulletWritten := false
	for _, line := range lines {
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		if strings.TrimSpace(ansi.Strip(line)) == "" {
			continue
		}
		if !bulletWritten {
			out.WriteString(prefix)
			bulletWritten = true
		} else {
			out.WriteString(continuation)
		}
		out.WriteString(line)
	}
	return strings.TrimRight(out.String(), "\n")
}

func (m *SystemMessageItem) Height(width int) int {
	cw := fullMessageWidth(width)
	if m.cache.height > 0 && m.cache.width == cw {
		return m.cache.height
	}
	return strings.Count(m.content, "\n") + 2
}
