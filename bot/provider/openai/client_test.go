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

func TestToAPIMessagesConsolidatesSystemContext(t *testing.T) {
	got := toAPIMessages([]types.Message{
		{Role: "system", Content: "stable"},
		{Role: "user", Content: "request"},
		{Role: "system", Content: "runtime"},
		{Role: "assistant", Content: "answer"},
		{Role: "system", Content: "current hint"},
	})
	if len(got) != 3 {
		t.Fatalf("messages = %d, want one system plus history: %+v", len(got), got)
	}
	if got[0].Role != "system" || got[0].Content != "stable\n\nruntime\n\ncurrent hint" {
		t.Fatalf("system context was not consolidated in order: %+v", got[0])
	}
	if got[1].Role != "user" || got[2].Role != "assistant" {
		t.Fatalf("non-system history order changed: %+v", got)
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
