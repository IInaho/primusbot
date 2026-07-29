package policy

import (
	"strings"

	"nekocode/bot/policy/ledger"
	"nekocode/bot/policy/semantics"
)

const ReadOnlySpiralThreshold = 3

func ReadOnlySpiralHint(streak int) *Hint {
	if streak < ReadOnlySpiralThreshold {
		return nil
	}
	return &Hint{
		Type:     "read_only_spiral",
		Severity: "warning",
		Content:  "You've been reading without acting. Summarize your findings now - don't read any more files.",
	}
}

// BeforeTool charges quota and evaluates tool guards before execution.
func (p *Policy) BeforeTool(request ToolRequest) []Result {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	err := p.quota.ConsumeCall(request.Name, request.Args)
	p.readsLeft = max(0, p.quota.MaxSlots-p.quota.Used)
	p.mu.Unlock()
	if err != nil {
		return []Result{{BlockTool: &BlockTool{Tool: request.Name, Reason: err.Error()}}}
	}

	target := targetPath(request.Name, request.Args)
	tool := ToolFacts{
		Name:                 request.Name,
		Args:                 request.Args,
		TargetPath:           target,
		TargetExists:         request.TargetExists,
		TargetWasRead:        target != "" && p.ledger.WasRead(target),
		EditAnchorSufficient: request.Name == "edit" && sufficientEditAnchor(request.Args),
	}
	return p.evaluate(PreToolUse, tool)
}

// RecordTool records one event without evaluating batch hooks. Subagents use
// this method when sharing the parent policy's ledger and exploration score.
func (p *Policy) RecordTool(result ToolResult) {
	if p == nil {
		return
	}
	sem := semantics.ClassifyToolCall(result.Name, result.Args)
	p.ledger.RecordTool(ledger.ToolEvent{
		Name:      result.Name,
		Args:      result.Args,
		Output:    result.Output,
		Error:     result.Error,
		Blocked:   result.Blocked,
		BlockText: result.BlockReason,
		Semantics: sem,
	})
	mutationApplied := !result.Blocked && (!sem.Mutating || result.Error == "")
	if !sem.Mutating || mutationApplied {
		p.exploration.Record(result.Name, result.Args)
	}
}

// RecordTools records one executed batch, evaluates per-tool hooks, updates
// the read-only streak and finally evaluates batch hooks.
func (p *Policy) RecordTools(results []ToolResult) []Result {
	if p == nil {
		return nil
	}
	var decisions []Result
	allExploratory := len(results) > 0
	for _, result := range results {
		p.RecordTool(result)
		sem := semantics.ClassifyToolCall(result.Name, result.Args)
		allExploratory = allExploratory && sem.Exploratory && !sem.Mutating
		if result.Blocked {
			continue
		}
		tool := ToolFacts{
			Name:  result.Name,
			Args:  result.Args,
			Error: result.Error != "",
		}
		decisions = append(decisions, p.evaluate(PostToolUse, tool)...)
	}

	p.mu.Lock()
	if allExploratory {
		p.readOnlyStreak++
	} else {
		p.readOnlyStreak = 0
	}
	p.mu.Unlock()
	return append(decisions, p.evaluate(PostTool, ToolFacts{})...)
}

func targetPath(toolName string, args map[string]any) string {
	if toolName != "write" && toolName != "edit" {
		return ""
	}
	path, _ := args["path"].(string)
	return path
}

func sufficientEditAnchor(args map[string]any) bool {
	oldString, _ := args["oldString"].(string)
	oldString = strings.TrimSpace(oldString)
	if oldString == "" {
		return false
	}
	if len([]rune(oldString)) >= 200 {
		return true
	}
	nonEmpty := 0
	for _, line := range strings.Split(oldString, "\n") {
		if strings.TrimSpace(line) != "" {
			nonEmpty++
		}
	}
	return nonEmpty >= 5
}
