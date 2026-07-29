package builtin

import "nekocode/bot/policy"

// dashIfEmpty renders an empty string as "-" in audit trigger output, matching
// the registry's formatting convention for missing values.
func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func All() []policy.Hook {
	return []policy.Hook{
		QuotaHook(),
		ToolResultGuardrailHook(),
		ReadBeforeWriteHook(),
		ReadOnlySpiralHook(),
		VerificationHook(),
		ExplorationExhaustedHook(),
		ExploreCascadeHook(),
		ProgressStallHook(),
		GarbledCircuitBreaker(),
	}
}

// Register installs all built-in hooks into p.
func Register(p *policy.Policy) {
	for _, h := range All() {
		p.Register(h)
	}
}
