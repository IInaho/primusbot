package runtime

import (
	"os"
	"strings"

	"nekocode/bot/policy"
	"nekocode/bot/policy/budget"
	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/toolutil"
	"nekocode/common/debug"
)

type filteredCalls struct {
	Allowed      []core.ToolCallItem
	Blocked      map[int]string
	PreToolHints []*policy.Hint
}

const policyBlockedDefault = "blocked by policy"

func (r *toolRunner) filterToolCalls(calls []core.ToolCallItem, quota *budget.ToolQuota) filteredCalls {
	out := filteredCalls{
		Allowed: make([]core.ToolCallItem, 0, len(calls)),
		Blocked: make(map[int]string),
	}
	for i, c := range calls {
		if err := quota.ConsumeCall(c.Name, c.Args); err != nil {
			out.Blocked[i] = err.Error()
			debug.Log("quota: blocked %s — %v", c.Name, err)
			continue
		}

		if r.applyPreToolPolicy(c, out.Blocked, i, &out.PreToolHints) {
			continue
		}

		out.Allowed = append(out.Allowed, c)
	}
	return out
}

func (r *toolRunner) applyPreToolPolicy(c core.ToolCallItem, blocked map[int]string, idx int, hints *[]*policy.Hint) bool {
	gov := r.agent.deps.gov
	if gov == nil || gov.HookReg == nil {
		return false
	}
	r.preparePreToolHookState(c)
	shouldBlock := false
	for _, result := range gov.HookReg.Evaluate(policy.PreToolUse, c.Name, false, c.Args) {
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

func (r *toolRunner) preparePreToolHookState(tc core.ToolCallItem) {
	gov := r.agent.deps.gov
	if gov == nil || gov.Ledger == nil {
		return
	}
	targetPath := extractTargetPath(tc.Name, tc.Args)
	gov.HookReg.SetStr(policy.StoreEditTargetPath, targetPath)
	gov.HookReg.Flag(policy.StoreEditTargetWasRead, targetPath != "" && gov.Ledger.WasRead(targetPath))
	gov.HookReg.Flag(policy.StoreEditAnchorSufficient, tc.Name == "edit" && hasSufficientEditAnchor(tc.Args))
	exists := false
	if targetPath != "" {
		if resolved, err := toolutil.ValidatePath(targetPath); err == nil {
			if _, err := os.Stat(resolved); err == nil {
				exists = true
			}
		}
	}
	gov.HookReg.Flag(policy.StoreEditTargetExists, exists)
}

func hasSufficientEditAnchor(args map[string]any) bool {
	oldString, _ := args["oldString"].(string)
	oldString = strings.TrimSpace(oldString)
	if oldString == "" {
		return false
	}
	if len([]rune(oldString)) >= 200 {
		return true
	}
	lines := strings.Split(oldString, "\n")
	nonEmpty := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty++
		}
	}
	return nonEmpty >= 5
}

func extractTargetPath(toolName string, args map[string]any) string {
	switch toolName {
	case "write", "edit":
		p, _ := args["path"].(string)
		return p
	}
	return ""
}
