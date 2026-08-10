package policy

import (
	"nekocode/bot/policy/ledger"
)

// BeforeTool evaluates deterministic tool guards before execution. Resource
// usage remains observable but is not promoted to a hard limit because whether
// another read is useful depends on the task.
func (p *Policy) BeforeTool(request ToolRequest) []Result {
	if p == nil {
		return nil
	}

	target := targetPath(request.Name, request.Args)
	tool := ToolFacts{
		Name:          request.Name,
		Args:          request.Args,
		TargetPath:    target,
		TargetExists:  request.TargetExists,
		TargetWasRead: target != "" && p.ledger.WasRead(target),
	}
	return p.evaluate(PreToolUse, tool)
}

// RecordTool records one event without evaluating batch hooks. Subagents use
// this method when sharing the parent policy's ledger.
func (p *Policy) RecordTool(result ToolResult) {
	if p == nil {
		return
	}
	p.ledger.RecordTool(ledger.ToolEvent{
		Name:      result.Name,
		Args:      result.Args,
		Output:    result.Output,
		Error:     result.Error,
		Blocked:   result.Blocked,
		BlockText: result.BlockReason,
	})
}

// RecordTools records one executed batch and evaluates post-tool hooks.
func (p *Policy) RecordTools(results []ToolResult) []Result {
	if p == nil {
		return nil
	}
	var decisions []Result
	for _, result := range results {
		p.RecordTool(result)
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

	return append(decisions, p.evaluate(PostToolBatch, ToolFacts{})...)
}

func targetPath(toolName string, args map[string]any) string {
	if toolName != "write" && toolName != "edit" {
		return ""
	}
	path, _ := args["path"].(string)
	return path
}
