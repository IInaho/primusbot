package policy

// contract.go — Hook 系统的完整协议契约。读这一个文件即可理解 hook 机制：
//
//	生产者（agent runtime / Policy）按 key 常量向 store 写入治理信号
//	  → Registry.Evaluate 在某个 HookPoint 触发时，该点的每条 Hook 拿到
//	    一致的 State 快照（Get/Flag/GetStr 读信号，Set/SetStr 打补丁）
//	  → Hook 返回 Result（提示 / 阻止 / 要求 / 终结 / 状态补丁）
//	  → 评估结束后补丁一次性写回 store，供模型提示、工具拦截与审计使用。
//
// key 前缀的生命周期（ResetTurn 每轮清理 / ResetSession 会话级清理）：
//   - counter:  survives ResetTurn; cleared by ResetSession.
//   - gauge/value/turn/flag: cleared by ResetTurn and recomputed each turn.
//   - session/policy: survives ResetTurn; cleared by ResetSession unless a
//     rule explicitly clears it.

// --- Hook points：评估触发时机 ---

type HookPoint string

const (
	PreTurn         HookPoint = "pre_turn"
	PreModelRequest HookPoint = "pre_model_request"
	PreToolUse      HookPoint = "pre_tool_use"
	PostToolUse     HookPoint = "post_tool_use" // per-tool (declarative hooks)
	PostTool        HookPoint = "post_tool"     // batch (builtin hooks)
	PostTurn        HookPoint = "post_turn"
	UserSubmit      HookPoint = "user_submit"
	Stop            HookPoint = "stop"
)

// --- Hook 与 Result：规则定义与产出 ---

type Hint struct {
	Type     string
	Severity string
	Content  string
}

type StopReason string

const (
	StopFormatError StopReason = "format_error"
	StopInterrupted StopReason = "interrupted"
	StopCompleted   StopReason = "completed"
)

func (s StopReason) String() string { return string(s) }

type Result struct {
	Hint        *Hint
	Stop        *StopReason
	BlockTool   *BlockTool
	RequireTool *RequireTool
	BlockFinal  *BlockFinal
	StatePatch  *StatePatch
}

type BlockTool struct {
	Tool   string
	Reason string
}

type RequireTool struct {
	Tool   string
	Reason string
}

type BlockFinal struct {
	Reason string
}

type Hook struct {
	Name  string
	Point HookPoint
	On    func(s State) *Result
	// DescribeTrigger renders the hook's trigger context for audit output.
	// Optional: when nil, the registry falls back to formatting tool args.
	DescribeTrigger func(s State) string
}

// --- Store keys：生产者与规则之间的信号词汇表 ---

const (
	KeyPrefixCounter = "counter:"
	KeyPrefixGauge   = "gauge:"
	KeyPrefixValue   = "value:"
	KeyPrefixTurn    = "turn:"
	KeyPrefixFlag    = "flag:"
	KeyPrefixSession = "session:"
	KeyPrefixPolicy  = "policy:"
)

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
