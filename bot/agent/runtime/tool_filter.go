package runtime

import (
	"os"

	"nekocode/bot/policy"
	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/toolutil"
)

type filteredCalls struct {
	Allowed      []core.ToolCallItem
	Blocked      map[int]string
	PreToolHints []*policy.Hint
}

const policyBlockedDefault = "blocked by policy"

func (r *toolRunner) filterToolCalls(calls []core.ToolCallItem) filteredCalls {
	out := filteredCalls{
		Allowed: make([]core.ToolCallItem, 0, len(calls)),
		Blocked: make(map[int]string),
	}
	for i, c := range calls {
		if r.applyPreToolPolicy(c, out.Blocked, i, &out.PreToolHints) {
			continue
		}

		out.Allowed = append(out.Allowed, c)
	}
	return out
}

func (r *toolRunner) applyPreToolPolicy(c core.ToolCallItem, blocked map[int]string, idx int, hints *[]*policy.Hint) bool {
	gov := r.agent.deps.gov
	if gov == nil {
		return false
	}
	shouldBlock := false
	request := policy.ToolRequest{
		Name:         c.Name,
		Args:         c.Args,
		TargetExists: toolTargetExists(c),
	}
	for _, result := range gov.BeforeTool(request) {
		if result.Hint != nil {
			*hints = append(*hints, result.Hint)
		}
		if result.BlockTool != nil && (result.BlockTool.Tool == "" || result.BlockTool.Tool == c.Name) {
			blocked[idx] = result.BlockTool.Reason
			if blocked[idx] == "" {
				blocked[idx] = policyBlockedDefault
			}
			shouldBlock = true
		}
		if result.Stop != nil {
			blocked[idx] = policyBlockedStop(result.Stop.String())
			shouldBlock = true
		}
	}
	return shouldBlock
}

func policyBlockedStop(stop string) string {
	return "blocked by stop policy: " + stop
}

func toolTargetExists(call core.ToolCallItem) bool {
	if call.Name != "write" && call.Name != "edit" {
		return false
	}
	targetPath, _ := call.Args["path"].(string)
	if targetPath != "" {
		if resolved, err := toolutil.ValidatePath(targetPath); err == nil {
			if _, err := os.Stat(resolved); err == nil {
				return true
			}
		}
	}
	return false
}
