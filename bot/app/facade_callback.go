package app

import (
	"sync"

	"nekocode/bot/agent/runtime"
	"nekocode/bot/config"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/bot/view"
)

// callbackBus 是 Bot 与外界交互的回调注册表：UI 层经 ConfigureRuntime 注入
// 回调集，bus 在 agent 构建时将其灌入 agent（applyAgentControlCallbacksTo），
// 并持有确认流程的共享状态（pendingConfirm/confirmCh）。插件安装确认的
// 适配在 facade_extension_plugin.go。
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
	ag.SetPhaseFn(c.phaseFn)
	ag.Executor().SetConfirmFn(c.confirmFn)
	ag.Executor().SetProjectStore(c.cwd)
	ag.Executor().SetPermissionPolicy(toPermDecl(c.policyCfg), c.cwd, c.home)
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

func (b *Bot) ConfigureRuntime(callbacks view.ControlCallbacks) {
	b.cb.ConfigureRuntime(callbacks)
	b.applyCallbacks()
}

func (b *Bot) SetCallbacks(textFn, reasonFn func(string)) {
	setAgentStreams(b.getAgent(), textFn, reasonFn)
}
