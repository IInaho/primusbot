package runtime

import (
	"context"
	"time"

	"nekocode/bot/view"
	commonview "nekocode/common/view"

	"nekocode/bot/agent/kernel"
	ctxmgr "nekocode/bot/contextmgr"
	aggov "nekocode/bot/policy"
	"nekocode/bot/provider"
	"nekocode/bot/tools"
	"nekocode/bot/tools/builtin/shell"
	"nekocode/bot/tools/runtime/execution"
	"nekocode/bot/tools/runtime/runner"
	"nekocode/common/debug"
)

// AgentConfig carries the construction-time dependencies of an Agent.
// Policy and TodoWriter are optional: nil leaves governance unwired and the
// todo_write tool without an update callback.
type AgentConfig struct {
	CtxMgr     *ctxmgr.Manager
	LLM        provider.LLM
	Registry   *tools.Registry
	Policy     *aggov.Policy
	TodoWriter commonview.TodoFunc
}

type agentDeps struct {
	ctxMgr       *ctxmgr.Manager
	llmClient    provider.LLM
	toolRegistry *tools.Registry
	toolExecutor *runner.Executor
	subSlotMgr   *slotManager
	gov          *aggov.Policy
}

type Agent struct {
	// Lifecycle.
	life *kernel.Lifecycle

	// Dependencies.
	deps agentDeps

	// Streaming callbacks and model reasoning.
	stream streamState

	// Token accounting.
	tokens tokenMeter

	// Current run state.
	run runState

	// Policy-block retry gate, persists across runs (reset per run in Reset).
	gate *kernel.Gate

	loopRunner  *loopRunner
	modelRunner *modelRunner
	turnRunner  *turnRunner
	toolRunner  *toolRunner
}

func New(ctx context.Context, cfg AgentConfig) *Agent {
	a := &Agent{
		life: kernel.NewLifecycle(ctx, steeringChBuffer),
		deps: agentDeps{
			ctxMgr:       cfg.CtxMgr,
			llmClient:    cfg.LLM,
			toolRegistry: cfg.Registry,
			toolExecutor: runner.NewExecutor(cfg.Registry),
			subSlotMgr:   newSlotManager(),
		},
		gate: kernel.NewGate(defaultMaxRetries),
	}
	a.deps.gov = cfg.Policy
	if cfg.TodoWriter != nil {
		a.wireTodoWrite(cfg.TodoWriter)
	}
	a.loopRunner = newLoopRunner(a)
	a.modelRunner = newModelRunner(a)
	a.turnRunner = newTurnRunner(a)
	a.toolRunner = newToolRunner(a)
	return a
}

func (a *Agent) Run(input string, callback RunCallback) *RunResult {
	return a.loopRunner.run(input, callback)
}

func (a *Agent) getCtx() context.Context {
	return a.life.Context()
}

func (a *Agent) Steer(msg string) {
	debug.Log("Steer: msg=%q", msg)
	select {
	case a.life.Steering() <- msg:
	default:
	}
	a.life.ReplaceContext()
	debug.Log("Steer: context replaced")
}

func (a *Agent) Abort() {
	debug.Log("Abort: user interrupt requested")
	a.life.Finished().Store(true)
	a.life.Cancel()
}

func (a *Agent) Duration() time.Duration {
	return a.life.Duration()
}

func (a *Agent) Reset() {
	a.life.ResetContextIfCanceled()
	a.stream.resetReasoning()
	a.run.reset()
	a.gate.Reset()

	a.life.Start()
	a.tokens.snapshot(a.ContextTokens())
	if a.deps.gov != nil {
		a.deps.gov.ResetRun()
	}
	a.deps.ctxMgr.SetTodos(nil)
	a.deps.ctxMgr.SetHints("")
}

func (a *Agent) injectHint(h *aggov.Hint) {
	if h != nil {
		a.run.pendingHints = append(a.run.pendingHints, *h)
	}
}

func collectHints(results []aggov.Result) []aggov.Hint {
	var hints []aggov.Hint
	for _, result := range results {
		if result.Hint != nil {
			hints = append(hints, *result.Hint)
		}
	}
	return hints
}

func (a *Agent) applyTurnHints(hints []aggov.Hint) {
	if len(a.run.pendingHints) > 0 {
		hints = append(hints, a.run.pendingHints...)
		a.run.pendingHints = nil
	}
	a.deps.ctxMgr.SetHints(aggov.FormatHints(hints))
}

func (a *Agent) drainSteering() {
	for {
		select {
		case msg := <-a.life.Steering():
			a.deps.ctxMgr.Add("user", msg, "user")
		default:
			return
		}
	}
}

func (a *Agent) SetStreamFn(fn StreamCallback) {
	a.stream.text = fn
}

func (a *Agent) SetReasoningStreamFn(fn ReasoningCallback) {
	a.stream.reasoning = fn
}

// SetPhaseFn wires the phase callback into both the agent's own stream
// state and the tool executor (which emits tool-level phases).
func (a *Agent) SetPhaseFn(fn view.PhaseFunc) {
	a.stream.phase = fn
	a.deps.toolExecutor.SetPhaseFn(fn)
}

func (a *Agent) PhaseFn() view.PhaseFunc {
	return a.stream.phase
}

func (a *Agent) Governance() *aggov.Policy {
	return a.deps.gov
}

func (a *Agent) AddTokens(prompt, completion int) {
	a.tokens.add(prompt, completion)
}

func (a *Agent) TokenUsage() (prompt, completion int) {
	return a.tokens.total(a.ContextTokens())
}

func (a *Agent) TurnTokenUsage() (prompt, completion int) {
	return a.tokens.turn(a.ContextTokens())
}

func (a *Agent) ContextTokens() int {
	_, tokens, _ := a.deps.ctxMgr.Stats()
	return tokens
}

// Executor exposes the tool executor so callers can configure tool-level
// behavior (confirm/permission/workspace/plan mode) directly, without the
// Agent relaying one setter per option.
func (a *Agent) Executor() *runner.Executor {
	return a.deps.toolExecutor
}

func (a *Agent) ConfirmFn() view.ConfirmFunc {
	return a.deps.toolExecutor.ConfirmFn()
}

// SandboxProfiler returns the permission engine as a SandboxProfiler so tools
// can look up sandbox rules (e.g. pnpm dev → network).
func (a *Agent) SandboxProfiler() shell.SandboxProfiler {
	return a.deps.toolExecutor.SandboxEngine()
}

func (a *Agent) ToolExecutionState() *execution.ExecutionState {
	return a.deps.toolExecutor.ExecutionState()
}

func (a *Agent) wireTodoWrite(fn commonview.TodoFunc) {
	if t, err := a.deps.toolRegistry.Get("todo_write"); err == nil {
		if updater, ok := t.(interface{ SetUpdateFn(commonview.TodoFunc) }); ok {
			updater.SetUpdateFn(fn)
		}
	}
}
