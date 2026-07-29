package subagent

import (
	"context"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/policy"
	"nekocode/bot/policy/builtin"
	"nekocode/bot/policy/ledger"
	"nekocode/bot/provider/types"
	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/runner"
	"nekocode/bot/view"
	commonview "nekocode/common/view"
)

func (e *Engine) newExecutor(cfg RunConfig) (*runner.Executor, func()) {
	executor := runner.NewExecutor(e.toolRegistry)
	executor.SetConfirmFn(func(req view.ConfirmRequest) view.ConfirmReply {
		return view.Deny()
	})
	if cfg.ConfirmFn != nil {
		executor.SetConfirmFn(cfg.ConfirmFn)
	}

	toolState := executor.ExecutionState()
	if cfg.ToolState != nil {
		toolState.FileCache.Seed(cfg.ToolState.FileCache)
		if cfg.ToolState.SnapshotStore != nil {
			toolState.SnapshotStore = cfg.ToolState.SnapshotStore
		}
	}
	return executor, func() {
		if cfg.ToolState != nil && cfg.ToolState.FileCache != nil {
			cfg.ToolState.FileCache.Merge(toolState.FileCache)
		}
	}
}

func (e *Engine) executeToolBatch(ctx context.Context, cfg RunConfig, ctxMgr *ctxmgr.Manager, executor *runner.Executor, calls []core.ToolCallItem, state *runState, phase func(string), subLog func(string, ...any)) {
	var toolNames []string
	for _, c := range calls {
		toolNames = append(toolNames, c.Name)
		phase("Running " + c.Name)
		if cfg.OnToolCall != nil {
			cfg.OnToolCall(ToolCallEvent{
				Action:   commonview.StepActionToolStart,
				CallID:   c.ID,
				ToolName: c.Name,
				ToolArgs: core.FormatArgs(c.Args),
			})
		}
	}

	subLog("tools: %v", toolNames)
	results := executor.ExecuteBatch(ctx, calls)
	batch := make([]ctxmgr.ToolResultMsg, len(results))
	for i, r := range results {
		content := r.EffectiveOutput()
		batch[i] = ctxmgr.ToolResultMsg{
			Message:  types.Message{Content: content, ToolCallID: r.ID},
			ToolName: calls[i].Name,
		}
		if cfg.OnToolCall != nil {
			cfg.OnToolCall(ToolCallEvent{
				Action: commonview.StepActionExecuteTool, CallID: r.ID, ToolName: calls[i].Name,
				ToolArgs: core.FormatArgs(calls[i].Args), Output: content, IsError: r.Error != "",
			})
		}
	}
	ctxMgr.AddToolResultsBatch(batch)
	if cfg.Policy != nil {
		for i, r := range results {
			cfg.Policy.RecordToolCall(ledger.ToolEvent{
				Name:   calls[i].Name,
				Args:   calls[i].Args,
				Output: r.Output,
				Error:  r.Error,
			})
		}
	}
	applyReadOnlySpiralGuard(ctxMgr, calls, state)
}

func applyReadOnlySpiralGuard(ctxMgr *ctxmgr.Manager, calls []core.ToolCallItem, state *runState) {
	if isAllExploratory(calls) {
		state.readOnlyStreak++
		if hint := builtin.ReadOnlySpiralHint(state.readOnlyStreak); hint != nil {
			ctxMgr.Add("system", policy.FormatHints([]policy.Hint{*hint}), "hook")
			state.readOnlyStreak = 0
		}
		return
	}
	state.readOnlyStreak = 0
}

// isAllExploratory reports whether every call in the batch is a read-only
// exploration tool. This is an agent-layer policy (the whitelist encodes
// subagent behavior), not a tool runtime concern.
func isAllExploratory(calls []core.ToolCallItem) bool {
	if len(calls) == 0 {
		return false
	}
	for _, c := range calls {
		switch c.Name {
		case "read", "grep", "glob", "list", "web_search", "web_fetch":
			continue
		default:
			return false
		}
	}
	return true
}
