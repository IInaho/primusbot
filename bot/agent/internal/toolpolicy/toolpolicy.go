// Package toolpolicy adapts deterministic policy checks to tool calls for
// both the main-agent and sub-agent execution paths.
package toolpolicy

import (
	"os"

	"nekocode/bot/extension/tool/runtime/core"
	"nekocode/bot/extension/tool/runtime/toolutil"
	"nekocode/bot/policy"
)

const defaultBlockReason = "blocked by policy"

// Decision is the complete PreToolUse projection for one tool call.
type Decision struct {
	BlockReason string
	Hints       []policy.Hint
}

// Check evaluates one governance handle using the call's effective identity.
func Check(gov *policy.Policy, call core.ToolCallItem) Decision {
	if gov == nil {
		return Decision{}
	}
	name, args := call.Effective()
	request := policy.ToolRequest{Name: name, Args: args, TargetExists: targetExists(name, args)}
	var decision Decision
	for _, result := range gov.BeforeTool(request) {
		if result.Hint != nil {
			decision.Hints = append(decision.Hints, *result.Hint)
		}
		if result.BlockTool != nil && (result.BlockTool.Tool == "" || result.BlockTool.Tool == name) {
			decision.BlockReason = result.BlockTool.Reason
			if decision.BlockReason == "" {
				decision.BlockReason = defaultBlockReason
			}
		}
		if result.Stop != nil {
			decision.BlockReason = "blocked by stop policy: " + result.Stop.String()
		}
	}
	return decision
}

func targetExists(name string, args map[string]any) bool {
	if name != "write" && name != "edit" {
		return false
	}
	path, _ := args["path"].(string)
	if path == "" {
		return false
	}
	resolved, err := toolutil.ValidatePath(path)
	if err != nil {
		return false
	}
	_, err = os.Stat(resolved)
	return err == nil
}
