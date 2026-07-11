package builder

import (
	"strings"
	"testing"

	"nekocode/bot/contextmgr/context"
	"nekocode/bot/contextmgr/internal/history"
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider/types"
)

func newTestState() *state.State {
	return &state.State{
		Ctx:     context.New("test prompt"),
		Tracker: &token.Tracker{},
	}
}

func TestBuild_PassesNormal(t *testing.T) {
	st := newTestState()
	h := &history.Store{State: st}
	h.Add("user", "hello")
	h.AddAssistantResponse("reply", "")

	msgs := (&Builder{State: st}).Build()
	if len(msgs) < 2 {
		t.Errorf("expected at least 2 messages in build output, got %d", len(msgs))
	}
}

func TestBuild_OrphanToolsDropped(t *testing.T) {
	st := newTestState()
	(&history.Store{State: st}).AddToolResultsBatch([]history.ToolResultMsg{
		{Message: types.Message{Content: "orphan content", ToolCallID: "orphan-id"}, ToolName: "read"},
	})

	msgs := (&Builder{State: st}).Build()
	for _, msg := range msgs {
		if msg.Role == "tool" && msg.ToolCallID == "orphan-id" {
			t.Error("orphan tool result should have been filtered out")
		}
	}
}

func TestBuild_AssistantWithToolCalls(t *testing.T) {
	st := newTestState()
	h := &history.Store{State: st}
	h.AddAssistantToolCall("I'll read", "", []types.ToolCall{
		{ID: "tc1", Type: "function", Function: types.FunctionCall{Name: "read", Arguments: "{}"}},
	})
	h.AddToolResultsBatch([]history.ToolResultMsg{
		{Message: types.Message{Content: "file content", ToolCallID: "tc1"}, ToolName: "read"},
	})

	msgs := (&Builder{State: st}).Build()
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
	st := newTestState()
	st.Ctx.Messages = append(st.Ctx.Messages, types.Message{Role: "assistant", Content: ""})

	msgs := (&Builder{State: st}).Build()
	for _, msg := range msgs {
		if msg.Role == "assistant" && msg.Content == "" {
			t.Error("empty assistant messages should get '.' placeholder")
		}
	}
}

func TestBuild_UsesCompactBoundary(t *testing.T) {
	st := newTestState()
	st.Ctx.Archive = "summary of old context"
	st.Ctx.Messages = []types.Message{
		{Role: "user", Content: "old user"},
		{Role: "assistant", Content: "old assistant"},
		{Role: "user", Content: "visible user"},
		{Role: "assistant", Content: "visible assistant"},
	}
	st.Ctx.CompactBoundary = 2

	msgs := (&Builder{State: st}).Build()
	foundArchive := false
	for _, msg := range msgs {
		if strings.Contains(msg.Content, "old user") || strings.Contains(msg.Content, "old assistant") {
			t.Fatalf("Build exported compacted history message: %+v", msg)
		}
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
	msgs := (&Builder{State: newTestState()}).Build()
	if len(msgs) == 0 {
		t.Error("Build should return at least system prompt")
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
