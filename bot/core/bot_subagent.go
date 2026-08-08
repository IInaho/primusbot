package core

import (
	"context"
	"fmt"

	agentcore "nekocode/bot/agent"
	"nekocode/bot/agent/subagent"
	"nekocode/bot/config"
	"nekocode/bot/extension/tool/runtime/taskbridge"
	"nekocode/bot/prompt"
	"nekocode/bot/provider"
	"nekocode/protocol"
)

func (b *Bot) wireTaskTool(fm config.ModelConfig, compactionModel provider.LLM, ag *agentcore.Agent) {
	registry := b.toolbox.Registry
	ctxMgr := b.ctxMgr
	contextWindow := b.cfg.EffectiveContextWindow()
	autoCompactPercent := b.cfg.EffectiveAutoCompactPercent()

	b.toolbox.WireTaskRunner(func(ctx context.Context, prompt, agentType, thoroughness string) (*taskbridge.TaskResult, error) {
		subLLM := provider.New(provider.Config{
			APIKey: fm.APIKey, BaseURL: fm.BaseURL, Model: fm.Model, Protocol: fm.Protocol,
			Reasoning: resolvedReasoning(fm),
		})
		subLLM.SetDisableThinking(true)
		engine := subagent.New(subagent.Config{
			LLM: subLLM, Tools: registry, CompactionModel: compactionModel,
		})
		cfg, ok := buildSubagentRunConfig(ctx, prompt, agentType, thoroughness, contextWindow, autoCompactPercent, ag, b.environment)
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
	prompt, agentType, thoroughness string,
	contextWindow, autoCompactPercent int,
	ag *agentcore.Agent,
	environment prompt.EnvironmentProvider,
) (subagent.RunConfig, bool) {
	at, ok := subagent.Get(agentType)
	if !ok {
		return subagent.RunConfig{}, false
	}
	cfg := subagent.RunConfig{
		Prompt:             prompt,
		AgentType:          at,
		Thoroughness:       thoroughness,
		ContextWindow:      contextWindow,
		AutoCompactPercent: autoCompactPercent,
		ConfirmFn:          ag.ConfirmFn(),
		FullAccess:         ag.Executor().FullAccess,
		ToolState:          ag.ToolExecutionState(),
		AddTokens: func(_ int, completion int) {
			ag.AddCompletionTokens(completion)
		},
		RecordLLMUsage: ag.RecordLLMUsage,
		Policy:         ag.Governance(),
		Environment:    environment,
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
