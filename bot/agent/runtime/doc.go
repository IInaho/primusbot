// Package runtime is the L2 agent core of NekoCode: it drives the
// Reason → Execute → Feedback loop of a single agent.
//
// # Layering
//
//	L0 primitives   bot/agent/kernel (loop/lifecycle/gate), bot/agent/llmstream, bot/provider
//	L1 capabilities bot/contextmgr, bot/tools, bot/policy
//	L2 agent core   bot/agent/runtime  ← this package
//	L3 product shell bot/app
//	L4 interfaces   bot/command, interaction/*, runtime/httpapi
//
// The package depends only on L0/L1; nothing here may import L3/L4.
//
// # Public contract
//
//   - Construction: New(ctx, AgentConfig). Optional capabilities (HookReg,
//     TodoWriter) are injected via AgentConfig — nil means disabled.
//   - Control: Run (one full agent run), Steer (interrupt with a message),
//     Abort, Reset.
//   - Introspection: TokenUsage / TurnTokenUsage / ContextTokens / Duration /
//     Executor / Governance / ToolExecutionState / SandboxProfiler.
//   - Stream wiring: SetStreamFn / SetReasoningStreamFn / SetPhaseFn
//     (re-attached by the caller around each Run).
//   - Events: RunCallback delivers commonview.StepEvent values — the single
//     currency crossing layer boundaries.
//
// # Contract rules
//
//   - Step events are the only data that crosses layers; callers never see
//     internal turn/tool state.
//   - Optional capabilities come in through AgentConfig; there are no
//     post-construction config setters on Agent.
//   - Tool-execution options are configured on the Executor directly, not
//     relayed through Agent.
//   - The internal runners (loopRunner / modelRunner / turnRunner /
//     toolRunner) and the reasoning classification types are not exported;
//     their behavior is observable only through Run's result and events.
package runtime
