package builtin

import "nekocode/bot/hooks"

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
