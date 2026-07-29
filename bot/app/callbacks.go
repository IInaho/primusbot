package app

import (
	"sync"

	"nekocode/bot/agent/runtime"
	"nekocode/bot/config"
	"nekocode/bot/extension"
	"nekocode/bot/extension/plugin"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/bot/view"
)

// callbackBus 是 Bot 与外界交互的回调注册表：UI 层经 ConfigureRuntime 注入
// 回调集，bus 在 agent 构建时将其灌入 agent（applyAgentControlCallbacksTo），
// 并持有异步命令确认状态。插件安装确认也在这里适配为
// extension.InstallCallbacks。
type callbackBus struct {
	confirmFn     view.ConfirmFunc
	phaseFn       view.PhaseFunc
	todoFn        view.TodoFunc
	notifyFn      func(string)
	commandDoneFn func()
	questionFn    view.QuestionFunc

	// Permission policy source: the config-declared allow/ask/deny rules
	// plus workspace/home for path-anchor resolution. Injected by the Bot
	// at startup; nil still uses builtin permission rules.
	policyCfg *config.PermissionsConfig
	cwd       string
	home      string

	confirmMu      sync.Mutex
	pendingConfirm bool
}

func (c *callbackBus) configure(callbacks view.ControlCallbacks) {
	c.confirmFn = callbacks.Confirm
	c.phaseFn = callbacks.Phase
	c.todoFn = callbacks.Todo
	c.notifyFn = callbacks.Notify
	c.commandDoneFn = callbacks.CommandDone
	c.questionFn = callbacks.Question
}

func (c *callbackBus) applyTo(ag *runtime.Agent) {
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

func (b *Bot) ConfigureRuntime(callbacks view.ControlCallbacks) {
	b.cb.configure(callbacks)
	b.applyCallbacks()
}

func (b *Bot) SetCallbacks(textFn, reasonFn func(string)) {
	ag := b.getAgent()
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

func (c *callbackBus) confirmInstall(source string, p *plugin.Plugin, isRemote bool) bool {
	if c.confirmFn == nil {
		return false
	}
	summary := plugin.ConfirmSummary(p, isRemote)
	result := c.confirmFn(view.NewConfirmRequest(
		"/plugin install",
		map[string]any{"source": source, "summary": summary},
		view.ConfirmKindInstall,
	))
	if !result.Allowed && c.notifyFn != nil {
		c.notifyFn("Install cancelled: " + source)
	}
	return result.Allowed
}

func (c *callbackBus) installCallbacks() extension.InstallCallbacks {
	return extension.InstallCallbacks{
		Confirm:    c.confirmInstall,
		Notify:     c.notifyFn,
		SetPending: c.setPendingConfirmation,
		Done: func() {
			c.setPendingConfirmation(false)
			if c.commandDoneFn != nil {
				c.commandDoneFn()
			}
		},
	}
}
