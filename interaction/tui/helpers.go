// helpers.go — 统计等格式化辅助函数；工具参数简报见 common/view/toolbrief.go。
package tui

import (
	controlruntime "nekocode/runtime"
	"nekocode/util/text"
)

func tokensSummary(stats controlruntime.BotStats) string {
	st := stats
	return "↑" + text.FormatTokens(st.TurnPrompt) + " ↓" + text.FormatTokens(st.TurnCompletion)
}
