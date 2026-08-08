package config

import (
	"slices"
	"strings"

	"nekocode/bot/reasoning"
)

const ReasoningAuto = ""

// ReasoningCapability is the model-specific reasoning contract exposed to
// configuration and interactive clients. Efforts contains provider-native
// values only; Auto is always available and is represented by an empty value.
type ReasoningCapability struct {
	Efforts        []string
	DisableEffort  string
	ThinkingToggle bool
}

func (c ReasoningCapability) Supports(effort string) bool {
	effort, ok := ParseReasoningEffort(effort)
	if !ok {
		return false
	}
	if effort == ReasoningAuto {
		return true
	}
	return slices.Contains(c.Efforts, effort)
}

// Values returns the command/config values in display order, including auto.
func (c ReasoningCapability) Values() []string {
	values := make([]string, 1, len(c.Efforts)+1)
	values[0] = "auto"
	return append(values, c.Efforts...)
}

type reasoningModelCapability struct {
	model          string
	efforts        []string
	disableEffort  string
	thinkingToggle bool
}

// First match wins. Entries intentionally describe model families rather than
// protocols: OpenAI-compatible endpoints do not imply a common reasoning API.
// Unknown models therefore expose Auto only.
var reasoningModelCapabilities = []reasoningModelCapability{
	{model: "deepseek-v4", efforts: []string{"none", "high", "max"}, disableEffort: "none", thinkingToggle: true},
	{model: "deepseek-chat", efforts: []string{"none", "high", "max"}, disableEffort: "none", thinkingToggle: true},
	{model: "deepseek-reasoner", efforts: []string{"none", "high", "max"}, disableEffort: "none", thinkingToggle: true},

	{model: "claude-opus-5", efforts: []string{"low", "medium", "high", "xhigh", "max"}},
	{model: "claude-sonnet-5", efforts: []string{"low", "medium", "high", "xhigh", "max"}},
	{model: "claude-fable-5", efforts: []string{"low", "medium", "high", "xhigh", "max"}},
	{model: "claude-opus-4-8", efforts: []string{"low", "medium", "high", "xhigh", "max"}},
	{model: "claude-opus-4-7", efforts: []string{"low", "medium", "high", "xhigh", "max"}},
	{model: "claude-opus-4-6", efforts: []string{"low", "medium", "high", "max"}},
	{model: "claude-sonnet-4-6", efforts: []string{"low", "medium", "high", "max"}},
	{model: "claude-opus-4-5", efforts: []string{"low", "medium", "high", "max"}},

	{model: "gpt-5.6", efforts: []string{"none", "low", "medium", "high", "xhigh", "max"}, disableEffort: "none"},
	{model: "gpt-5.5", efforts: []string{"none", "low", "medium", "high", "xhigh", "max"}, disableEffort: "none"},
	{model: "gpt-5.1", efforts: []string{"none", "low", "medium", "high"}, disableEffort: "none"},
	{model: "gpt-5-pro", efforts: []string{"high"}},
	{model: "gpt-5", efforts: []string{"minimal", "low", "medium", "high"}},
	{model: "o1-", efforts: []string{"low", "medium", "high"}},
	{model: "o3-", efforts: []string{"low", "medium", "high"}},
	{model: "o4-mini", efforts: []string{"low", "medium", "high"}},

	{model: "gemini-3", efforts: []string{"minimal", "low", "medium", "high"}},
	{model: "gemini-2.5-pro", efforts: []string{"low", "medium", "high"}},
	{model: "gemini-2.5-flash", efforts: []string{"none", "minimal", "low", "medium", "high"}, disableEffort: "none"},
}

// ReasoningCapabilityFor resolves the values accepted by the configured
// model. Matching is case-insensitive and permits provider-prefixed IDs.
func ReasoningCapabilityFor(model ModelConfig) ReasoningCapability {
	id := strings.ToLower(strings.TrimSpace(model.Model))
	for _, known := range reasoningModelCapabilities {
		if strings.Contains(id, known.model) {
			return ReasoningCapability{
				Efforts: append([]string(nil), known.efforts...), DisableEffort: known.disableEffort,
				ThinkingToggle: known.thinkingToggle,
			}
		}
	}
	return ReasoningCapability{}
}

// ResolveReasoning validates and translates one model's portable setting into
// the provider-facing reasoning contract.
func ResolveReasoning(model ModelConfig) (reasoning.Settings, bool) {
	requested, ok := ParseReasoningEffort(model.ReasoningEffort)
	capability := ReasoningCapabilityFor(model)
	if !ok || !capability.Supports(requested) {
		return reasoning.Settings{}, false
	}
	return reasoning.Settings{
		Requested:      requested,
		Effort:         requested,
		Disabled:       requested == "none",
		DisableEffort:  capability.DisableEffort,
		ThinkingToggle: capability.ThinkingToggle,
	}, true
}
