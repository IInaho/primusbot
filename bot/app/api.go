package app

import (
	"fmt"

	"nekocode/bot/agent/runtime"
	"nekocode/bot/command"
	"nekocode/bot/config"
	"nekocode/bot/contextmgr/memory"
	"nekocode/bot/view"
	commonview "nekocode/common/view"
)

func (b *Bot) Steer(msg string) { b.getAgent().Steer(msg) }
func (b *Bot) Abort()           { b.getAgent().Abort() }

func (b *Bot) Close() {
	b.mu.Lock()
	sh := b.shellTool
	b.shellTool = nil
	b.mu.Unlock()
	if sh != nil {
		sh.Shutdown()
	}
}

func (b *Bot) ProviderModel() (string, string) {
	am := b.cfg.ActiveModelConfig()
	return am.Provider, am.Model
}

func (b *Bot) CommandNames() []string { return b.cmdParser.Commands() }

func (b *Bot) ExecuteCommand(input string) (string, view.CmdResult) {
	b.skillState.WantsAgent = false
	cmd := b.cmdParser.Parse(input)
	if cmd.Name == "" {
		command.ClearSkillContext(b.ctxMgr, b.skillState)
		return "", view.CmdNone
	}
	resp, _ := b.cmdParser.Execute(cmd)

	// Commands like /summarize, /clear, /new modify context state
	// (Archive, CompactBoundary, Messages). Save the session so those
	// changes are persisted — RunAgent already does this after each turn.
	b.sess.Save()

	resumed := b.sess.DrainResumed()
	if resumed {
		b.syncHookSessionID()
	}
	result := commandResult(b.cb.pendingConfirmation(), resumed)
	return resp, result
}

func commandResult(pendingConfirm, sessionResumed bool) view.CmdResult {
	switch {
	case pendingConfirm:
		return view.CmdConfirming
	case sessionResumed:
		return view.CmdSessionResumed
	default:
		return view.CmdHandled
	}
}

func (b *Bot) SkillHint() (string, bool) {
	hint := b.skillState.Hint
	cont := b.skillState.WantsAgent
	b.skillState.Hint = ""
	b.skillState.WantsAgent = false
	return hint, cont
}

func (b *Bot) getAgent() *runtime.Agent {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ag
}

func (b *Bot) Run(input string, callbacks view.RunCallbacks) (string, error) {
	if callbacks.Text != nil || callbacks.Reason != nil {
		b.SetCallbacks(callbacks.Text, callbacks.Reason)
	}
	return b.RunAgent(input, callbacks.Step)
}

func (b *Bot) RunAgent(input string, onStep func(ev commonview.StepEvent)) (string, error) {
	ag := b.getAgent()
	result := ag.Run(input, onStep)
	ag.SetPlanMode(false)
	b.ctxMgr.SetSystemPrompt(b.promptBuilder.Build())
	command.SummarizeIfNeeded(b.ctxMgr)
	if result.Interrupted {
		if err := b.sess.SaveIfNotEmpty(); err != nil && result.Error == nil {
			result.Error = err
		}
	} else {
		b.sess.Save()
	}
	return result.FinalOutput, result.Error
}

func (b *Bot) ConfigView() view.ConfigView {
	b.mu.Lock()
	defer b.mu.Unlock()
	return view.NewConfigView(*b.cfg)
}

func (b *Bot) ApplyConfig(cfgView view.ConfigView) (view.ConfigView, error) {
	next := view.ToConfig(cfgView)
	if err := config.Validate(&next); err != nil {
		return view.ConfigView{}, err
	}
	if err := config.Save(next); err != nil {
		return view.ConfigView{}, err
	}

	b.mu.Lock()
	oldPrompt, oldCompl := 0, 0
	if b.ag != nil {
		oldPrompt, oldCompl = b.ag.TokenUsage()
	}
	b.cfg = &next
	b.mu.Unlock()

	go b.reloadRuntime(oldPrompt, oldCompl)

	return view.NewConfigView(next), nil
}

func (b *Bot) reloadRuntime(oldPrompt, oldCompl int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reinit()
	if b.ag != nil {
		b.ag.AddTokens(oldPrompt, oldCompl)
	}
}

func (b *Bot) SwitchModel(name string) (string, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.cfg.SwitchModel(name) {
		return "", "", fmt.Errorf("model %q not found. Available: %v", name, b.cfg.AllModelNames())
	}

	oldPrompt, oldCompl := b.ag.TokenUsage()
	b.initAgent()
	b.ag.AddTokens(oldPrompt, oldCompl)
	b.ctxMgr.ResetCache()

	am := b.cfg.ActiveModelConfig()
	return am.Model, am.Provider, nil
}

func (b *Bot) ContextStatus() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return command.ContextStats(b.ctxMgr)
}

func (b *Bot) ContextReport() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return command.ContextReport(b.ctxMgr, b.toolRegistry.Descriptors())
}

func (b *Bot) ContextSnapshot() view.ContextSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()

	r := b.ctxMgr.Report()
	r.ToolDefCount = len(b.toolRegistry.Descriptors())
	r.ToolDefTokens = command.EstimateToolDefTokens(b.toolRegistry.Descriptors())
	return view.NewContextSnapshot(r)
}

func (b *Bot) MemoryView(scope view.MemoryScope) view.MemoryView {
	b.mu.Lock()
	defer b.mu.Unlock()

	if scope == "" {
		scope = view.MemoryScopeProject
	}
	snap := b.ctxMgr.Snapshot()
	return view.NewMemoryView(scope, memory.DefaultPath(), snap.Memory)
}

func (b *Bot) SelectSkill(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	skills := skillCommandProvider{manager: b.ext.skills}
	sk, ok := skills.GetForCommand(name)
	if !ok {
		return fmt.Errorf("skill %q not found", name)
	}
	command.ClearSkillContext(b.ctxMgr, b.skillState)
	b.skillState.MsgStart = b.ctxMgr.Len()
	b.ctxMgr.Add("user", sk.Context)
	b.skillState.MsgEnd = b.ctxMgr.Len()
	b.skillState.Hint = name
	skills.MarkLoaded(name)
	return nil
}

func (b *Bot) ClearSelectedSkill() {
	b.mu.Lock()
	defer b.mu.Unlock()
	command.ClearSkillContext(b.ctxMgr, b.skillState)
	b.skillState.Hint = ""
	b.skillState.WantsAgent = false
}

func (b *Bot) SkillManagementView() commonview.SkillManagementView {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ext.SkillManagementView()
}

func (b *Bot) SetPluginEnabled(name string, enabled bool) (commonview.SkillManagementView, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ext.SetPluginEnabled(name, enabled)
}

func (b *Bot) RefreshSkillManagement() commonview.SkillManagementView {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ext.RefreshSkillManagement()
}

func (b *Bot) Stats() view.BotStats {
	ag := b.getAgent()

	p, c := ag.TokenUsage()
	tp, tc := ag.TurnTokenUsage()
	d := ag.Duration()
	compactCount, _ := b.ctxMgr.CompactStats()
	return view.NewBotStats(view.BotStatsInput{
		PromptTokens:     p,
		CompletionTokens: c,
		TurnPrompt:       tp,
		TurnCompletion:   tc,
		ContextTokens:    ag.ContextTokens(),
		CompactCount:     compactCount,
		Duration:         d,
	})
}
