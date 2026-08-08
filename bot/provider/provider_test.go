package provider

import (
	"fmt"
	"testing"

	"nekocode/bot/provider/anthropic"
	"nekocode/bot/provider/openai"
	"nekocode/bot/provider/types"
)

func TestOpenAIClient(t *testing.T) {
	c := openai.New("key", "", "deepseek-chat")
	if c.APIKey != "key" {
		t.Error("bad API key")
	}
	if c.BaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("bad base URL: %s", c.BaseURL)
	}
}

func TestOpenAIClient_CustomURL(t *testing.T) {
	c := openai.New("key", "https://api.xiaomimimo.com/v1", "mimo-v3")
	if c.BaseURL != "https://api.xiaomimimo.com/v1" {
		t.Errorf("bad base URL: %s", c.BaseURL)
	}
}

func TestOpenAIThinking(t *testing.T) {
	c := openai.New("k", "", "m")
	c.SetDisableThinking(true)
}

func TestAnthropicClient(t *testing.T) {
	c := anthropic.New("key", "https://api.xiaomimimo.com/anthropic/v1", "mimo-v3")
	if c.BaseURL != "https://api.xiaomimimo.com/anthropic/v1" {
		t.Errorf("bad base URL: %s", c.BaseURL)
	}
}

func TestFactory(t *testing.T) {
	if New(Config{APIKey: "k", Model: "m", Protocol: "openai"}) == nil {
		t.Error("deepseek should return LLM")
	}
	if New(Config{APIKey: "k", BaseURL: "https://api.xiaomimimo.com/anthropic/v1", Model: "mimo-v3", Protocol: "anthropic"}) == nil {
		t.Error("mimo + anthropic should return LLM")
	}
	if New(Config{APIKey: "k", Model: "mimo-v3", Protocol: "openai"}) == nil {
		t.Error("mimo + openai should return LLM")
	}
}

func TestFactoryAppliesReasoningEffort(t *testing.T) {
	openAI := New(Config{Model: "m", Protocol: "openai", Reasoning: types.ReasoningSettings{Requested: "high", Effort: "high"}})
	openAIClient, ok := openAI.(*openai.Client)
	if !ok || openAIClient.ReasoningSettings().Effort != "high" || openAIClient.GetDisableThinking() {
		t.Fatalf("openai effort was not applied: %#v", openAI)
	}
	anthropicClient := New(Config{Model: "m", Protocol: "anthropic", Reasoning: types.ReasoningSettings{Requested: "medium", Effort: "medium"}})
	if got := anthropicClient.(*anthropic.Client).ReasoningSettings().Effort; got != "medium" {
		t.Fatalf("anthropic effort = %q, want medium", got)
	}
	none := New(Config{Model: "m", Protocol: "openai", Reasoning: types.ReasoningSettings{
		Requested: "none", Effort: "none", Disabled: true, DisableEffort: "none",
	}}).(*openai.Client)
	if !none.GetDisableThinking() || none.ReasoningSettings().Effort != "none" {
		t.Fatalf("none effort should disable thinking while preserving the request value: %#v", none)
	}
}

func TestRequestMetaReportsRequestedAndEffectiveReasoning(t *testing.T) {
	client := New(Config{Model: "m", Protocol: "openai"}).(*openai.Client)
	if got := client.RequestMeta(); got.RequestedEffort != "auto" || got.EffectiveEffort != "auto" {
		t.Fatalf("auto metadata = %+v", got)
	}
	client.SetReasoningSettings(types.ReasoningSettings{
		Requested: "high", Effort: "high", ThinkingToggle: true,
	})
	client.SetDisableThinking(true)
	if got := client.RequestMeta(); got.RequestedEffort != "high" || got.EffectiveEffort != "none" {
		t.Fatalf("disabled metadata = %+v", got)
	}
}

func TestRetryable(t *testing.T) {
	if !IsRetryable(fmt.Errorf("connection refused")) {
		t.Error("connection refused should be retryable")
	}
}
