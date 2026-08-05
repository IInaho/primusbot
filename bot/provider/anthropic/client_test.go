package anthropic

import (
	"testing"

	"nekocode/bot/provider/types"
)

func TestToMessagesConsolidatesSystemContext(t *testing.T) {
	got, system := toMessages([]types.Message{
		{Role: "system", Content: "stable"},
		{Role: "user", Content: "request"},
		{Role: "system", Content: "runtime", Source: types.MessageSourceVolatileTail},
		{Role: "assistant", Content: "answer"},
		{Role: "system", Content: "current hint", Source: types.MessageSourceVolatileTail},
	})
	if system != "stable" {
		t.Fatalf("volatile tail was hoisted into system context: %q", system)
	}
	if len(got) != 4 || got[0].Role != "user" || got[1].Role != "user" || got[2].Role != "assistant" || got[3].Role != "user" {
		t.Fatalf("volatile tail position changed: %+v", got)
	}
}

func TestEndpointJoinsBaseURL(t *testing.T) {
	c := New("", "https://api.example.com/anthropic/v1/", "test-model")
	if got := c.endpoint("/messages"); got != "https://api.example.com/anthropic/v1/messages" {
		t.Fatalf("endpoint = %q", got)
	}
}

func TestEndpointAddsMissingVersion(t *testing.T) {
	c := New("", "https://api.longcat.chat/anthropic", "test-model")
	if got := c.endpoint("messages"); got != "https://api.longcat.chat/anthropic/v1/messages" {
		t.Fatalf("endpoint = %q", got)
	}
}

func TestHeadersSupportAnthropicAndBearerAuth(t *testing.T) {
	c := New("test-key", "", "test-model")
	headers := c.headers()
	if headers["x-api-key"] != "test-key" {
		t.Fatalf("x-api-key = %q", headers["x-api-key"])
	}
	if headers["Authorization"] != "Bearer test-key" {
		t.Fatalf("Authorization = %q", headers["Authorization"])
	}
}
