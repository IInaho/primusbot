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

// Register registers all built-in hooks into r.
func Register(r *policy.Registry) {
	for _, h := range All() {
		r.Register(h)
	}
}
