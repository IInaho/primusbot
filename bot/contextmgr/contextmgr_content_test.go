package contextmgr

import (
	"nekocode/protocol"
	"strings"
	"testing"

	"nekocode/bot/provider/types"
)

func TestNewContextContent(t *testing.T) {
	c := newContextContent("you are helpful")
	if c.SystemPrompt != "you are helpful" {
		t.Errorf("SystemPrompt = %q", c.SystemPrompt)
	}
	if len(c.Messages) != 0 {
		t.Error("Messages should be empty")
	}
}

func TestLoadTodos(t *testing.T) {
	c := newContextContent("")
	c.LoadTodos([]protocol.TodoItem{
		{Content: "task 1", Status: "pending"},
		{Content: "task 2", Status: "completed"},
	})
	if c.Todo == "" {
		t.Error("Todo should not be empty")
	}
	if !strings.Contains(c.Todo, "task 1") {
		t.Errorf("Todo missing task: %s", c.Todo)
	}
}

func TestAllTasksDone(t *testing.T) {
	c := newContextContent("")
	if !c.AllTasksDone() {
		t.Error("empty list should be 'done'")
	}
	c.LoadTodos([]protocol.TodoItem{
		{Content: "a", Status: "completed"},
	})
	if !c.AllTasksDone() {
		t.Error("all completed should be 'done'")
	}
	c.LoadTodos([]protocol.TodoItem{
		{Content: "a", Status: "pending"},
	})
	if c.AllTasksDone() {
		t.Error("pending tasks should not be 'done'")
	}
}

func TestBuildLayer0(t *testing.T) {
	c := newContextContent("system")
	c.Skills = "skill list"
	msgs := c.BuildLayer0()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "system" {
		t.Errorf("msg0 = %q", msgs[0].Content)
	}
	if msgs[1].Content != "skill list" {
		t.Errorf("msg1 = %q", msgs[1].Content)
	}
}

func TestBuildLayer0_Empty(t *testing.T) {
	c := newContextContent("")
	msgs := c.BuildLayer0()
	if len(msgs) != 0 {
		t.Error("empty system/skills should produce no messages")
	}
}

func TestBuildLayer1(t *testing.T) {
	c := newContextContent("")
	c.Memory = "memory content"
	msgs := c.BuildLayer1()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[0].Content != "memory content" {
		t.Error("memory message incorrect")
	}
}

func TestBuildLayer1_Empty(t *testing.T) {
	c := newContextContent("")
	msgs := c.BuildLayer1()
	if len(msgs) != 0 {
		t.Error("empty memory should produce no messages")
	}
}

func TestBuildLayer2(t *testing.T) {
	c := newContextContent("")
	c.Archive = "archived summary"
	msgs := c.BuildLayer2()
	if len(msgs) != 1 {
		t.Fatal("expected 1 message")
	}
	if !strings.Contains(msgs[0].Content, "[Archive]") {
		t.Errorf("archive message: %s", msgs[0].Content)
	}
}

func TestBuildLayer2_Empty(t *testing.T) {
	c := newContextContent("")
	msgs := c.BuildLayer2()
	if len(msgs) != 0 {
		t.Error("empty archive should produce no messages")
	}
}

func TestBuildLayer5(t *testing.T) {
	c := newContextContent("")
	c.LoadTodos([]protocol.TodoItem{{Content: "x", Status: "pending"}})
	c.Hints = "<hint>"
	msgs := c.BuildLayer5()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

func TestBuildLayer5_Empty(t *testing.T) {
	c := newContextContent("")
	msgs := c.BuildLayer5()
	if len(msgs) != 0 {
		t.Error("empty layer5 should produce no messages")
	}
}

func TestFormatTodoItems_AllDone(t *testing.T) {
	c := newContextContent("")
	c.LoadTodos([]protocol.TodoItem{
		{Content: "a", Status: "completed"},
		{Content: "b", Status: "completed"},
	})
	if !strings.Contains(c.Todo, "All 2 tasks complete") {
		t.Errorf("all done: %s", c.Todo)
	}
}

func TestFormatTodoItems_Mixed(t *testing.T) {
	c := newContextContent("")
	c.LoadTodos([]protocol.TodoItem{
		{Content: "a", Status: "completed"},
		{Content: "b", Status: "pending"},
	})
	if !strings.Contains(c.Todo, "[x]") || !strings.Contains(c.Todo, "[ ]") {
		t.Errorf("mixed: %s", c.Todo)
	}
}

func TestAddMessage(t *testing.T) {
	c := newContextContent("")
	c.Messages = append(c.Messages, types.Message{Role: "user", Content: "hi"})
	if len(c.Messages) != 1 {
		t.Error("message not added")
	}
}

func TestFormatTodo(t *testing.T) {
	s := formatTodo("do stuff")
	if s != "<todo>do stuff</todo>" {
		t.Errorf("formatTodo = %q", s)
	}
}
