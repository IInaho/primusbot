package anthropic

import "testing"

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
