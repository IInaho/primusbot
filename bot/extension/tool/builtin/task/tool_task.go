package task

import (
	"context"
	"fmt"

	"nekocode/bot/extension/tool/runtime/core"
	"nekocode/bot/extension/tool/runtime/taskbridge"
	"nekocode/bot/extension/tool/runtime/toolutil"
)

type TaskTool struct {
	toolutil.SafeReadOnlyTool
	run taskbridge.TaskRunner
}

func NewTaskTool() *TaskTool { return &TaskTool{} }

func (t *TaskTool) Wire(run taskbridge.TaskRunner) {
	t.run = run
}

func (t *TaskTool) Name() string { return "task" }
func (t *TaskTool) Description() string {
	return "Delegate independent, multi-step work to an isolated sub-agent. Only the main agent can use this tool; sub-agents cannot spawn nested agents or see this conversation. Include the exact goal, scope, relevant paths, constraints, and expected evidence. Types: researcher (read-only analysis), executor (implementation), verify (read-only validation). Use direct tools for simple or tightly coupled work, and verify returned claims before relying on them."
}

func (t *TaskTool) Parameters() []core.Parameter {
	return []core.Parameter{
		{Name: "type", Type: "string", Required: true,
			Enum: []string{"researcher", "executor", "verify"}, Description: "Sub-agent role."},
		{Name: "prompt", Type: "string", Required: true,
			Description: "Self-contained task description with exact file paths and expected output."},
	}
}

func (t *TaskTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if t.run == nil {
		return "", fmt.Errorf("task tool: not wired")
	}

	prompt, err := toolutil.RequireStringArg(args, "prompt")
	if err != nil {
		return "", err
	}

	typeName, err := toolutil.RequireStringArg(args, "type")
	if err != nil {
		return "", fmt.Errorf("missing type parameter — must specify: researcher, executor, verify")
	}

	thoroughness := ""
	if len(prompt) < 300 {
		thoroughness = "quick"
	} else if len(prompt) > 1000 {
		thoroughness = "very thorough"
	}

	// Read sub-callback from args (injected by agent for TUI forwarding).
	subCtx := ctx
	if cb, ok := args["_sub_callback"].(taskbridge.TaskCallbackFn); ok {
		subCtx = taskbridge.WithTaskCallback(ctx, cb)
		delete(args, "_sub_callback") // clean up
	}
	result, err := t.run(subCtx, prompt, typeName, thoroughness)
	if err != nil && result == nil {
		return "", err
	}
	if result == nil {
		return "", fmt.Errorf("task tool: subagent returned nil result")
	}

	out := result.Content
	if err != nil && result.Status == taskbridge.TaskStatusPartial {
		out += fmt.Sprintf("\n\nNote: subagent was interrupted before completion: %v", err)
	}
	return out, nil
}
