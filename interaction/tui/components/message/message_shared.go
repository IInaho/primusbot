// message_shared.go — 消息渲染共享 helper：缓存、宽度计算、左侧彩色竖条。
package message

import (
	"image/color"
	"strings"

	"nekocode/interaction/tui/styles"

	"charm.land/lipgloss/v2"
)

const (
	barOverhead = 3
)

var barBorder = lipgloss.Border{Left: "▐"}

type cachedRender struct {
	rendered string
	width    int
	height   int
}

func fullMessageWidth(available int) int {
	return styles.MessageWidth(available)
}

func stripLeadingSpaces(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimLeft(line, " ")
	}
	return strings.Join(lines, "\n")
}

func thickLeftBar(content string, barColor color.Color, width int) string {
	return lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(barBorder).
		BorderForeground(barColor).
		PaddingLeft(1).PaddingRight(1).
		Width(width).MaxWidth(width).
		Render(content)
}
