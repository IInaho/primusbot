package taskview

import (
	"fmt"
	"strings"

	"nekocode/interaction/connect"
	controlruntime "nekocode/runtime"
)

// ApprovalMessage renders the pending-approval push (tool summary); the
// decision itself travels on the inline keyboard built from intent actions.
func ApprovalMessage(p controlruntime.ApprovalView) string {
	return compactMessage(htmlTitle("需要审批"), approvalSummary(p))
}

// QuestionMessage renders the pending-question push; free-form and
// multi-part questions include the /answer and /dismiss instructions.
func QuestionMessage(p controlruntime.QuestionView) string {
	return compactMessage(htmlTitle("提问"), questionSummary(p))
}

// StoppedMessage is the push text for a cancelled run.
func StoppedMessage() string {
	return htmlTitle("已停止")
}

func (t *Tracker) doneReplyLocked(card *taskCard) string {
	if card.Status == statusFailed {
		lines := []string{htmlTitle("失败")}
		if card.Error != "" {
			lines = append(lines, "", htmlTitle("错误"), htmlPre(connect.TruncateRunes(card.Error, 1400)))
		}
		if strings.TrimSpace(card.Result) != "" {
			lines = append(lines, "", htmlTitle("结果"), markdownBody(card.Result, 1600))
		}
		return compactMessage(lines...)
	}

	lines := make([]string, 0, 3)
	if strings.TrimSpace(card.Result) != "" {
		lines = append(lines, markdownBody(card.Result, 1800))
	}
	lines = appendDiffShortcut(lines, card, true)
	return compactMessage(lines...)
}

func (t *Tracker) lastSummaryLocked(card *taskCard) string {
	title := "完成"
	if card.Status == statusFailed {
		title = "失败"
	}
	lines := []string{htmlTitle(title)}
	lines = append(lines, HTMLEscape(compactCounts(card)))
	if card.Error != "" {
		lines = append(lines, "", htmlTitle("错误"), htmlPre(connect.TruncateRunes(card.Error, 1400)))
	}
	if strings.TrimSpace(card.Result) != "" {
		lines = append(lines, "", htmlTitle("结果"), markdownBody(card.Result, 1600))
	}
	lines = appendDiffShortcut(lines, card, true)
	return compactMessage(lines...)
}

func (t *Tracker) statusSummaryLocked(card *taskCard) string {
	lines := []string{htmlTitle("状态"), HTMLEscape(statusTitle(card.Status))}
	lines = appendTaskMeta(lines, card)
	if card.Status == statusWaitingApproval {
		lines = append(lines, HTMLEscape("等待审批"))
	}
	if card.Status == statusWaitingQuestion {
		lines = append(lines, HTMLEscape("等待输入"))
	}
	lines = appendDiffShortcut(lines, card, false)
	return compactMessage(lines...)
}

func (t *Tracker) cardStatusLocked(card *taskCard) string {
	lines := []string{htmlTitle(statusTitle(card.Status))}
	lines = appendTaskMeta(lines, card)
	if len(card.Tools) > 0 {
		start := len(card.Tools) - 5
		if start < 0 {
			start = 0
		}
		lines = append(lines, "", htmlTitle("工具"))
		for _, tool := range card.Tools[start:] {
			lines = append(lines, "- "+HTMLEscape(toolLine(tool)))
		}
	}
	lines = appendDiffShortcut(lines, card, len(card.Tools) > 0)
	return compactMessage(lines...)
}

func appendTaskMeta(lines []string, card *taskCard) []string {
	if card.Title != "" {
		lines = append(lines, labelText("任务", card.Title))
	}
	if card.LastPhase != "" {
		lines = append(lines, labelText("阶段", card.LastPhase))
	}
	return append(lines, HTMLEscape(compactCounts(card)))
}

func appendDiffShortcut(lines []string, card *taskCard, leadingBlank bool) []string {
	if len(card.Diffs) == 0 {
		return lines
	}
	if leadingBlank {
		lines = append(lines, "")
	}
	return append(lines, labelCode("差异", "/diff"))
}

func statusTitle(status cardStatus) string {
	switch status {
	case statusCreated, statusAccepted:
		return "排队中"
	case statusRunning:
		return "执行中"
	case statusWaitingApproval:
		return "等待审批"
	case statusWaitingQuestion:
		return "等待你的输入"
	case statusDone:
		return "已完成"
	case statusFailed:
		return "已失败"
	case statusAborted:
		return "已停止"
	default:
		return "状态"
	}
}

func compactCounts(card *taskCard) string {
	parts := []string{fmt.Sprintf("步骤: %d", len(card.Tools))}
	if len(card.Diffs) > 0 {
		parts = append(parts, fmt.Sprintf("差异: %d", len(card.Diffs)))
	}
	return strings.Join(parts, " | ")
}

func approvalSummary(p controlruntime.ApprovalView) string {
	var b strings.Builder
	b.WriteString(labelCode("工具", p.ToolName))
	if p.Kind != "" {
		fmt.Fprintf(&b, "\n%s", labelText("类型", p.Kind))
	}
	if cmd, ok := stringArg(p.Args, "command"); ok && cmd != "" {
		fmt.Fprintf(&b, "\n%s", labelText("命令", ""))
		fmt.Fprintf(&b, "\n%s", htmlPre(connect.TruncateRunes(cmd, 900)))
		return b.String()
	}
	if path, ok := stringArg(p.Args, "path"); ok && path != "" {
		fmt.Fprintf(&b, "\n%s", labelCode("路径", path))
	}
	if summary, ok := stringArg(p.Args, "summary"); ok && summary != "" {
		fmt.Fprintf(&b, "\n%s", HTMLEscape(connect.TruncateRunes(summary, 900)))
	}
	if preview, ok := stringArg(p.Args, "_preview"); ok && preview != "" {
		fmt.Fprintf(&b, "\n%s", labelText("预览", ""))
		fmt.Fprintf(&b, "\n%s", htmlPre(connect.TruncateRunes(preview, 1600)))
	}
	return b.String()
}

func Help() string {
	return strings.Join([]string{
		htmlTitle("命令"),
		labelCode("状态", "/status"),
		labelCode("最近", "/last"),
		labelCode("差异", "/diff"),
		labelCode("停止", "/stop"),
		labelCode("永久允许", "/always <id>"),
		"",
		HTMLEscape("审批和单选问题可直接点击按钮。"),
	}, "\n")
}

func toolLine(tool toolCard) string {
	brief := strings.TrimSpace(tool.Brief)
	if brief == "" {
		return strings.TrimSpace(tool.Name + " " + tool.Status)
	}
	return strings.TrimSpace(tool.Name + " " + tool.Status + " · " + brief)
}

func isDiffLike(toolName, preview string) bool {
	if toolName == "edit" || toolName == "write" || toolName == "diff" {
		return true
	}
	return strings.Contains(preview, "\n---") && strings.Contains(preview, "\n+++")
}
