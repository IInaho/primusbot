package todo

import (
	"context"
	"encoding/json"
	"fmt"
	"nekocode/bot/tools/runtime/core"
	commonview "nekocode/common/view"
	"strings"
	"sync"

	"nekocode/bot/tools/runtime/toolhelpers"
)

type TodoWriteTool struct {
	toolhelpers.SequentialSafeTool
	mu       sync.Mutex
	onUpdate commonview.TodoFunc
	items    []commonview.TodoItem
}

func (t *TodoWriteTool) SetUpdateFn(fn commonview.TodoFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onUpdate = fn
}

func (t *TodoWriteTool) Name() string { return "todo_write" }
func (t *TodoWriteTool) Description() string {
	return "Update the task list (record only, not for planning). Each call fully replaces the list. Write the complete list in one call — never append. Format: [{\"content\":\"...\",\"status\":\"pending|in_progress|completed\"}]"
}

func (t *TodoWriteTool) Parameters() []core.Parameter {
	return []core.Parameter{
		{Name: "todos", Type: "string", Required: true, Description: "JSON task list: [{\"content\":\"...\",\"status\":\"pending|in_progress|completed\"}]"},
	}
}

func (t *TodoWriteTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	var items []commonview.TodoItem
	switch v := args["todos"].(type) {
	case string:
		if v == "" {
			return "", fmt.Errorf("missing todos parameter")
		}
		if err := json.Unmarshal([]byte(v), &items); err != nil {
			return "", fmt.Errorf("failed to parse todos: %w", err)
		}
	case []any:
		raw, _ := json.Marshal(v)
		if err := json.Unmarshal(raw, &items); err != nil {
			return "", fmt.Errorf("failed to parse todos: %w", err)
		}
	default:
		return "", fmt.Errorf("invalid todos format: expected JSON string or array, got %T", args["todos"])
	}

	t.mu.Lock()
	t.items = items
	fn := t.onUpdate
	t.mu.Unlock()

	if fn != nil {
		fn(items)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Task list updated (%d items):\n", len(items))
	done := commonview.CountCompleted(items)
	for i, it := range items {
		fmt.Fprintf(&b, "%d. %s %s\n", i+1, commonview.TodoStatusIcon(it.Status), it.Content)
	}
	if done == len(items) {
		fmt.Fprintf(&b, "All %d tasks complete.", done)
	}
	return b.String(), nil
}
