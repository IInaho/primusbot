package app

import (
	"fmt"

	"nekocode/bot/agent/runtime"
	"nekocode/bot/command"
	"nekocode/bot/view"
	commonview "nekocode/common/view"
)

// api_agent.go — Bot API：agent 运行与内省（运行、打断、模型切换、token/时长统计）。

func (b *Bot) Steer(msg string) { b.getAgent().Steer(msg) }
func (b *Bot) Abort()           { b.getAgent().Abort() }

func (b *Bot) ProviderModel() (string, string) {
	am := b.cfg.ActiveModelConfig()
	return am.Provider, am.Model
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
	ag.Executor().SetPlanMode(false)
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
