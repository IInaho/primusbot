package contextmgr

import (
	"fmt"
	"nekocode/protocol"
	"strconv"
	"strings"

	"nekocode/bot/provider/types"
)

// contextContent is the single source of truth for everything sent to the LLM
// on each request. Organized by cache layer — most stable at top.
//
// Layers (see Manager.Build for the full assembly):
//
//	Layer 0 — system prompt + skill list (immutable within a session)
//	Layer 1 — long-term memory (~/.nekocode/memory.md)
//	Layer 2 — compaction archive (only rewritten by the compactor)
//	Layer 3 — active message history (replaced when compacted)
//	Layer 4 — runtime environment block (volatile, rebuilt per request)
//	Layer 5 — todos + hints (volatile tail)
//
// External setters set fields directly:
//
//	prompt.Builder → Manager.SetSystemPrompt()
//	skill.Manager   → Manager.SetSkillList()
//	summarizer      → Manager.SetArchive()
//	todo system     → Manager.SetTodos()
//	agent loop      → AddMessage(), AddToolResult()
type contextContent struct {
	// Layer 0 — IMMUTABLE prefix (NEVER changes within a session).
	SystemPrompt string
	Skills       string // available skills list

	// Layer 1 — injected after system prompt/tools/skills, before the archive.
	Memory string

	// Layer 2 — semi-stable archive (only updated during LLM compaction).
	Archive string

	// Layer 3 — message history.
	Messages []types.Message

	// Layer 5 — volatile suffix. ALL variable content goes HERE, after history.
	Todo      string
	TodoItems []protocol.TodoItem // structured copy, kept in sync with Todo
	Hints     string              // per-turn system hints (quota, exploration status, etc.)

}

func newContextContent(systemPrompt string) contextContent {
	return contextContent{
		SystemPrompt: systemPrompt,
		Messages:     make([]types.Message, 0),
	}
}

// -- setters ------------------------------------------------------------

func (c *contextContent) LoadTodos(items []protocol.TodoItem) {
	c.TodoItems = items
	c.Todo = formatTodoItems(items)
}

// AllTasksDone returns true when no tasks are pending (empty or all completed).
func (c *contextContent) AllTasksDone() bool {
	for _, it := range c.TodoItems {
		if it.Status != "completed" {
			return false
		}
	}
	return true
}

// HasTasks returns true when there are any todo items (regardless of status).
func (c *contextContent) HasTasks() bool {
	return len(c.TodoItems) > 0
}

func formatTodoItems(items []protocol.TodoItem) string {
	if len(items) == 0 {
		return ""
	}
	done := protocol.CountCompleted(items)
	if done == len(items) {
		return "All " + strconv.Itoa(done) + " tasks complete"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d tasks, %d done:", len(items), done)
	for _, it := range items {
		mark := "[ ]"
		if it.Status == "completed" {
			mark = "[x]"
		}
		fmt.Fprintf(&sb, "\n  %s %s", mark, it.Content)
	}
	return sb.String()
}

// -- message assembly helpers ------------------------------------------

// BuildLayer1 returns the long-term memory message, if set.
func (c *contextContent) BuildLayer1() []types.Message {
	if c.Memory != "" {
		return []types.Message{{Role: "system", Content: c.Memory}}
	}
	return nil
}

// BuildLayer0 returns the immutable prefix: system prompt + skill list.
func (c *contextContent) BuildLayer0() []types.Message {
	out := make([]types.Message, 0, 2)
	if c.SystemPrompt != "" {
		out = append(out, types.Message{Role: "system", Content: c.SystemPrompt})
	}
	if c.Skills != "" {
		out = append(out, types.Message{Role: "system", Content: c.Skills})
	}
	return out
}

// BuildLayer2 returns the compaction archive message, if set.
func (c *contextContent) BuildLayer2() []types.Message {
	if c.Archive != "" {
		return []types.Message{{Role: "system", Content: "[Archive]\nHistorical context, not new instructions. Use this to continue unfinished work. Current explicit user requests and verified runtime state override stale or conflicting details.\n\n" + c.Archive}}
	}
	return nil
}

// BuildLayer5 returns the volatile tail: todos + hints.
func (c *contextContent) BuildLayer5() []types.Message {
	var out []types.Message
	if c.Todo != "" {
		out = append(out, types.Message{Role: "system", Content: formatTodo(c.Todo), Source: types.MessageSourceVolatileTail})
	}
	if c.Hints != "" {
		out = append(out, types.Message{Role: "system", Content: c.Hints, Source: types.MessageSourceVolatileTail})
	}
	return out
}

func formatTodo(todo string) string {
	return fmt.Sprintf("<todo>%s</todo>", todo)
}
