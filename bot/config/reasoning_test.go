package config

import (
	"reflect"
	"testing"

	"nekocode/bot/reasoning"
)

func TestReasoningCapabilityForModel(t *testing.T) {
	tests := []struct {
		model string
		want  []string
	}{
		{"deepseek/deepseek-v4-flash", []string{"auto", "none", "low", "high", "max"}},
		{"gpt-5.5", []string{"auto", "none", "low", "medium", "high", "xhigh"}},
		{"gpt-5.4", []string{"auto", "none", "low", "medium", "high", "xhigh"}},
		{"claude-opus-4-5", []string{"auto", "low", "medium", "high"}},
		{"gpt-5-pro", []string{"auto", "high"}},
		{"gpt-5", []string{"auto", "minimal", "low", "medium", "high"}},
		{"unknown-model", []string{"auto"}},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := ReasoningCapabilityFor(ModelConfig{Model: tt.model}).Values()
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("values = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLegacyClaudeDoesNotUseAdaptiveThinking(t *testing.T) {
	model := ModelConfig{Model: "claude-opus-4-5", Protocol: "anthropic"}
	if got := ReasoningCapabilityFor(model).ThinkingMode; got != "" {
		t.Fatalf("Opus 4.5 thinking mode = %q, want no adaptive mode", got)
	}
}

func TestReasoningCapabilityDefinesReplayContract(t *testing.T) {
	if got := ReasoningCapabilityFor(ModelConfig{Model: "deepseek-v4-flash"}).Replay; got != reasoning.ReplayToolCalls {
		t.Fatalf("DeepSeek replay policy = %v, want tool calls", got)
	}
	if got := ReasoningCapabilityFor(ModelConfig{Model: "claude-sonnet-4-6"}).Replay; got != reasoning.ReplaySigned {
		t.Fatalf("Anthropic replay policy = %v, want signed", got)
	}
	if got := ReasoningCapabilityFor(ModelConfig{Model: "gpt-5.6"}).Replay; got != reasoning.ReplayNone {
		t.Fatalf("default OpenAI replay policy = %v, want none", got)
	}
}

func TestReasoningThinkingModeFollowsWireProtocol(t *testing.T) {
	claude := ModelConfig{Model: "claude-sonnet-4-6", Protocol: "anthropic"}
	if got := ReasoningCapabilityFor(claude).ThinkingMode; got != "adaptive" {
		t.Fatalf("Anthropic Claude thinking mode = %q, want adaptive", got)
	}
	claude.Protocol = "openai"
	if got := ReasoningCapabilityFor(claude).ThinkingMode; got != "" {
		t.Fatalf("OpenAI-compatible Claude emitted Anthropic thinking mode %q", got)
	}
	deepseek := ModelConfig{Model: "deepseek-v4-flash", Protocol: "anthropic"}
	if got := ReasoningCapabilityFor(deepseek).ThinkingMode; got != "enabled" {
		t.Fatalf("Anthropic-compatible DeepSeek thinking mode = %q, want enabled", got)
	}
}

func TestResolveReasoningRejectsUnsupportedValue(t *testing.T) {
	model := ModelConfig{Model: "unknown-model", ReasoningEffort: "high"}
	if _, ok := ResolveReasoning(model); ok {
		t.Fatal("unknown model must not inherit protocol-wide effort values")
	}
	model.Model = "deepseek-v4-flash"
	if got, ok := ResolveReasoning(model); !ok || got.Effort != "high" || got.Disabled {
		t.Fatalf("resolved = %+v, ok=%v", got, ok)
	}
}
