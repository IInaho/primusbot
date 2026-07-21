package compression

import (
	"strconv"
	"strings"

	"nekocode/common/debug"
)

// DefaultBudget is the fallback context window size when not configured.
const DefaultBudget = 64000

const (
	grepHeadLines = 50
	grepTailLines = 50
)

func BudgetResult(content string, toolName string) (string, bool) {
	if toolName != "grep" {
		return content, false
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= grepHeadLines+grepTailLines {
		return content, false
	}
	head := lines[:grepHeadLines]
	tail := lines[len(lines)-grepTailLines:]
	truncated := strings.Join(head, "\n") + "\n... [" +
		strconv.Itoa(len(lines)-grepHeadLines-grepTailLines) + " lines truncated] ...\n" +
		strings.Join(tail, "\n")
	debug.Log("budget_result: truncated grep from %d lines (%d chars) to head=%d tail=%d",
		len(lines), len(content), grepHeadLines, grepTailLines)
	return truncated, true
}
