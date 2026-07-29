package app

import (
	"context"
	"fmt"

	agentruntime "nekocode/bot/agent/runtime"
	"nekocode/bot/agent/subagent"
	"nekocode/bot/config"
	"nekocode/bot/provider"
	"nekocode/bot/tools/runtime/taskbridge"
	commonview "nekocode/common/view"
)

func (b *Bot) wireTaskTool(fm config.ModelConfig, ag *agentruntime.Agent) {
	registry := b.toolbox.Registry
	ctxMgr := b.ctxMgr
	cwd := b.cwd
	contextWindow := b.cfg.ContextWindow

	t, err := registry.Get("task")
	if err != nil {
		return
	}
	taskTool, ok := t.(taskbridge.TaskRunnerTool)
	if !ok {
		return
	}
	taskTool.Wire(func(ctx context.Context, prompt, agentType, thoroughness string) (*taskbridge.TaskResult, error) {
		subLLM := provider.NewClientWithProtocol(fm.APIKey, fm.BaseURL, fm.Model, fm.Protocol)
		subLLM.SetDisableThinking(true)
		engine := subagent.NewEngine(subLLM, registry, ctxMgr.MergeClient())
		cfg, ok := buildSubagentRunConfig(ctx, prompt, agentType, thoroughness, cwd, contextWindow, ag)
		if !ok {
			return nil, fmt.Errorf("unknown sub-agent type: %s", agentType)
		}
		result, err := engine.Run(ctx, cfg)
		if result != nil && (result.CacheHitTokens > 0 || result.CacheMissTokens > 0) {
			ctxMgr.RecordSubagent(result.TotalTokens, result.CacheHitTokens, result.CacheMissTokens)
		}
		return subagentTaskResult(result), err
	})
}

func buildSubagentRunConfig(
	ctx context.Context,
	prompt, agentType, thoroughness, cwd string,
	contextWindow int,
	ag *agentruntime.Agent,
) (subagent.RunConfig, bool) {
	at, ok := subagent.Get(agentType)
	if !ok {
		return subagent.RunConfig{}, false
	}
	cfg := subagent.RunConfig{
		Prompt:        prompt,
		AgentType:     at,
		Cwd:           cwd,
		Thoroughness:  thoroughness,
		ContextWindow: contextWindow,
		ConfirmFn:     ag.ConfirmFn(),
		ToolState:     ag.ToolExecutionState(),
		AddTokens:     ag.AddTokens,
		Policy:        ag.Governance(),
	}
	if subCB, ok := taskbridge.TaskCallbackFromCtx(ctx); ok {
		cfg.OnToolCall = func(ev subagent.ToolCallEvent) {
			subCB(commonview.StepEvent{
				Action:   subagentStepAction(ev.Action),
				CallID:   ev.CallID,
				ToolName: ev.ToolName,
				ToolArgs: ev.ToolArgs,
				Output:   ev.Output,
				IsError:  ev.IsError,
			})
		}
	}
	if phaseFn := ag.PhaseFn(); phaseFn != nil {
		cfg.OnPhase = func(p string) { phaseFn(at.Name + " · " + p) }
	}
	return cfg, true
}

func subagentStepAction(action commonview.StepAction) commonview.StepAction {
	switch action {
	case commonview.StepActionToolStart:
		return commonview.StepActionSubToolStart
	case commonview.StepActionExecuteTool:
		return commonview.StepActionSubExecuteTool
	default:
		return action
	}
}

func subagentTaskResult(result *subagent.Result) *taskbridge.TaskResult {
	if result == nil {
		return nil
	}
	status := taskbridge.TaskStatusCompleted
	switch result.Status {
	case subagent.StatusFailed:
		status = taskbridge.TaskStatusFailed
	case subagent.StatusPartial:
		status = taskbridge.TaskStatusPartial
	}
	return &taskbridge.TaskResult{
		Status:  status,
		Content: subagent.FormatResult(result),
	}
}
