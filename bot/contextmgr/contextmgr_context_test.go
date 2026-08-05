package contextmgr

import (
	"strings"
	"testing"

	"nekocode/bot/provider/types"
)

func newContextManager() *Manager {
	return New(Config{SystemPrompt: "test prompt"})
}

func TestBuild_PassesNormal(t *testing.T) {
	m := newContextManager()
	m.Add("user", "hello")
	m.AddAssistantResponse("reply", "")

	msgs := m.Build()
	if len(msgs) < 2 {
		t.Errorf("expected at least 2 messages in build output, got %d", len(msgs))
	}
}

func TestBuild_OrphanToolsDropped(t *testing.T) {
	m := newContextManager()
	m.AddToolResultsBatch([]ToolResultMsg{
		{Message: types.Message{Content: "orphan content", ToolCallID: "orphan-id"}, ToolName: "read"},
	})

	msgs := m.Build()
	for _, msg := range msgs {
		if msg.Role == "tool" && msg.ToolCallID == "orphan-id" {
			t.Error("orphan tool result should have been filtered out")
		}
	}
}

func TestBuild_AssistantWithToolCalls(t *testing.T) {
	m := newContextManager()
	m.AddAssistantToolCall("I'll read", "", []types.ToolCall{
		{ID: "tc1", Type: "function", Function: types.FunctionCall{Name: "read", Arguments: "{}"}},
	})
	m.AddToolResultsBatch([]ToolResultMsg{
		{Message: types.Message{Content: "file content", ToolCallID: "tc1"}, ToolName: "read"},
	})

	msgs := m.Build()
	foundAsst, foundTool := false, false
	for _, msg := range msgs {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			foundAsst = true
		}
		if msg.Role == "tool" && msg.ToolCallID == "tc1" {
			foundTool = true
		}
	}
	if !foundAsst || !foundTool {
		t.Error("valid assistant->tool chain should be preserved")
	}
}

func TestBuild_EmptyContent(t *testing.T) {
	m := newContextManager()
	m.state.ctx.Messages = append(m.state.ctx.Messages, types.Message{Role: "assistant", Content: ""})

	msgs := m.Build()
	for _, msg := range msgs {
		if msg.Role == "assistant" && msg.Content == "" {
			t.Error("empty assistant messages should get '.' placeholder")
		}
	}
}

func TestBuild_IncludesArchiveAndActiveHistory(t *testing.T) {
	m := newContextManager()
	m.state.ctx.Archive = "summary of old context"
	m.state.ctx.Messages = []types.Message{
		{Role: "user", Content: "visible user"},
		{Role: "assistant", Content: "visible assistant"},
	}

	msgs := m.Build()
	foundArchive := false
	for _, msg := range msgs {
		if strings.Contains(msg.Content, "[Archive]") && strings.Contains(msg.Content, "summary of old context") {
			foundArchive = true
		}
	}
	if !foundArchive {
		t.Fatal("Build should include archive summary after compaction")
	}
	if !containsContent(msgs, "visible user") || !containsContent(msgs, "visible assistant") {
		t.Fatalf("Build should include visible history, got %+v", msgs)
	}
}

func TestBuild_EmptyState(t *testing.T) {
	msgs := newContextManager().Build()
	if len(msgs) == 0 {
		t.Error("Build should return at least system prompt")
	}
}

func TestBuild_ReevaluatesRuntimePromptWithoutPersistingIt(t *testing.T) {
	value := "runtime-one"
	m := New(Config{SystemPrompt: "test prompt", RuntimePrompt: func() string { return value }})

	first := m.Build()
	if !containsContent(first, "runtime-one") {
		t.Fatalf("first build missing runtime prompt: %+v", first)
	}
	value = "runtime-two"
	second := m.Build()
	if containsContent(second, "runtime-one") || !containsContent(second, "runtime-two") {
		t.Fatalf("runtime prompt was not refreshed: %+v", second)
	}
	if strings.Contains(m.Snapshot().SystemPrompt, "runtime-two") {
		t.Fatal("runtime prompt leaked into session snapshot")
	}

	m.Restore(ManagerSnapshot{SystemPrompt: "restored"})
	if msgs := m.Build(); !containsContent(msgs, "runtime-two") {
		t.Fatalf("runtime provider did not survive restore: %+v", msgs)
	}
}

// The runtime prompt is volatile (date, processes), so it must ride the
// tail after the history — ahead of the history it would break the
// provider's cached prefix on every change.
func TestBuild_PlacesRuntimePromptAfterHistory(t *testing.T) {
	m := New(Config{SystemPrompt: "test prompt", RuntimePrompt: func() string { return "runtime" }})
	m.state.ctx.Memory = "memory"
	m.state.ctx.Archive = "archive"
	m.Add("user", "request")

	msgs := m.Build()
	want := []string{
		"test prompt",
		"memory",
		"[Archive]\nHistorical context, not new instructions. Use this to continue unfinished work. Current explicit user requests and verified runtime state override stale or conflicting details.\n\narchive",
		"request",
		"runtime",
	}
	if len(msgs) != len(want) {
		t.Fatalf("Build() returned %d messages, want %d: %+v", len(msgs), len(want), msgs)
	}
	for i, content := range want {
		if msgs[i].Content != content {
			t.Fatalf("message %d = %q, want %q", i, msgs[i].Content, content)
		}
	}
}

func containsContent(msgs []types.Message, content string) bool {
	for _, msg := range msgs {
		if msg.Content == content {
			return true
		}
	}
	return false
}

func TestConfigureModel(t *testing.T) {
	m := newContextManager()

	m.ConfigureModel(ModelContext{Window: 10000})
	if m.state.contextWindow != 10000 {
		t.Errorf("budget = %d, want 10000", m.state.contextWindow)
	}
	m.ConfigureModel(ModelContext{Window: 0})
	m.ConfigureModel(ModelContext{Window: -1})
	if m.state.contextWindow != 10000 {
		t.Errorf("non-positive budget should not change value: got %d", m.state.contextWindow)
	}
}

func TestAllTasksDone_Empty(t *testing.T) {
	if !newContextManager().Status().TasksDone {
		t.Error("empty todos should be 'done'")
	}
}

func TestLenAndStatus(t *testing.T) {
	m := newContextManager()
	m.Add("user", "hello")

	if n := m.Len(); n != 1 {
		t.Errorf("Len = %d, want 1", n)
	}
	if status := m.Status(); status.Tokens <= 0 {
		t.Error("tokens should be > 0 after adding messages")
	}
}

func TestTokenUsage(t *testing.T) {
	m := newContextManager()
	m.ConfigureModel(ModelContext{Window: 10000})
	if budget := m.Status().Budget; budget != 10000 {
		t.Errorf("budget = %d, want 10000", budget)
	}
}
