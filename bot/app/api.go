package app

import (
	"nekocode/bot/command"
	"nekocode/bot/config"
	"nekocode/bot/view"
)

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

func (b *Bot) ContextStatus() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return command.ContextStats(b.ctxMgr)
}

func (b *Bot) ContextReport() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return command.ContextReport(b.ctxMgr, b.toolbox.Registry.Descriptors())
}

func (b *Bot) ContextSnapshot() view.ContextSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()

	r := b.ctxMgr.Report()
	r.ToolDefCount = len(b.toolbox.Registry.Descriptors())
	r.ToolDefTokens = command.EstimateToolDefTokens(b.toolbox.Registry.Descriptors())
	return view.NewContextSnapshot(r)
}
