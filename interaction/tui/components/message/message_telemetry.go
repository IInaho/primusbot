package message

import (
	"fmt"
	"strings"

	"nekocode/interaction/tui/styles"
	"nekocode/protocol"
	"nekocode/util/text"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// TelemetryMessageItem renders a standalone turn footer, used when a failed
// run has no assistant message to attach the metrics to.
type TelemetryMessageItem struct {
	telemetry protocol.Metrics
	sty       *styles.Styles
}

func NewTelemetryMessageItem(sty *styles.Styles, telemetry protocol.Metrics) *TelemetryMessageItem {
	return &TelemetryMessageItem{telemetry: telemetry, sty: sty}
}

func (m *TelemetryMessageItem) Render(width int) string {
	available := max(fullMessageWidth(width)-2, 1)
	return "  " + renderTelemetryLine(m.telemetry, available, m.sty)
}

func (m *TelemetryMessageItem) Height(int) int { return 1 }

func renderTelemetryLine(telemetry protocol.Metrics, width int, sty *styles.Styles) string {
	variants := []string{
		renderFullTelemetry(telemetry, sty),
		renderCompactTelemetry(telemetry, sty),
		renderTinyTelemetry(telemetry, sty),
	}
	for _, variant := range variants {
		if lipgloss.Width(variant) <= width {
			return variant
		}
	}
	return ansi.Truncate(variants[len(variants)-1], max(width, 1), "")
}

func renderFullTelemetry(t protocol.Metrics, sty *styles.Styles) string {
	return renderDetailedTelemetry(t, sty, telemetryDetail{totalSuffix: " tok", showUnknownNew: true})
}

func renderCompactTelemetry(t protocol.Metrics, sty *styles.Styles) string {
	return renderDetailedTelemetry(t, sty, telemetryDetail{})
}

type telemetryDetail struct {
	totalSuffix    string
	showUnknownNew bool
}

func renderDetailedTelemetry(t protocol.Metrics, sty *styles.Styles, detail telemetryDetail) string {
	total := t.TurnTotal
	if total <= 0 {
		total = max(0, t.TurnInput) + max(0, t.TurnOutput)
	}
	parts := make([]string, 0, 9)
	if t.Duration != "" {
		parts = append(parts, sty.Subtle.Render(t.Duration))
	}
	parts = append(parts,
		telemetryMetric("总计", text.FormatTokens(total)+detail.totalSuffix, sty.Muted, sty),
		telemetryMetric("输入", text.FormatTokens(t.TurnInput), sty.Muted, sty),
	)
	if t.TurnCacheReported {
		parts = append(parts,
			telemetryMetric("缓存", text.FormatTokens(t.TurnCached), sty.Teal, sty),
			telemetryMetric("未缓存", text.FormatTokens(t.TurnNew), sty.Yellow, sty),
		)
	} else {
		parts = append(parts, telemetryMetric("缓存", "—", sty.Subtle, sty))
		if detail.showUnknownNew {
			parts = append(parts, telemetryMetric("未缓存", "—", sty.Subtle, sty))
		}
	}
	parts = append(parts, telemetryMetric("输出", text.FormatTokens(t.TurnOutput), sty.Blue, sty))
	if t.TurnReasoning > 0 {
		parts = append(parts, telemetryMetric("推理", text.FormatTokens(t.TurnReasoning), sty.Primary, sty))
	}
	parts = append(parts, telemetryMetric("本次命中", turnHitRatio(t), sty.Teal, sty))
	return sty.Subtle.Render("↳ ") + strings.Join(parts, sty.Subtle.Render(" · "))
}

func renderTinyTelemetry(t protocol.Metrics, sty *styles.Styles) string {
	parts := []string{sty.Subtle.Render("↳")}
	if t.TurnCacheReported {
		parts = append(parts,
			telemetryMetric("缓", text.FormatTokens(t.TurnCached), sty.Teal, sty),
			telemetryMetric("新", text.FormatTokens(t.TurnNew), sty.Yellow, sty),
		)
	} else {
		parts = append(parts, telemetryMetric("输入", text.FormatTokens(t.TurnInput), sty.Muted, sty), sty.Subtle.Render("缓存—"))
	}
	parts = append(parts, telemetryMetric("出", text.FormatTokens(t.TurnOutput), sty.Blue, sty))
	parts = append(parts, telemetryMetric("命中", turnHitRatio(t), sty.Teal, sty))
	return strings.Join(parts, " ")
}

func turnHitRatio(t protocol.Metrics) string {
	if !t.TurnCacheReported || t.TurnCached+t.TurnNew <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.2f%%", float64(t.TurnCached)*100/float64(t.TurnCached+t.TurnNew))
}

func telemetryMetric(label, value string, valueStyle lipgloss.Style, sty *styles.Styles) string {
	return sty.Subtle.Render(label+" ") + valueStyle.Render(value)
}
