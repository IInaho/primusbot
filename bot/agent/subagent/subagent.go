package subagent

import (
	"context"
	"time"

	"nekocode/bot/agent/internal/kernel"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/provider"
	"nekocode/bot/tools"
	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/runner"
	"nekocode/logger"
)

const (
	thoroughDeep     = "very thorough"
	taskToolName     = "task"
	maxSubAgentSteps = 50
)

type Engine struct {
	llmClient       provider.LLM
	toolRegistry    *tools.Registry
	compactionModel provider.LLM
}

// Config contains the services shared by subagent runs.
type Config struct {
	LLM             provider.LLM
	Tools           *tools.Registry
	CompactionModel provider.LLM
}

type runState struct {
	startTime      time.Time
	toolUseCount   int
	totalTokens    int
	sensitiveOps   int
	readOnlyStreak int
	lastText       string
}

// New creates a reusable subagent engine.
func New(config Config) *Engine {
	return &Engine{
		llmClient: config.LLM, toolRegistry: config.Tools, compactionModel: config.CompactionModel,
	}
}

func newRunState() *runState {
	return &runState{startTime: time.Now()}
}

func (s *runState) addTokens(cfg RunConfig) func(int, int) {
	return func(prompt, compl int) {
		s.totalTokens += prompt + compl
		if cfg.AddTokens != nil {
			cfg.AddTokens(prompt, compl)
		}
	}
}

func (s *runState) meta(ctxMgr *ctxmgr.Manager) runMeta {
	hit, miss := ctxMgr.CacheStats()
	return runMeta{
		totalTokens:     s.totalTokens,
		toolUseCount:    s.toolUseCount,
		durationMs:      time.Since(s.startTime).Milliseconds(),
		cacheHitTokens:  hit,
		cacheMissTokens: miss,
		sensitiveOps:    s.sensitiveOps,
	}
}

func (s *runState) recordCalls(calls []core.ToolCallItem) {
	s.toolUseCount += len(calls)
	for _, c := range calls {
		if isSensitiveCall(c) {
			s.sensitiveOps++
		}
	}
}

func (e *Engine) Run(ctx context.Context, cfg RunConfig) (*Result, error) {
	subLog := logger.Sub(cfg.AgentType.Name)
	subLog("start: prompt=%q", cfg.Prompt[:min(len(cfg.Prompt), 120)])
	defer func(start time.Time) {
		subLog("done: duration=%v", time.Since(start).Round(time.Millisecond))
	}(time.Now())

	state := newRunState()
	executor, cleanupExecutor := e.newExecutor(cfg)
	defer cleanupExecutor()

	ctxMgr := e.newContextManager(cfg)

	ctxMgr.Add("user", buildTaskPrompt(cfg))
	phase := phaseReporter(cfg)
	phase("Waiting")

	run := &engineRun{
		engine:   e,
		ctx:      ctx,
		cfg:      cfg,
		ctxMgr:   ctxMgr,
		executor: executor,
		state:    state,
		phase:    phase,
		log:      subLog,
	}

	kernel.RunLoop(kernel.Loop{
		StepLimitReached: run.stepLimitReached,
		Step:             run.stepOnce,
	})

	return run.finish()
}

type engineRun struct {
	engine   *Engine
	ctx      context.Context
	cfg      RunConfig
	ctxMgr   *ctxmgr.Manager
	executor *runner.Executor
	state    *runState
	phase    func(string)
	log      func(string, ...any)

	step   int
	result *Result
	err    error
}

func (r *engineRun) stepLimitReached() bool {
	if r.step < maxSubAgentSteps {
		return false
	}
	r.log("max steps reached: step=%d", r.step)
	r.result = buildPartialResult(r.state.lastText, r.state.meta(r.ctxMgr))
	return true
}

