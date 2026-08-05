// Package agent implements the Reason-Execute-Feedback loop independently of
// product extensions and interaction transports. New accepts all mandatory and
// optional capabilities through Config; runtime/agentrunner is the standard
// bridge to the transport-neutral runtime.
package agent

import (
	"context"
	"sync"

	"nekocode/bot/agent/internal/kernel"
	"nekocode/bot/checkpoint"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/extension/tool"
	"nekocode/bot/extension/tool/runtime/execution"
	"nekocode/bot/extension/tool/runtime/permission"
	"nekocode/bot/extension/tool/runtime/runner"
	aggov "nekocode/bot/policy"
	"nekocode/bot/provider"
	"nekocode/logger"
	"nekocode/protocol"
)

// Config contains the dependencies and optional behavior of an Agent.
type Config struct {
	Context     *ctxmgr.Manager
	Model       provider.LLM
	Tools       *tools.Registry
	Policy      *aggov.Policy
	Output      Output
	Interaction Interaction
	Checkpoints *checkpoint.Manager
}

// Output contains optional streaming callbacks.
type Output struct {
	Text   func(string)
	Reason func(string)
	Phase  protocol.PhaseFunc
}

// Interaction contains optional application callbacks used by agent tools.
type Interaction struct {
	Confirm protocol.ConfirmFunc
	Ask     protocol.QuestionFunc
	Todos   protocol.TodoFunc
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

	interactionMu sync.RWMutex
	interaction   Interaction

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

func New(ctx context.Context, cfg Config) *Agent {
	a := &Agent{
		life: kernel.NewLifecycle(ctx, steeringChBuffer),
		deps: agentDeps{
			ctxMgr:       cfg.Context,
			llmClient:    cfg.Model,
			toolRegistry: cfg.Tools,
			toolExecutor: runner.NewExecutor(cfg.Tools),
			subSlotMgr:   newSlotManager(),
		},
		gate: kernel.NewGate(defaultMaxRetries),
	}
	a.deps.gov = cfg.Policy
	a.deps.toolExecutor.ExecutionState().Checkpoints = cfg.Checkpoints
	a.setOutput(cfg.Output)
	a.interaction = cfg.Interaction
	a.deps.toolExecutor.SetConfirmFn(a.confirm)
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
	logger.Log("Steer: msg=%q", msg)
	select {
	case a.life.Steering() <- msg:
	default:
	}
	a.life.ReplaceContext()
	logger.Log("Steer: context replaced")
}

func (a *Agent) Abort() {
	logger.Log("Abort: user interrupt requested")
	a.life.Finished().Store(true)
	a.life.Cancel()
}

func (a *Agent) Reset() {
	a.life.ResetContextIfCanceled()
	a.stream.resetReasoning()
	a.run.reset()
	a.gate.Reset()

	a.life.Start()
	a.tokens.snapshot(a.contextTokens())
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

func (a *Agent) PhaseFn() protocol.PhaseFunc {
	return a.stream.phase
}

func (a *Agent) Governance() *aggov.Policy {
	return a.deps.gov
}

func (a *Agent) AddTokens(prompt, completion int) {
	a.tokens.add(prompt, completion)
}

func (a *Agent) TokenUsage() (prompt, completion int) {
	return a.tokens.total(a.contextTokens())
}

func (a *Agent) contextTokens() int {
	return a.deps.ctxMgr.Status().Tokens
}

// Metrics returns one complete operational snapshot for runtime adapters.
func (a *Agent) Metrics() protocol.Metrics {
	status := a.deps.ctxMgr.Status()
	contextTokens := status.Tokens
	prompt, completion := a.tokens.total(contextTokens)
	turnPrompt, turnCompletion := a.tokens.turn(contextTokens)
	return protocol.NewMetrics(protocol.MetricsInput{
		PromptTokens: prompt, CompletionTokens: completion,
		TurnPrompt: turnPrompt, TurnCompletion: turnCompletion,
		ContextTokens: contextTokens, CompactCount: status.CompactCount,
		Duration: a.life.Duration(),
	})
}

// Executor exposes the tool executor so callers can configure tool-level
// behavior (confirm/permission/workspace/plan mode) directly, without the
// Agent relaying one setter per option.
func (a *Agent) Executor() *runner.Executor {
	return a.deps.toolExecutor
}

func (a *Agent) ConfirmFn() protocol.ConfirmFunc {
	return a.deps.toolExecutor.ConfirmFn()
}

// BindInteraction installs callbacks for one run and returns a restore
// function. Runtime adapters should defer the returned function.
func (a *Agent) BindInteraction(next Interaction) func() {
	a.interactionMu.Lock()
	previous := a.interaction
	a.interaction = next
	a.interactionMu.Unlock()
	return func() {
		a.interactionMu.Lock()
		a.interaction = previous
		a.interactionMu.Unlock()
	}
}

// BindOutput installs streaming callbacks for one run and restores the
// previous callbacks afterward.
func (a *Agent) BindOutput(next Output) func() {
	previous := Output{
		Text:   a.stream.text,
		Reason: a.stream.reasoning,
		Phase:  a.stream.phase,
	}
	a.setOutput(next)
	return func() { a.setOutput(previous) }
}

func (a *Agent) setOutput(output Output) {
	a.stream.text = output.Text
	a.stream.reasoning = output.Reason
	a.stream.phase = output.Phase
	a.deps.toolExecutor.SetPhaseFn(output.Phase)
}

// SandboxEngine returns the concrete permission engine shared with tools.
func (a *Agent) SandboxEngine() *permission.Engine {
	return a.deps.toolExecutor.SandboxEngine()
}

func (a *Agent) ToolExecutionState() *execution.ExecutionState {
	return a.deps.toolExecutor.ExecutionState()
}

// ToolInteraction returns the callbacks used by the concrete question and
// todo tools owned by the application composition root.
func (a *Agent) ToolInteraction() (protocol.QuestionFunc, protocol.TodoFunc) {
	return a.ask, a.updateTodos
}

func (a *Agent) confirm(req protocol.ConfirmRequest) protocol.ConfirmReply {
	a.interactionMu.RLock()
	fn := a.interaction.Confirm
	a.interactionMu.RUnlock()
	if fn == nil {
		return protocol.Deny()
	}
	return fn(req)
}

func (a *Agent) ask(req protocol.QuestionRequest) protocol.QuestionReply {
	a.interactionMu.RLock()
	fn := a.interaction.Ask
	a.interactionMu.RUnlock()
	if fn == nil {
		return protocol.QuestionReply{Rejected: true}
	}
	return fn(req)
}

func (a *Agent) updateTodos(items []protocol.TodoItem) {
	a.interactionMu.RLock()
	fn := a.interaction.Todos
	a.interactionMu.RUnlock()
	if fn != nil {
		fn(items)
	}
}
