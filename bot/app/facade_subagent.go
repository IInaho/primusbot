package app

import (
	"context"
	"fmt"

	"nekocode/bot/agent/subagent"
	"nekocode/bot/config"
	"nekocode/bot/contextmgr"
	"nekocode/bot/provider"
	"nekocode/bot/tools"
	"nekocode/bot/tools/runtime/execution"
	"nekocode/bot/tools/runtime/taskbridge"
	"nekocode/bot/view"
	commonview "nekocode/common/view"
)

type subagentWiring struct {
	toolRegistry  *tools.Registry
	ctxMgr        *contextmgr.Manager
	cwd           string
	contextWindow int
}

func newSubagentWiring(toolRegistry *tools.Registry, ctxMgr *contextmgr.Manager, cwd string, contextWindow int) *subagentWiring {
	return &subagentWiring{
		toolRegistry:  toolRegistry,
		ctxMgr:        ctxMgr,
		cwd:           cwd,
		contextWindow: contextWindow,
	}
}

func (w *subagentWiring) WireTaskTool(fm config.ModelConfig, ag agentCallbacks) {
	t, err := w.toolRegistry.Get("task")
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
		engine := subagent.NewEngine(subLLM, w.toolRegistry, w.ctxMgr.MergeClient())
		cfg, ok := w.buildSubagentRunConfig(ctx, prompt, agentType, thoroughness, ag)
		if !ok {
			return nil, fmt.Errorf("unknown sub-agent type: %s", agentType)
		}
		result, err := engine.Run(ctx, cfg)
		if result != nil && (result.CacheHitTokens > 0 || result.CacheMissTokens > 0) {
			w.ctxMgr.RecordSubagent(result.TotalTokens, result.CacheHitTokens, result.CacheMissTokens)
		}
		return subagentTaskResult(result), err
	})
}

type agentCallbacks interface {
	ConfirmFn() view.ConfirmFunc
	ToolExecutionState() *execution.ExecutionState
	PhaseFn() view.PhaseFunc
	AddTokens(prompt, completion int)
}

func (w *subagentWiring) buildSubagentRunConfig(ctx context.Context, prompt, agentType, thoroughness string, ag agentCallbacks) (subagent.RunConfig, bool) {
	return buildSubagentRunConfig(subagentRunConfigInput{
		Context:       ctx,
		Prompt:        prompt,
		AgentTypeName: agentType,
		Thoroughness:  thoroughness,
		Cwd:           w.cwd,
		ContextWindow: w.contextWindow,
		ConfirmFn:     ag.ConfirmFn(),
		ToolState:     ag.ToolExecutionState(),
		PhaseFn:       ag.PhaseFn(),
		AddTokens:     ag.AddTokens,
	})
}

type subagentRunConfigInput struct {
	Context       context.Context
	Prompt        string
	AgentTypeName string
	Thoroughness  string
	Cwd           string
	ContextWindow int
	ConfirmFn     view.ConfirmFunc
	ToolState     *execution.ExecutionState
	PhaseFn       func(string)
	AddTokens     func(prompt, completion int)
}

func buildSubagentRunConfig(input subagentRunConfigInput) (subagent.RunConfig, bool) {
	at, ok := subagent.Get(input.AgentTypeName)
	if !ok {
		return subagent.RunConfig{}, false
	}
	cfg := subagent.RunConfig{
		Prompt:        input.Prompt,
		AgentType:     at,
		Cwd:           input.Cwd,
		Thoroughness:  input.Thoroughness,
		ContextWindow: input.ContextWindow,
		ConfirmFn:     input.ConfirmFn,
		ToolState:     input.ToolState,
		AddTokens:     input.AddTokens,
	}
	if subCB, ok := taskbridge.TaskCallbackFromCtx(input.Context); ok {
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
	if input.PhaseFn != nil {
		cfg.OnPhase = func(p string) { input.PhaseFn(at.Name + " · " + p) }
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
