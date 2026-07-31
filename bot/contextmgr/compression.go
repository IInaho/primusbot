package contextmgr

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"nekocode/bot/provider"
	"nekocode/bot/provider/types"
	"nekocode/logger"
	"nekocode/util/text"
)

type Summarizer func([]types.Message, string) (string, error)

const (
	defaultBudget   = 64000
	clearedMarker   = "[Old tool result cleared]"
	grepHeadLines   = 50
	grepTailLines   = 50
	mergeMaxTokens  = 2000
	mergeFailureTag = "[Merge Failed - raw append]"
)

const summaryNoToolsPreamble = `CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.
- Do NOT use Read, Bash, Grep, Glob, Edit, Write, or ANY other tool.
- Tool calls will be REJECTED and will waste your only turn — you will fail the task.
`

func formatSummaryMessages(msgs []types.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		content := strings.TrimSpace(m.Content)
		if content == "" || content == "." || content == clearedMarker {
			continue
		}
		limit := 500
		if m.Role == "tool" {
			limit = 800
		}
		fmt.Fprintf(&b, "[%s]: %s\n", m.Role, text.TruncateByRune(content, limit))
	}
	return b.String()
}

func buildSummaryPrompt(msgs []types.Message, prevSummary string) string {
	conversation := formatSummaryMessages(msgs)

	template := summaryNoToolsPreamble + `
You are a context summarization assistant for coding sessions.
Summarize only the conversation history provided below.
If a previous summary exists, update it incrementally — add new information and remove superseded items.
Do NOT mention that you are summarizing or compacting context.

CRITICAL Preservation Rules:
- Code snippets: preserve FULL code for any file that was modified or is under discussion.
- Error messages: copy VERBATIM — do NOT paraphrase. Exact error text enables accurate future diagnosis.
- File paths: always include the exact path with line numbers when available (e.g., "bot/agent/run.go:212").
- User directives and constraints: preserve all user-specified rules, preferences, and prohibitions.

Previous summary (if any):
` + prevSummary + `

Conversation to summarize:
` + conversation + `

Output your response in the following format:

<analysis>
Organize your thoughts here. Identify the key themes, decisions, and outcomes.
This section is a scratchpad and will be stripped — write freely.
</analysis>

<summary>
The compressed summary text that will replace the original messages in context.
Write concisely but include ALL code snippets and error messages verbatim.
</summary>

<key-facts>
- Fact 1: one-line established fact about the project or environment
- Fact 2: another fact
Only include facts that are confirmed true and likely relevant to future turns. Limit 5 facts.
</key-facts>`

	return template
}

func formatCompactSummary(raw string) string {
	return extractXMLBlock(raw, "summary")
}

func extractXMLBlock(raw, tag string) string {
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	start := strings.Index(raw, openTag)
	if start < 0 {
		return ""
	}
	start += len(openTag)
	end := strings.Index(raw[start:], closeTag)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(raw[start : start+end])
}

func budgetToolResult(content, toolName string) (string, bool) {
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
	logger.Log("budget_result: truncated grep from %d lines (%d chars) to head=%d tail=%d",
		len(lines), len(content), grepHeadLines, grepTailLines)
	return truncated, true
}

type compactionLevel int

const (
	compactionNormal compactionLevel = iota
	compactionWarning
	compactionMicro
	compactionRequired
	compactionBlocking
)

type compressionConfig struct {
	warningBuffer  int
	microBuffer    int
	compactBuffer  int
	blockingBuffer int
}

var defaultCompressionConfig = compressionConfig{
	warningBuffer:  44800,
	microBuffer:    35200,
	compactBuffer:  25600,
	blockingBuffer: 6400,
}

func classifyCompaction(remaining int, config compressionConfig) compactionLevel {
	switch {
	case remaining <= config.blockingBuffer:
		return compactionBlocking
	case remaining <= config.compactBuffer:
		return compactionRequired
	case remaining <= config.microBuffer:
		return compactionMicro
	case remaining <= config.warningBuffer:
		return compactionWarning
	default:
		return compactionNormal
	}
}

func mergeSummaries(ctx context.Context, client provider.LLM, oldSummary, newSummary string) string {
	if oldSummary == "" {
		return newSummary
	}
	if newSummary == "" {
		return oldSummary
	}

	merged, err := tryMergeSummaries(ctx, client, oldSummary, newSummary)
	if err == nil {
		return merged
	}
	combined := oldSummary + "\n\n" + newSummary
	runes := []rune(combined)
	if len(runes) > mergeMaxTokens*4 {
		combined = string(runes[:mergeMaxTokens*4]) + "\n... (truncated)"
	}
	return fmt.Sprintf("%s\n\n---\n%s\nPrevious merge failed. Content preserved as-is. Async healing will clean up.",
		combined, mergeFailureTag)
}

func tryMergeSummaries(ctx context.Context, client provider.LLM, oldSummary, newSummary string) (string, error) {
	originalMaxTokens := client.GetMaxTokens()
	originalThinking := client.GetDisableThinking()
	client.SetMaxTokens(mergeMaxTokens)
	client.SetDisableThinking(true)
	defer func() {
		client.SetMaxTokens(originalMaxTokens)
		client.SetDisableThinking(originalThinking)
	}()

	var merged string
	err := provider.Retry(ctx, provider.DefaultRetryConfig, func() error {
		var err error
		merged, err = callSummaryMerge(ctx, client, oldSummary, newSummary)
		return err
	})
	return merged, err
}

func callSummaryMerge(ctx context.Context, client provider.LLM, oldSummary, newSummary string) (string, error) {
	response, err := client.Chat(ctx, []types.Message{{
		Role: "user", Content: buildSummaryMergePrompt(oldSummary, newSummary),
	}}, nil)
	if err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("empty merge response")
	}
	content := strings.TrimSpace(response.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("empty merge content")
	}
	return content, nil
}

func buildSummaryMergePrompt(oldSummary, newSummary string) string {
	return fmt.Sprintf(`Merge the following two exploration summaries into one concise, deduplicated summary.
Keep ONLY the latest information for each module. Remove contradictions by trusting the newer summary.

Rules:
- Same module path -> keep the NEWER State and Main_Responsibility
- If a Key_Dependency appears in both, merge (union)
- If information conflicts, trust the NEWER
- Output ONLY the merged summaries in the same format - no commentary.

OLD SUMMARY:
%s

NEW SUMMARY:
%s

MERGED:`, oldSummary, newSummary)
}
