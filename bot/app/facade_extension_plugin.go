package app

import (
	"fmt"

	"nekocode/bot/agent/subagent"
	"nekocode/bot/command"
	"nekocode/bot/extension/plugin"
	"nekocode/bot/view"
	"nekocode/common/debug"
)

func (e *extensionFacade) InitPlugins() {
	e.applyMCPTools(nil, e.mcp.CloseAll())
	e.plugins = plugin.NewManager(plugin.ManagerOptions{
		Hooks: e.hookReg,
		Logf:  debug.Log,
		OnInstall: func(p *plugin.Plugin) {
			if e.skills != nil {
				e.skills.LoadPluginSkillDirs(p.SkillDirs())
			}
		},
		OnChanged:           e.RefreshPluginSkills,
		RegisterAgentPath:   e.registerPluginAgentPath,
		UnregisterAgentPath: e.unregisterPluginAgentPath,
		RegisterMCPServer:   e.registerPluginMCPServer,
		UnregisterMCPServer: e.unregisterPluginMCPServer,
	})
	e.plugins.LoadAll()
}

func (e *extensionFacade) RegisterPluginCommands(p *command.Parser, callbacks plugin.InstallCallbacks) {
	p.Register("plugin", func(cmd *command.Command) (string, bool) {
		if len(cmd.Args) == 0 {
			return plugin.Usage(), true
		}
		switch cmd.Args[0] {
		case "install":
			return e.plugins.Install(cmd.Args[1:], callbacks), true
		case "uninstall":
			return e.plugins.Uninstall(cmd.Args[1:]), true
		case "list":
			return e.plugins.List(cmd.Args[1:]), true
		case "enable":
			return e.plugins.Enable(cmd.Args[1:]), true
		case "disable":
			return e.plugins.Disable(cmd.Args[1:]), true
		case "info":
			return e.plugins.Info(cmd.Args[1:]), true
		default:
			return fmt.Sprintf("Unknown subcommand: %s\n%s", cmd.Args[0], plugin.Usage()), true
		}
	})
}

func (e *extensionFacade) registerPluginAgentPath(path string) error {
	def, err := subagent.ParseAgentMD(path)
	if err != nil {
		return err
	}
	subagent.RegisterPlugin(def.ToAgentType())
	return nil
}

func (e *extensionFacade) unregisterPluginAgentPath(path string) {
	def, err := subagent.ParseAgentMD(path)
	if err == nil {
		subagent.UnregisterPlugin(def.Name)
	}
}

// --- 插件安装确认 ---
//
// 安装流程的交互适配：把 callbackBus 的通用确认/通知回调组装成
// plugin.InstallCallbacks。状态（pendingConfirm/confirmCh）仍由 bus 持有，
// 这里只做插件语义的翻译。

// ConfirmInstall asks the user to confirm a plugin install, showing the
// plugin preview summary. Cancelled installs notify and return false.
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

// InstallCallbacks assembles the plugin manager's install interaction points
// from the bus's generic callbacks.
func (c *callbackBus) InstallCallbacks() plugin.InstallCallbacks {
	return plugin.InstallCallbacks{
		Confirm:    c.ConfirmInstall,
		Notify:     c.notifyFn,
		SetPending: c.setPendingConfirmation,
		Unblock:    c.UnblockConfirm,
	}
}
