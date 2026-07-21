package subagent

import (
	"context"
	"nekocode/bot/view"
	"sync"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/hooks"
	"nekocode/bot/hooks/builtin"
	"nekocode/bot/provider/types"
	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/runner"
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
	applyReadOnlySpiralGuard(ctxMgr, calls, state)
}

func applyReadOnlySpiralGuard(ctxMgr *ctxmgr.Manager, calls []core.ToolCallItem, state *runState) {
	if isAllExploratory(calls) {
		state.readOnlyStreak++
		if hint := evaluateReadOnlySpiralHook(state.readOnlyStreak); hint != nil {
			ctxMgr.Add("system", hooks.FormatHints([]hooks.Hint{*hint}), "hook")
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

// spiralRegistry is built once: hook registration is immutable after
// builtin.Register, and Registry.Evaluate works on a mutex-guarded snapshot,
// so the registry itself is safe to share. However, Set (per-call streak) and
// Evaluate are not atomic together — concurrent subagents could interleave a
// Set between another goroutine's Set and Evaluate. spiralMu serializes the
// Set+Evaluate pair so each evaluation sees its own streak value.
var (
	spiralRegistryOnce sync.Once
	spiralRegistry     *hooks.Registry
	spiralMu           sync.Mutex
)

func evaluateReadOnlySpiralHook(streak int) *hooks.Hint {
	spiralRegistryOnce.Do(func() {
		spiralRegistry = hooks.NewRegistry()
		builtin.Register(spiralRegistry)
	})
	spiralMu.Lock()
	defer spiralMu.Unlock()
	spiralRegistry.Set(hooks.StoreReadOnlyStreak, int64(streak))
	for _, r := range spiralRegistry.Evaluate(hooks.PostTool, "", false) {
		if r.Hint != nil && r.Hint.Type == "read_only_spiral" {
			return r.Hint
		}
	}
	return nil
}