func (r *engineRun) stepOnce() bool {
	if r.ctx.Err() != nil {
		r.log("interrupted: step=%d lastText=%q", r.step, r.state.lastText[:min(len(r.state.lastText), 200)])
		r.result = buildPartialResult(r.state.lastText, r.state.meta(r.ctxMgr))
		r.err = r.ctx.Err()
		return true
	}

	if err := r.ctxMgr.AutoCompactIfNeeded(); err != nil {
		r.log("compact error: %v", err)
		if r.state.lastText != "" {
			r.result = buildPartialResult(r.state.lastText, r.state.meta(r.ctxMgr))
		} else {
			r.result = buildFailedResult(err.Error(), r.state.meta(r.ctxMgr))
		}
		r.err = err
		return true
	}
	calls, text, err := r.engine.reason(r.ctx, r.ctxMgr, r.cfg.AgentType.Tools, r.state.addTokens(r.cfg), r.phase)
	r.ctxMgr.SetHints("")
	if err != nil {
		r.log("error: %v", err)
		if r.state.lastText != "" {
			r.log("partial_result: %q", r.state.lastText[:min(len(r.state.lastText), 300)])
			r.result = buildPartialResult(r.state.lastText, r.state.meta(r.ctxMgr))
			return true
		}
		r.result = buildFailedResult(err.Error(), r.state.meta(r.ctxMgr))
		r.err = err
		return true
	}

	if text != "" {
		r.state.lastText = text
	}
	if len(calls) == 0 {
		r.complete(text)
		return true
	}

	r.state.recordCalls(calls)
	r.engine.executeToolBatch(r.ctx, r.cfg, r.ctxMgr, r.executor, calls, r.state, r.phase, r.log)
	r.phase("Waiting")
	r.step++
	return false
}

func (r *engineRun) complete(text string) {
	r.phase("done")
	r.result = buildResult(text, r.state.meta(r.ctxMgr))
	r.log("result: tokens=%d tools=%d duration=%dms output=%q",
		r.result.TotalTokens, r.result.ToolUseCount, r.result.DurationMs,
		text[:min(len(text), 300)])
}

func (r *engineRun) finish() (*Result, error) {
	if r.result == nil {
		r.result = buildPartialResult(r.state.lastText, r.state.meta(r.ctxMgr))
	}
	return r.result, r.err
}

type Status int

const (
	StatusCompleted Status = iota
	StatusFailed
	StatusPartial
)

type classification int

const (
	classPass classification = iota
	classWarn
	classUnavailable
)

type Result struct {
	Status          Status
	Content         string
	TotalTokens     int
	ToolUseCount    int
	DurationMs      int64
	CacheHitTokens  int
	CacheMissTokens int
	classification  classification
}

type runMeta struct {
	totalTokens     int
	toolUseCount    int
	durationMs      int64
	cacheHitTokens  int
	cacheMissTokens int
	sensitiveOps    int
}

func FormatResult(r *Result) string {
	if r.classification == classWarn {
		return "SECURITY WARNING: This sub-agent performed actions that may violate security policy.\n\n" + r.Content
	}
	return r.Content
}

// classifyHandoff inspects both the subagent's text output and its actual tool
// operations (via meta.sensitiveOps) for dangerous patterns. This catches cases
// where a subagent performed sensitive operations (reading .env, running rm,
// etc.) but the text output doesn't mention the filenames or commands explicitly.
func classifyHandoff(rawOutput string, meta runMeta) classification {

	if meta.sensitiveOps > 0 {
		return classWarn
	}

	if isDangerousCommand(rawOutput) || isSensitivePath(rawOutput) {
		return classWarn
	}
	return classPass
}

func newResult(status Status, content string, meta runMeta, cls classification) *Result {
	return &Result{
		Status:          status,
		Content:         content,
		TotalTokens:     meta.totalTokens,
		ToolUseCount:    meta.toolUseCount,
		DurationMs:      meta.durationMs,
		CacheHitTokens:  meta.cacheHitTokens,
		CacheMissTokens: meta.cacheMissTokens,
		classification:  cls,
	}
}

func buildResult(rawOutput string, meta runMeta) *Result {
	return newResult(StatusCompleted, rawOutput, meta, classifyHandoff(rawOutput, meta))
}

func buildPartialResult(lastText string, meta runMeta) *Result {
	return newResult(StatusPartial, lastText, meta, classUnavailable)
}

func buildFailedResult(errMsg string, meta runMeta) *Result {
	return newResult(StatusFailed, errMsg, meta, classUnavailable)
}
