package extension

import (
	"context"
	"fmt"

	"nekocode/bot/command"
	"nekocode/bot/extension/plugin"
)

type InstallConfirm func(source string, plugin *plugin.Plugin, remote bool) bool

// RegisterCommands installs the /plugin command family.
func (m *Manager) RegisterCommands(handler *command.Handler, confirm InstallConfirm) {
	m.mu.Lock()
	m.commands = handler
	m.syncSkillCommandsLocked()
	m.mu.Unlock()
	p := handler.Parser()
	p.Register("plugin", func(ctx context.Context, cmd *command.Command) (string, bool) {
		if len(cmd.Args) == 0 {
			return plugin.Usage(), true
		}
		switch cmd.Args[0] {
		case "install":
			return m.installPlugin(ctx, cmd.Args[1:], confirm), true
		case "uninstall":
			return m.uninstallPlugin(ctx, cmd.Args[1:]), true
		case "list":
			return m.listPlugins(), true
		case "enable":
			return m.enablePlugin(ctx, cmd.Args[1:], true), true
		case "disable":
			return m.enablePlugin(ctx, cmd.Args[1:], false), true
		case "info":
			return m.pluginInfo(cmd.Args[1:]), true
		default:
			return fmt.Sprintf("Unknown subcommand: %s\n%s", cmd.Args[0], plugin.Usage()), true
		}
	})
}

func (m *Manager) installPlugin(ctx context.Context, args []string, confirm InstallConfirm) string {
	if len(args) == 0 {
		return plugin.InstallUsage
	}
	source := args[0]
	confirmed := len(args) >= 2 && args[1] == "--yes"
	if confirmed {
		return m.install(ctx, source)
	}

	if !plugin.IsLocalSource(source) {
		return m.fetchAndConfirmRemote(ctx, source, confirm)
	}

	p, remote, err := m.plugins.Preview(ctx, source)
	if err != nil {
		return fmt.Sprintf("Preview failed: %v", err)
	}
	if confirm == nil || !confirm(source, p, remote) {
		return "Install cancelled: " + source
	}
	return m.install(ctx, source)
}

func (m *Manager) fetchAndConfirmRemote(ctx context.Context, source string, confirm InstallConfirm) string {
	p, remote, err := m.plugins.Preview(ctx, source)
	if err != nil {
		return fmt.Sprintf("%v\n\n/plugin install %s --yes to skip preview.", err, source)
	}
	if confirm == nil || !confirm(source, p, remote) {
		return "Install cancelled: " + source
	}
	return m.install(ctx, source)
}

func (m *Manager) uninstallPlugin(ctx context.Context, args []string) string {
	if len(args) == 0 {
		return "Usage: /plugin uninstall <name>"
	}
	if err := ctx.Err(); err != nil {
		return "Uninstall cancelled: " + err.Error()
	}

	m.ops.Lock()
	defer m.ops.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	name := args[0]
	p, ok := m.plugins.Get(name)
	if !ok {
		return fmt.Sprintf("Uninstall failed: plugin %q not found", name)
	}
	if p.Enabled {
		m.deactivateLocked(p.Name)
	}
	if _, err := m.plugins.Remove(name); err != nil {
		if p.Enabled {
			_ = m.activateLocked(context.Background(), p)
		}
		return fmt.Sprintf("Uninstall failed: %v", err)
	}
	m.skills.Reload(m.plugins.SkillDirs())
	m.syncSkillCommandsLocked()
	return fmt.Sprintf("Uninstalled plugin %q.", name)
}

func (m *Manager) install(ctx context.Context, source string) string {
	m.ops.Lock()
	defer m.ops.Unlock()
	installation, err := m.plugins.PrepareInstall(ctx, source)
	if err != nil {
		return fmt.Sprintf("Install failed: %v", err)
	}
	p := installation.Plugin()
	if err := ctx.Err(); err != nil {
		_ = installation.Rollback()
		return "Install cancelled: " + err.Error()
	}

	m.mu.Lock()
	previous, wasActive := m.active[p.Name]
	if wasActive {
		m.deactivateLocked(p.Name)
	}
	if p.Enabled {
		if err := m.activateLocked(ctx, p); err != nil {
			rollbackErr := installation.Rollback()
			if wasActive {
				_ = m.activateLocked(context.Background(), previous.plugin)
			}
			m.mu.Unlock()
			if rollbackErr != nil {
				return fmt.Sprintf("Install cancelled: %v (rollback failed: %v)", err, rollbackErr)
			}
			return "Install cancelled: " + err.Error()
		}
	}
	if err := installation.Commit(); err != nil {
		m.deactivateLocked(p.Name)
		if wasActive {
			_ = m.activateLocked(context.Background(), previous.plugin)
		}
		m.mu.Unlock()
		return "Install failed: " + err.Error()
	}
	m.skills.Reload(m.plugins.SkillDirs())
	m.syncSkillCommandsLocked()
	m.mu.Unlock()

	return plugin.InstallResult(p)
}

func (m *Manager) listPlugins() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.plugins.ListText()
}

func (m *Manager) pluginInfo(args []string) string {
	if len(args) == 0 {
		return "Usage: /plugin info <name>"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	info, ok := m.plugins.InfoText(args[0])
	if !ok {
		return fmt.Sprintf("Plugin %q not found.", args[0])
	}
	return info
}

func (m *Manager) enablePlugin(ctx context.Context, args []string, enabled bool) string {
	if len(args) == 0 {
		if enabled {
			return "Usage: /plugin enable <name>"
		}
		return "Usage: /plugin disable <name>"
	}
	name := args[0]
	changed, err := m.setPluginEnabled(ctx, name, enabled)
	if err != nil {
		action := "Enable"
		if !enabled {
			action = "Disable"
		}
		return fmt.Sprintf("%s failed: %v", action, err)
	}
	if !changed {
		state := "disabled"
		if enabled {
			state = "enabled"
		}
		return fmt.Sprintf("Plugin %q is already %s.", name, state)
	}
	if enabled {
		return fmt.Sprintf("Enabled plugin %q.", name)
	}
	return fmt.Sprintf("Disabled plugin %q.", name)
}
