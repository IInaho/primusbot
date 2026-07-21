package app

import (
	"sync"

	"nekocode/bot/agent/runtime"
	"nekocode/bot/config"
	"nekocode/bot/extension/plugin"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/bot/view"
)

type callbackBus struct {
	confirmFn  view.ConfirmFunc
	phaseFn    view.PhaseFunc
	todoFn     view.TodoFunc
	notifyFn   func(string)
	confirmCh  chan view.ConfirmRequest
	questionFn view.QuestionFunc

	// Permission policy source: the config-declared allow/ask/deny rules
	// plus workspace/home for path-anchor resolution. Injected by the Bot
	// at startup; nil still uses builtin permission rules.
	policyCfg *config.PermissionsConfig
	cwd       string
	home      string

	confirmMu      sync.Mutex
	pendingConfirm bool
}

func (c *callbackBus) Configure(confirmFn view.ConfirmFunc, phaseFn view.PhaseFunc, todoFn view.TodoFunc, notifyFn func(string), confirmCh chan view.ConfirmRequest, questionFn view.QuestionFunc) {
	c.ConfigureRuntime(view.ControlCallbacks{
		Confirm:   confirmFn,
		Phase:     phaseFn,
		Todo:      todoFn,
		Notify:    notifyFn,
		ConfirmCh: confirmCh,
		Question:  questionFn,
	})
}

func (c *callbackBus) ConfigureRuntime(callbacks view.ControlCallbacks) {
	c.confirmFn = callbacks.Confirm
	c.phaseFn = callbacks.Phase
	c.todoFn = callbacks.Todo
	c.notifyFn = callbacks.Notify
	c.confirmCh = callbacks.ConfirmCh
	c.questionFn = callbacks.Question
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
		Allow:   p.Allow,
		Ask:     p.Ask,
		Deny:    p.Deny,
		Sandbox: toSandboxDecl(p.Sandbox),
	}
}

func toSandboxDecl(in map[string]config.SandboxConfig) map[string]permission.SandboxProfile {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]permission.SandboxProfile, len(in))
	for rule, profile := range in {
		out[rule] = permission.SandboxProfile{
			SandboxMode:   profile.SandboxMode,
			Network:       profile.Network,
			WritableRoots: append([]string(nil), profile.WritableRoots...),
		}
	}
	return out
}

func (c *callbackBus) todoWriter() func([]view.TodoItem) {
	return func(items []view.TodoItem) {
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
		case c.confirmCh <- view.ConfirmRequest{Response: nil}:
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
	result := c.confirmFn(view.NewConfirmRequest("/plugin install", map[string]any{"source": source, "summary": summary}, view.ConfirmKindInstall))
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

func (b *Bot) Configure(confirmFn view.ConfirmFunc, phaseFn view.PhaseFunc, todoFn view.TodoFunc, notifyFn func(string), confirmCh chan view.ConfirmRequest, questionFn view.QuestionFunc) {
	b.cb.Configure(confirmFn, phaseFn, todoFn, notifyFn, confirmCh, questionFn)
	b.applyCallbacks()
}

func (b *Bot) ConfigureRuntime(callbacks view.ControlCallbacks) {
	b.cb.ConfigureRuntime(callbacks)
	b.applyCallbacks()
}

func (b *Bot) SetCallbacks(textFn, reasonFn func(string)) {
	setAgentStreams(b.getAgent(), textFn, reasonFn)
}
