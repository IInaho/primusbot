package runtime

import (
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/policy"
	"nekocode/bot/policy/ledger"
	"nekocode/bot/provider/types"
	"nekocode/bot/tools/runtime/core"
	commonview "nekocode/common/view"
)

func emitStartCallbacks(calls []core.ToolCallItem, blocked map[int]string, callback RunCallback) {
	if callback == nil {
		return
	}
	for i, c := range calls {
		action := commonview.StepActionToolStart
		preview, _ := c.Args["_preview"].(string)
		if reason, ok := blocked[i]; ok {
			action = commonview.StepActionToolBlocked
			preview = reason
		}
		callback(commonview.StepEvent{Action: action, CallID: c.ID, ToolName: c.Name, ToolArgs: core.FormatArgs(c.Args), Output: preview})
	}
}

func mergeResults(calls []core.ToolCallItem, blocked map[int]string, execResults []core.ToolCallResult) []core.ToolCallResult {
	results := make([]core.ToolCallResult, len(calls))
	execIdx := 0
	for i := range calls {
		if msg, ok := blocked[i]; ok {
			results[i] = core.ToolCallResult{ID: calls[i].ID, Name: calls[i].Name, Error: msg}
			continue
		}
		results[i] = execResults[execIdx]
		execIdx++
	}
	return results
}

func emitResultCallbacks(calls []core.ToolCallItem, blocked map[int]string, results []core.ToolCallResult, callback RunCallback) []types.Message {
	msgs := make([]types.Message, len(results))
	for i, r := range results {
		content := r.EffectiveOutput()
		msgs[i] = types.Message{Content: content, ToolCallID: r.ID, IsError: r.Error != ""}
		if callback != nil {
			if _, isBlocked := blocked[i]; isBlocked {
				continue
			}
			callback(commonview.StepEvent{Action: commonview.StepActionExecuteTool, CallID: r.ID, ToolName: r.Name, ToolArgs: core.FormatArgs(calls[i].Args), Output: content, IsError: r.Error != ""})
		}
	}
	return msgs
}

func (r *toolRunner) executeAllowedTools(allowed []core.ToolCallItem, callback RunCallback) []core.ToolCallResult {
	executor := r.agent.deps.toolExecutor
	if callback != nil {
		executor.SetPreviewFn(func(toolName string, _ map[string]any, preview string) {
			callback(commonview.StepEvent{Action: commonview.StepActionToolPreview, ToolName: toolName, Output: preview})
		})
	} else {
		executor.SetPreviewFn(nil)
	}
	if len(allowed) == 0 {
		return nil
	}
	return executor.ExecuteBatch(r.agent.getCtx(), allowed)
}

func (r *toolRunner) recordToolCalls(calls []core.ToolCallItem, blocked map[int]string, results []core.ToolCallResult) {
	gov := r.agent.deps.gov
	if gov == nil {
		return
	}
	for i, tc := range calls {
		if msg, ok := blocked[i]; ok {
			gov.RecordToolCall(ledger.ToolEvent{
				Name:      tc.Name,
				Args:      tc.Args,
				Blocked:   true,
				BlockText: msg,
			})
			continue
		}
		gov.RecordToolCall(ledger.ToolEvent{
			Name:   tc.Name,
			Args:   tc.Args,
			Output: results[i].Output,
			Error:  results[i].Error,
		})
	}
}

func (r *toolRunner) evaluatePostToolUseHints(calls []core.ToolCallItem, blocked map[int]string, results []core.ToolCallResult) []*policy.Hint {
	gov := r.agent.deps.gov
	if gov == nil || gov.HookReg == nil {
		return nil
	}
	var hints []*policy.Hint
	for i, result := range results {
		if _, skip := blocked[i]; skip {
			continue
		}
		toolErr := result.Error != ""
		for _, hr := range gov.HookReg.Evaluate(policy.PostToolUse, calls[i].Name, toolErr, calls[i].Args) {
			if hr.Hint != nil {
				hints = append(hints, hr.Hint)
			}
		}
	}
	return hints
}

func (r *toolRunner) addToolResultsAndHints(calls []core.ToolCallItem, msgs []types.Message, preToolHints, postToolHints []*policy.Hint) {
	toolResults := make([]ctxmgr.ToolResultMsg, len(msgs))
	for i, m := range msgs {
		toolResults[i] = ctxmgr.ToolResultMsg{Message: m, ToolName: calls[i].Name}
	}
	r.agent.deps.ctxMgr.AddToolResultsBatch(toolResults)

	for _, h := range preToolHints {
		r.agent.injectHint(h)
	}
	for _, h := range postToolHints {
		r.agent.injectHint(h)
	}
}
