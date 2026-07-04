package app

import (
	"sync"

	"nekocode/bot/agent/runtime"
	"nekocode/bot/config"
	"nekocode/bot/extension/plugin"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/common"
)

type callbackBus struct {
	confirmFn  common.ConfirmFunc
	phaseFn    common.PhaseFunc
	todoFn     common.TodoFunc
	notifyFn   func(string)
	confirmCh  chan common.ConfirmRequest
	questionFn common.QuestionFunc

	// Permission policy source: the config-declared allow/ask/deny rules
	// plus workspace/home for path-anchor resolution. Injected by the Bot
	// at startup; nil still uses builtin permission rules.
	policyCfg *config.PermissionsConfig
	cwd       string
	home      string

	confirmMu      sync.Mutex
	pendingConfirm bool
}

func (c *callbackBus) Configure(confirmFn common.ConfirmFunc, phaseFn common.PhaseFunc, todoFn common.TodoFunc, notifyFn func(string), confirmCh chan common.ConfirmRequest, questionFn common.QuestionFunc) {
	c.confirmFn = confirmFn
	c.phaseFn = phaseFn
	c.todoFn = todoFn
	c.notifyFn = notifyFn
	c.confirmCh = confirmCh
	c.questionFn = questionFn
}

func (c *callbackBus) applyAgentControlCallbacksTo(ag *runtime.Agent) {
	if ag == nil {
		return
	}
	ag.SetConfirmFn(c.confirmFn)
	ag.SetPhaseFn(c.phaseFn)
	ag.SetProjectStore(c.cwd)
	ag.SetPermissionPolicy(toPermDecl(c.policyCfg), c.cwd, c.home)
}

// toPermDecl converts config.PermissionsConfig to the permission.PermissionsDecl
// used by the engine (decoupled from the config package to avoid an import cycle).
func toPermDecl(p *config.PermissionsConfig) permission.PermissionsDecl {
	if p == nil {
		return permission.PermissionsDecl{}
	}
	return permission.PermissionsDecl{
		Allow: p.Allow,
		Ask:   p.Ask,
		Deny:  p.Deny,
	}
}

func (c *callbackBus) todoWriter() func([]common.TodoItem) {
	return func(items []common.TodoItem) {
		if c.todoFn != nil {
			c.todoFn(items)
		}
	}
}

func setAgentStreams(ag *runtime.Agent, textFn, reasonFn func(string)) {
	if ag == nil {
		return
	}
	if textFn != nil {
		ag.SetStreamFn(func(delta string, _ bool) { textFn(delta) })
	}
	if reasonFn != nil {
		ag.SetReasoningStreamFn(reasonFn)
	}
}

func (c *callbackBus) pendingConfirmation() bool {
	c.confirmMu.Lock()
	defer c.confirmMu.Unlock()
	return c.pendingConfirm
}

func (c *callbackBus) setPendingConfirmation(pending bool) {
	c.confirmMu.Lock()
	c.pendingConfirm = pending
	c.confirmMu.Unlock()
}

func (c *callbackBus) UnblockConfirm() {
	c.setPendingConfirmation(false)
	if c.confirmCh != nil {
		select {
		case c.confirmCh <- common.ConfirmRequest{Response: nil}:
		default:
		}
	}
}

func (c *callbackBus) ConfirmInstall(source string, p *plugin.Plugin, isRemote bool) bool {
	summary := plugin.ConfirmSummary(p, isRemote)
	if c.confirmFn == nil {
		c.UnblockConfirm()
		return false
	}
	result := c.confirmFn(common.NewConfirmRequest("/plugin install", map[string]any{"source": source, "summary": summary}, common.ConfirmKindInstall))
	c.setPendingConfirmation(false)
	if !result.Allowed && c.notifyFn != nil {
		c.notifyFn("Install cancelled: " + source)
	}
	return result.Allowed
}

func (c *callbackBus) InstallCallbacks() plugin.InstallCallbacks {
	return plugin.InstallCallbacks{
		Confirm:    c.ConfirmInstall,
		Notify:     c.notifyFn,
		SetPending: c.setPendingConfirmation,
		Unblock:    c.UnblockConfirm,
	}
}

func (b *Bot) Configure(confirmFn common.ConfirmFunc, phaseFn common.PhaseFunc, todoFn common.TodoFunc, notifyFn func(string), confirmCh chan common.ConfirmRequest, questionFn common.QuestionFunc) {
	b.cb.Configure(confirmFn, phaseFn, todoFn, notifyFn, confirmCh, questionFn)
	b.applyCallbacks()
}

func (b *Bot) SetCallbacks(textFn, reasonFn func(string)) {
	setAgentStreams(b.getAgent(), textFn, reasonFn)
}
