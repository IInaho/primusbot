package defaultbot

import (
	"nekocode/bot"
	commonview "nekocode/common/view"
	controlruntime "nekocode/runtime"
)

// runnerBot is the slice of *bot.Bot the adapter needs. Every other runtime
// port is satisfied by *bot.Bot directly: the runtime port types are aliases
// of common/view (via runtime/internal/core), and bot.Bot's methods use the
// same common/view types (via bot/view), so the signatures are identical.
type runnerBot interface {
	Run(input string, callbacks commonview.RunCallbacks) (string, error)
	ConfigureRuntime(callbacks commonview.ControlCallbacks)
	Steer(msg string)
	Abort()
	Close()
}

// adapter exists only where forwarding alone is not enough: it bridges the
// bot's confirm channel into the runtime's confirm channel and stops the
// bridge goroutine on reconfigure or close.
type adapter struct {
	bot           runnerBot
	confirmChStop chan struct{}
}

func coreOptions(b *bot.Bot) controlruntime.CoreSessionRuntimeOptions {
	a := &adapter{bot: b}
	return controlruntime.CoreSessionRuntimeOptions{
		Runner:            a,
		Control:           a,
		Commands:          b,
		Skills:            b,
		Catalog:           b,
		Stats:             b,
		Model:             b,
		Messages:          b,
		ModelManagement:   b,
		ContextManagement: b,
		SkillManagement:   b,
		ConfigManagement:  b,
		SessionManagement: b,
	}
}

func (a adapter) Run(input string, callbacks controlruntime.RunCallbacks) (string, error) {
	return a.bot.Run(input, callbacks)
}

func (a adapter) Steer(msg string) { a.bot.Steer(msg) }
func (a adapter) Abort()           { a.bot.Abort() }

func (a *adapter) ConfigureRuntime(callbacks controlruntime.ControlCallbacks) {
	if a.confirmChStop != nil {
		close(a.confirmChStop)
	}
	a.confirmChStop = make(chan struct{})
	viewCallbacks := commonview.ControlCallbacks(callbacks)
	if callbacks.ConfirmCh != nil {
		viewCallbacks.ConfirmCh = make(chan commonview.ConfirmRequest, 1)
		go a.bridgeConfirmCh(callbacks.ConfirmCh, viewCallbacks.ConfirmCh, a.confirmChStop)
	}
	a.bot.ConfigureRuntime(viewCallbacks)
}

func (a *adapter) bridgeConfirmCh(coreCh chan commonview.ConfirmRequest, viewCh chan commonview.ConfirmRequest, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case req, ok := <-viewCh:
			if !ok {
				return
			}
			if req.Response != nil {
				// Runtime handles approvals via the synchronous Confirm callback.
				// Non-nil Response on ConfirmCh is not expected; ignore to avoid
				// leaking goroutines waiting for a reply.
				continue
			}
			select {
			case <-stop:
				return
			case coreCh <- req:
			}
		}
	}
}

func (a *adapter) Close() {
	if a.confirmChStop != nil {
		close(a.confirmChStop)
	}
	a.bot.Close()
}
