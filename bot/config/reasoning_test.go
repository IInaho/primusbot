package config

import (
	"reflect"
	"testing"
)

func TestReasoningCapabilityForModel(t *testing.T) {
	tests := []struct {
		model string
		want  []string
	}{
		{"deepseek/deepseek-v4-flash", []string{"auto", "none", "high", "max"}},
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
