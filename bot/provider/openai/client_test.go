package openai

import (
	"encoding/json"
	"strings"
	"testing"

	"nekocode/bot/provider/types"
)

func TestBuildBodyOmitsInternalToolErrorFlag(t *testing.T) {
	c := New("", "", "test-model")
	body := c.buildBody([]types.Message{
		{Role: "tool", Content: "command failed", ToolCallID: "tc1", IsError: true},
	}, nil, false)
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "is_error") {
		t.Fatalf("request body leaks internal is_error flag: %s", s)
	}
	if !strings.Contains(s, `"tool_call_id":"tc1"`) {
		t.Fatalf("request body missing tool_call_id: %s", s)
	}
}

// Volatile layers must keep their tail positions: the provider's prefix
// cache matches byte-for-byte from the front, so hoisting a per-turn hint
// ahead of the history would cold-start the cache on every turn.
func TestToAPIMessagesPreservesOrderAndPositions(t *testing.T) {
	got := toAPIMessages([]types.Message{
		{Role: "system", Content: "stable"},
		{Role: "user", Content: "request"},
		{Role: "system", Content: "runtime"},
		{Role: "assistant", Content: "answer"},
		{Role: "system", Content: "current hint"},
	})
	if len(got) != 5 {
		t.Fatalf("messages = %d, want all 5 in original order: %+v", len(got), got)
	}
	want := []struct{ role, content string }{
		{"system", "stable"},
		{"user", "request"},
		{"system", "runtime"},
		{"assistant", "answer"},
		{"system", "current hint"},
	}
	for i, w := range want {
		if got[i].Role != w.role || got[i].Content != w.content {
			t.Fatalf("message %d = %+v, want %s %q", i, got[i], w.role, w.content)
		}
	}
}

func TestEndpointJoinsBaseURL(t *testing.T) {
	c := New("", "https://api.example.com/openai/v1/", "test-model")
	if got := c.endpoint("/chat/completions"); got != "https://api.example.com/openai/v1/chat/completions" {
		t.Fatalf("endpoint = %q", got)
	}
}

func TestEndpointAddsMissingVersion(t *testing.T) {
	c := New("", "https://api.longcat.chat/openai", "test-model")
	if got := c.endpoint("chat/completions"); got != "https://api.longcat.chat/openai/v1/chat/completions" {
		t.Fatalf("endpoint = %q", got)
	}
}
