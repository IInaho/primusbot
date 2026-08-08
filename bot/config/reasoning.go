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
	Efforts       []string
	DisableEffort string
	ThinkingMode  string
	Replay        reasoning.ReplayPolicy
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

// ReasoningCapabilityFor resolves the values accepted by the configured
// model. Matching is case-insensitive and permits provider-prefixed IDs.
func ReasoningCapabilityFor(model ModelConfig) ReasoningCapability {
	profile, ok := findModelProfile(model.Model)
	if !ok {
		return ReasoningCapability{}
	}
	known := profile.reasoning
	thinkingMode := known.openAIThinking
	if strings.EqualFold(strings.TrimSpace(model.Protocol), "anthropic") {
		thinkingMode = known.anthropicThinking
	}
	return ReasoningCapability{
		Efforts: append([]string(nil), known.efforts...), DisableEffort: known.disableEffort,
		ThinkingMode: thinkingMode, Replay: known.replay,
	}
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
		Requested:     requested,
		Effort:        requested,
		Disabled:      requested == "none",
		DisableEffort: capability.DisableEffort,
		ThinkingMode:  capability.ThinkingMode,
		Replay:        capability.Replay,
	}, true
}
