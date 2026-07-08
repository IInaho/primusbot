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
