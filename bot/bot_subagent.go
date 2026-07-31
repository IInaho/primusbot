package bot

import (
	"context"
	"fmt"

	agentcore "nekocode/bot/agent"
	"nekocode/bot/agent/subagent"
	"nekocode/bot/config"
	"nekocode/bot/provider"
	"nekocode/bot/tools/runtime/taskbridge"
	"nekocode/protocol"
)

func (b *Bot) wireTaskTool(fm config.ModelConfig, compactionModel provider.LLM, ag *agentcore.Agent) {
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
		subLLM := provider.New(provider.Config{
			APIKey: fm.APIKey, BaseURL: fm.BaseURL, Model: fm.Model, Protocol: fm.Protocol,
		})
		subLLM.SetDisableThinking(true)
		engine := subagent.New(subagent.Config{
			LLM: subLLM, Tools: registry, CompactionModel: compactionModel,
		})
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
	ag *agentcore.Agent,
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
			subCB(protocol.StepEvent{
				Action:   ev.Action,
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
