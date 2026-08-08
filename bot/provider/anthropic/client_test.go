package anthropic

import (
	"encoding/json"
	"strings"
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

func TestNormalizeUsageIncludesCacheReadAndCreationTokens(t *testing.T) {
	created, read := 50, 14_500
	got := normalizeUsage(wireUsage{
		InputTokens: 25, OutputTokens: 12,
		CacheCreationInputTokens: &created, CacheReadInputTokens: &read,
	})
	if !got.CacheUsageReported || got.PromptTokens != 14_575 || got.CacheHitTokens != 14_500 || got.CacheMissTokens != 75 || got.CompletionTokens != 12 {
		t.Fatalf("normalized usage = %+v", got)
	}
}

func TestNormalizeUsagePreservesMissingCacheDetails(t *testing.T) {
	var usage wireUsage
	if err := json.Unmarshal([]byte(`{"input_tokens":100,"output_tokens":12}`), &usage); err != nil {
		t.Fatal(err)
	}
	got := normalizeUsage(usage)
	if got.CacheUsageReported || got.CacheHitTokens != 0 || got.CacheMissTokens != 0 {
		t.Fatalf("missing cache fields were treated as reported: %+v", got)
	}
}

func TestBuildRequestUsesProviderDefaultTemperatureUnlessConfigured(t *testing.T) {
	c := New("", "", "test-model")
	data, err := json.Marshal(c.buildRequest(nil, nil, false))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"temperature"`) {
		t.Fatalf("default request forced temperature: %s", data)
	}
	c.Temperature = 0.7
	data, err = json.Marshal(c.buildRequest(nil, nil, false))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"temperature":0.7`) {
		t.Fatalf("configured temperature missing: %s", data)
	}
}

func TestBuildRequestIncludesConfiguredReasoningEffort(t *testing.T) {
	c := New("", "", "test-model")
	c.SetReasoningSettings(types.ReasoningSettings{Requested: "medium", Effort: "medium"})
	data, err := json.Marshal(c.buildRequest(nil, nil, false))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"output_config":{"effort":"medium"}`) {
		t.Fatalf("configured effort missing: %s", data)
	}
}

func TestBuildRequestDisableOverridesConfiguredEffort(t *testing.T) {
	c := New("", "", "test-model")
	c.SetReasoningSettings(types.ReasoningSettings{Requested: "high", Effort: "high"})
	c.SetDisableThinking(true)
	data, err := json.Marshal(c.buildRequest(nil, nil, false))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"output_config"`) {
		t.Fatalf("disabled reasoning retained output_config: %s", data)
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
