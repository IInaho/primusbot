package connect

import (
	"fmt"
	"strings"

	controlruntime "nekocode/runtime"
)

// ApprovalText is the canonical plain-text rendering of a pending approval,
// used directly by text-only channels and as the fallback when a rich card
// cannot be delivered.
func ApprovalText(p controlruntime.ApprovalView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "需要审批: %s", p.ToolName)
	if cmd, ok := p.Args["command"].(string); ok && cmd != "" {
		fmt.Fprintf(&b, "\n%s", TruncateRunes(cmd, 600))
	} else if path, ok := p.Args["path"].(string); ok && path != "" {
		fmt.Fprintf(&b, "\n%s", path)
	}
	fmt.Fprintf(&b, "\n回复 /approve %s 批准一次,/always %s 永久允许,/reject %s 拒绝", p.ID, p.ID, p.ID)
	return b.String()
}

// QuestionText is the canonical plain-text rendering of a pending question,
// with the /answer slash command as the reply path.
func QuestionText(p controlruntime.QuestionView) string {
	var b strings.Builder
	b.WriteString("NekoCode 提问:")
	for _, q := range p.Questions {
		fmt.Fprintf(&b, "\n- %s", q.Question)
		for _, opt := range q.Options {
			fmt.Fprintf(&b, "\n    · %s", opt.Label)
		}
	}
	b.WriteString("\n回复 /answer <回答内容> 作答")
	return b.String()
}

// TruncateRunes shortens s to at most n runes, appending an ellipsis when
// truncated.
func TruncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// SharedHelp renders the help text for the commands every channel supports
// (shared slash commands plus /answer). extras are appended verbatim for
// channel-specific commands and usage notes.
func SharedHelp(extras ...string) string {
	lines := []string{
		"Commands:",
		"  /stop          停止当前任务",
		"  /approve <id>  批准一次工具调用",
		"  /always <id>   批准并永久允许",
		"  /reject <id>   拒绝工具调用",
		"  /answer <内容> 回答进行中的问题",
		"  /dismiss [id]  忽略进行中的问题",
		"  /help          显示帮助",
	}
	return strings.Join(append(lines, extras...), "\n")
}

// StatusView builds the baseline connector status view (running/stopped)
// so channels only fill in configuration, metadata, and devices.
func StatusView(name string, running bool) controlruntime.ConnectorView {
	status := "stopped"
	if running {
		status = "running"
	}
	return controlruntime.ConnectorView{
		Name:        name,
		Registered:  true,
		Initialized: true,
		Running:     running,
		Status:      status,
		Metadata:    make(map[string]any),
	}
}
