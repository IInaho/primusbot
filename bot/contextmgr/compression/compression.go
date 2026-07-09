package compression

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"nekocode/bot/provider"
	"nekocode/bot/provider/types"
	"nekocode/common/debug"
	"nekocode/util/text"
)

const defaultBudget = 64000

type Level int

const (
	LevelNormal Level = iota
	LevelWarning
	LevelMicroCompact
	LevelCompact
	LevelBlocking
)

func (l Level) String() string {
	switch l {
	case LevelNormal:
		return "normal"
	case LevelWarning:
		return "warning"
	case LevelMicroCompact:
		return "micro_compact"
	case LevelCompact:
		return "compact"
	case LevelBlocking:
		return "blocking"
	default:
		return "unknown"
	}
}

type Config struct {
	WarningBuffer      int
	MicroCompactBuffer int
	CompactBuffer      int
	BlockingBuffer     int
}

var DefaultConfig = Config{
	WarningBuffer:      44800,
	MicroCompactBuffer: 35200,
	CompactBuffer:      25600,
	BlockingBuffer:     6400,
}

func ClassifyLevel(remaining int, cfg Config) Level {
	if remaining <= cfg.BlockingBuffer {
		return LevelBlocking
	}
	if remaining <= cfg.CompactBuffer {
		return LevelCompact
	}
	if remaining <= cfg.MicroCompactBuffer {
		return LevelMicroCompact
	}
	if remaining <= cfg.WarningBuffer {
		return LevelWarning
	}
	return LevelNormal
}

type Summarizer func(msgs []types.Message, prevSummary string) (string, error)

// Strategy is the context manager's compaction boundary.
// Implementations may mutate ctx, archive, and token counters while the
// Manager write lock is held.
type Strategy interface {
	AutoCompactIfNeeded() (Level, error)
	NeedsSummarization() bool
	Summarize() error
	SetSummarizer(Summarizer)
}

const ClearedMarker = "[Old tool result cleared]"

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

const NO_TOOLS_PREAMBLE = `CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.
- Do NOT use Read, Bash, Grep, Glob, Edit, Write, or ANY other tool.
- Tool calls will be REJECTED and will waste your only turn — you will fail the task.
`

func FormatMessages(msgs []types.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		content := strings.TrimSpace(m.Content)
		if content == "" || content == "." || content == ClearedMarker {
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

func BuildPrompt(msgs []types.Message, prevSummary string) string {
	conversation := FormatMessages(msgs)

	template := NO_TOOLS_PREAMBLE + `
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

func FormatCompactSummary(raw string) string {
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

const (
	mergeMaxTokens = 2000
	mergeFailTag   = "[Merge Failed — raw append]"
)

func MergeSummaries(ctx context.Context, llmClient provider.LLM, oldSummary, newSummary string) string {
	if oldSummary == "" {
		return newSummary
	}
	if newSummary == "" {
		return oldSummary
	}

	merged, err := tryMerge(ctx, llmClient, oldSummary, newSummary)
	if err != nil {
		combined := oldSummary + "\n\n" + newSummary
		runes := []rune(combined)
		if len(runes) > mergeMaxTokens*4 {
			combined = string(runes[:mergeMaxTokens*4]) + "\n... (truncated)"
		}
		return fmt.Sprintf("%s\n\n---\n%s\n%s",
			combined, mergeFailTag,
			"Previous merge failed. Content preserved as-is. Async healing will clean up.")
	}
	return merged
}

func tryMerge(ctx context.Context, client provider.LLM, oldSummary, newSummary string) (string, error) {
	origMaxTokens := client.GetMaxTokens()
	origThinking := client.GetDisableThinking()
	client.SetMaxTokens(mergeMaxTokens)
	client.SetDisableThinking(true)
	defer func() {
		client.SetMaxTokens(origMaxTokens)
		client.SetDisableThinking(origThinking)
	}()

	var merged string
	err := provider.Retry(ctx, provider.DefaultRetryConfig, func() error {
		m, err := callMerge(ctx, client, oldSummary, newSummary)
		if err != nil {
			return err
		}
		merged = m
		return nil
	})
	return merged, err
}

func callMerge(ctx context.Context, client provider.LLM, oldSummary, newSummary string) (string, error) {
	prompt := buildMergePrompt(oldSummary, newSummary)
	resp, err := client.Chat(ctx, []types.Message{{Role: "user", Content: prompt}}, nil)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty merge response")
	}
	text := strings.TrimSpace(resp.Choices[0].Message.Content)
	if text == "" {
		return "", fmt.Errorf("empty merge content")
	}
	return text, nil
}

func buildMergePrompt(oldSummary, newSummary string) string {
	return fmt.Sprintf(`Merge the following two exploration summaries into one concise, deduplicated summary.
Keep ONLY the latest information for each module. Remove contradictions by trusting the newer summary.

Rules:
- Same module path → keep the NEWER State and Main_Responsibility
- If a Key_Dependency appears in both, merge (union)
- If information conflicts, trust the NEWER
- Output ONLY the merged summaries in the same format — no commentary.

OLD SUMMARY:
%s

NEW SUMMARY:
%s

MERGED:`, oldSummary, newSummary)
}
