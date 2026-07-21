package taskview

import (
	"fmt"
	"strings"

	controlruntime "nekocode/runtime"
)

func renderDiffPreview(preview string) string {
	path := diffPath(preview)
	clean := cleanDiffPreview(preview)
	if strings.TrimSpace(clean) == "" {
		return ""
	}
	if path == "" {
		path = diffPath(clean)
	}
	add, del := diffLineCounts(clean)
	title := "Diff"
	var meta []string
	if path != "" {
		meta = append(meta, path)
	}
	if add > 0 || del > 0 {
		meta = append(meta, fmt.Sprintf("+%d -%d", add, del))
	}
	if len(meta) > 0 {
		return compactMessage(htmlTitle(title), htmlCode(strings.Join(meta, "  ")), htmlPre(truncateRunes(clean, 2600)))
	}
	return compactMessage(htmlTitle(title), htmlPre(truncateRunes(clean, 2600)))
}

func (t *Tracker) doneReplyLocked(card *taskCard) string {
	if card.Status == statusFailed {
		lines := []string{htmlTitle("Failed")}
		if card.Error != "" {
			lines = append(lines, "", htmlTitle("Error"), htmlPre(truncateRunes(card.Error, 1400)))
		}
		if strings.TrimSpace(card.Result) != "" {
			lines = append(lines, "", htmlTitle("Result"), htmlBody(card.Result, 1600))
		}
		return compactMessage(lines...)
	}

	lines := make([]string, 0, 3)
	if strings.TrimSpace(card.Result) != "" {
		lines = append(lines, htmlBody(card.Result, 1800))
	}
	lines = appendDiffShortcut(lines, card, true)
	return compactMessage(lines...)
}

func (t *Tracker) lastSummaryLocked(card *taskCard) string {
	title := "Done"
	if card.Status == statusFailed {
		title = "Failed"
	}
	lines := []string{htmlTitle(title)}
	lines = append(lines, HTMLEscape(compactCounts(card)))
	if card.Error != "" {
		lines = append(lines, "", htmlTitle("Error"), htmlPre(truncateRunes(card.Error, 1400)))
	}
	if strings.TrimSpace(card.Result) != "" {
		lines = append(lines, "", htmlTitle("Result"), htmlBody(card.Result, 1600))
	}
	lines = appendDiffShortcut(lines, card, true)
	return compactMessage(lines...)
}

func (t *Tracker) statusSummaryLocked(card *taskCard) string {
	lines := []string{htmlTitle("Status"), HTMLEscape(statusTitle(card.Status))}
	lines = appendTaskMeta(lines, card)
	if card.Status == statusWaitingApproval {
		lines = append(lines, HTMLEscape("Waiting for approval"))
	}
	if card.Status == statusWaitingQuestion {
		lines = append(lines, HTMLEscape("Waiting for input"))
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
		lines = append(lines, "", htmlTitle("Tools"))
		for _, tool := range card.Tools[start:] {
			lines = append(lines, "- "+HTMLEscape(toolLine(tool)))
		}
	}
	lines = appendDiffShortcut(lines, card, len(card.Tools) > 0)
	return compactMessage(lines...)
}

func appendTaskMeta(lines []string, card *taskCard) []string {
	if card.Title != "" {
		lines = append(lines, labelText("Task", card.Title))
	}
	if card.LastPhase != "" {
		lines = append(lines, labelText("Phase", card.LastPhase))
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
	return append(lines, labelCode("Diff", "/diff"))
}

func statusTitle(status cardStatus) string {
	switch status {
	case statusCreated, statusAccepted:
		return "Queued"
	case statusRunning:
		return "Working"
	case statusWaitingApproval:
		return "Waiting for approval"
	case statusWaitingQuestion:
		return "Waiting for your input"
	case statusDone:
		return "Done"
	case statusFailed:
		return "Failed"
	case statusAborted:
		return "Stopped"
	default:
		return "Status"
	}
}

func compactCounts(card *taskCard) string {
	parts := []string{fmt.Sprintf("Steps: %d", len(card.Tools))}
	if len(card.Diffs) > 0 {
		parts = append(parts, fmt.Sprintf("Diff: %d", len(card.Diffs)))
	}
	return strings.Join(parts, " | ")
}

func approvalSummary(p controlruntime.ApprovalView) string {
	var b strings.Builder
	b.WriteString(labelCode("Tool", p.ToolName))
	if p.Kind != "" {
		fmt.Fprintf(&b, "\n%s", labelText("Kind", p.Kind))
	}
	if cmd, ok := stringArg(p.Args, "command"); ok && cmd != "" {
		fmt.Fprintf(&b, "\n%s", labelText("Command", ""))
		fmt.Fprintf(&b, "\n%s", htmlPre(truncateRunes(cmd, 900)))
		return b.String()
	}
	if path, ok := stringArg(p.Args, "path"); ok && path != "" {
		fmt.Fprintf(&b, "\n%s", labelCode("Path", path))
	}
	if summary, ok := stringArg(p.Args, "summary"); ok && summary != "" {
		fmt.Fprintf(&b, "\n%s", HTMLEscape(truncateRunes(summary, 900)))
	}
	if preview, ok := stringArg(p.Args, "_preview"); ok && preview != "" {
		fmt.Fprintf(&b, "\n%s", labelText("Preview", ""))
		fmt.Fprintf(&b, "\n%s", htmlPre(truncateRunes(preview, 1600)))
	}
	return b.String()
}

func Help() string {
	return strings.Join([]string{
		htmlTitle("Commands"),
		labelCode("Status", "/status"),
		labelCode("Last", "/last"),
		labelCode("Diff", "/diff"),
		labelCode("Stop", "/stop"),
		"",
		HTMLEscape("Use buttons for approvals and single-choice questions."),
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
