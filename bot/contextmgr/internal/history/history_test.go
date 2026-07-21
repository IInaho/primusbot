package history

import (
	"testing"

	"nekocode/bot/contextmgr/context"
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/contextmgr/token"
	"nekocode/bot/provider/types"
)

func newTestStore() (*Store, *state.State) {
	st := &state.State{
		Ctx:     content.New("test prompt"),
		Tracker: &token.Tracker{},
	}
	return &Store{State: st}, st
}

func TestAdd(t *testing.T) {
	h, st := newTestStore()
	h.Add("user", "hello")
	h.Add("assistant", "world")

	if n := len(st.Ctx.Messages); n != 2 {
		t.Errorf("len = %d, want 2", n)
	}
}

func TestAddAssistantResponse(t *testing.T) {
	h, st := newTestStore()
	h.AddAssistantResponse("response text", "thinking...")
	if n := len(st.Ctx.Messages); n != 1 {
		t.Errorf("len = %d, want 1", n)
	}
}

func TestAddAssistantToolCall(t *testing.T) {
	h, st := newTestStore()
	h.AddAssistantToolCall("let me check", "", []types.ToolCall{
		{ID: "tc1", Type: "function", Function: types.FunctionCall{Name: "read", Arguments: `{}`}},
	})
	if n := len(st.Ctx.Messages); n != 1 {
		t.Errorf("len = %d, want 1", n)
	}
}

func TestAddToolResultsBatch(t *testing.T) {
	h, st := newTestStore()
	h.AddToolResultsBatch([]ToolResultMsg{
		{Message: types.Message{Content: "result1", ToolCallID: "tc1"}, ToolName: "read"},
		{Message: types.Message{Content: "result2", ToolCallID: "tc2"}, ToolName: "grep"},
	})
	if n := len(st.Ctx.Messages); n != 2 {
		t.Errorf("len = %d, want 2", n)
	}
}

func TestAddToolResult_NoToolCallID(t *testing.T) {
	h, st := newTestStore()
	h.AddToolResultsBatch([]ToolResultMsg{
		{Message: types.Message{Content: "orphan result", ToolCallID: ""}, ToolName: "unknown"},
	})
	if n := len(st.Ctx.Messages); n != 1 {
		t.Errorf("len = %d, want 1", n)
	}
	if role := st.Ctx.Messages[0].Role; role != "user" {
		t.Errorf("role = %q, want user", role)
	}
}

func TestTruncateTo(t *testing.T) {
	h, st := newTestStore()
	for i := 0; i < 10; i++ {
		h.Add("user", "msg")
	}
	h.TruncateTo(5)
	if n := len(st.Ctx.Messages); n != 5 {
		t.Errorf("len = %d, want 5", n)
	}
}

func TestTruncateTo_Negative(t *testing.T) {
	h, st := newTestStore()
	h.Add("user", "hello")
	h.TruncateTo(-1)
	if n := len(st.Ctx.Messages); n != 0 {
		t.Errorf("negative truncate clamps to 0: got %d, want 0", n)
	}
}

func TestTruncateTo_Beyond(t *testing.T) {
	h, st := newTestStore()
	h.Add("user", "hello")
	h.TruncateTo(100)
	if n := len(st.Ctx.Messages); n != 1 {
		t.Errorf("beyond-length truncate should keep all: got %d", n)
	}
}

func TestRemoveMessages(t *testing.T) {
	h, st := newTestStore()
	for i := 0; i < 5; i++ {
		h.Add("user", "msg")
	}
	h.RemoveMessages(1, 2)
	if n := len(st.Ctx.Messages); n != 3 {
		t.Errorf("len = %d, want 3", n)
	}
}

func TestRemoveMessages_InvalidRange(t *testing.T) {
	h, st := newTestStore()
	h.Add("user", "hello")
	h.RemoveMessages(0, 10)
	h.RemoveMessages(3, 1)
	h.RemoveMessages(-1, 0)
	if n := len(st.Ctx.Messages); n != 1 {
		t.Errorf("invalid ranges should not modify: got %d", n)
	}
}

func TestClear(t *testing.T) {
	h, st := newTestStore()
	h.Add("user", "hello")
	h.Add("assistant", "world")
	h.Clear()
	if n := len(st.Ctx.Messages); n != 0 {
		t.Errorf("after Clear: len = %d, want 0", n)
	}
}

func TestFreshStart(t *testing.T) {
	h, st := newTestStore()
	h.Add("user", "hello")
	h.FreshStart()
	if n := len(st.Ctx.Messages); n != 0 {
		t.Errorf("after FreshStart: len = %d, want 0", n)
	}
}
