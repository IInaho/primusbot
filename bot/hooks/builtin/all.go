package builtin

import "nekocode/bot/hooks"

// dashIfEmpty renders an empty string as "-" in audit trigger output, matching
// the registry's formatting convention for missing values.
func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func All() []hooks.Hook {
	return []hooks.Hook{
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
