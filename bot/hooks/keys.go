package hooks

const (
	KeyPrefixCounter = "counter:"
	KeyPrefixGauge   = "gauge:"
	KeyPrefixValue   = "value:"
	KeyPrefixTurn    = "turn:"
	KeyPrefixFlag    = "flag:"
	KeyPrefixSession = "session:"
	KeyPrefixPolicy  = "policy:"
)

// Store keys used by runtime/policy instrumentation and builtin hooks.
//
// Lifecycle:
//   - counter: survives ResetTurn; cleared by ResetSession.
//   - gauge/value/turn/flag: cleared by ResetTurn and recomputed for each turn.
//   - session/policy: survives ResetTurn; cleared by ResetSession unless a
//     rule explicitly clears it.
const (
	// Tool call instrumentation. Written by policy.RecordToolCall.
	StoreToolPrefix     = KeyPrefixCounter + "tool:" // + tool name
	StoreToolResearcher = KeyPrefixTurn + "researcher"
	StoreTurnToolCalls  = KeyPrefixTurn + "tool_calls"
	StoreExploreCalls   = KeyPrefixCounter + "explore_calls"
	StoreHasEdits       = KeyPrefixTurn + "has_edits"

	// Turn input and task state. Written by runtime/policy at turn start.
	StoreQuotaReads   = KeyPrefixGauge + "quota_reads"
	StoreExploreScore = KeyPrefixGauge + "explore"
	StoreTasksAllDone = KeyPrefixGauge + "tasks_done"
	StoreHasTasks     = KeyPrefixTurn + "has_tasks"
	StoreStepInputLen = KeyPrefixTurn + "step_len"
	StoreStepInput    = KeyPrefixValue + "step"
	StoreFinalIntent  = KeyPrefixValue + "final_intent"

	// Ledger snapshot. Written by policy.SyncLedgerToHooks.
	StoreLedgerProgress = KeyPrefixTurn + "ledger_progress"

	// Model/tool context. Written near the relevant model/tool hook point.
	StoreRespGarbled          = KeyPrefixCounter + "garbled"
	StoreToolResultCount      = KeyPrefixGauge + "tool_results"
	StoreEditTargetPath       = KeyPrefixValue + "edit_target_path"
	StoreEditTargetExists     = KeyPrefixTurn + "edit_target_exists"
	StoreEditTargetWasRead    = KeyPrefixTurn + "edit_target_was_read"
	StoreEditAnchorSufficient = KeyPrefixTurn + "edit_anchor_sufficient"
	StoreReadOnlyStreak       = KeyPrefixTurn + "read_only_streak"

	// Plugin/session state.
	StoreSessionStarted = KeyPrefixSession + "started"

	// Builtin rule debounce counters.
	CounterQuotaWarned      = KeyPrefixCounter + "quota_warned"
	CounterVerifyInjected   = KeyPrefixCounter + "verify_injected"
	CounterExploreInjected  = KeyPrefixCounter + "explore_injected"
	CounterStallTurns       = KeyPrefixCounter + "stall_turns"
	CounterToolResultWarned = KeyPrefixCounter + "tool_result_warned"

	// Builtin policy latches.
	PolicyExploreExhausted = KeyPrefixPolicy + "explore_exhausted"
)

const (
	FinalIntentFinal       = "final"
	FinalIntentError       = "error"
	FinalIntentFormatError = "format_error"
	FinalIntentNonFinal    = "non_final"
)
