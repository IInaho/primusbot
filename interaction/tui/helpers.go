// helpers.go — TUI 统计文案。
package tui

import (
	controlruntime "nekocode/runtime"
	"nekocode/util/text"
)

func tokensSummary(metrics controlruntime.MetricsSnapshot) string {
	return "↑" + text.FormatTokens(metrics.TurnPrompt) + " ↓" + text.FormatTokens(metrics.TurnCompletion)
}
