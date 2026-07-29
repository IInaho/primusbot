package extension

import (
	"fmt"

	"nekocode/bot/command"
	"nekocode/bot/extension/plugin"
)

// InstallCallbacks is the UI boundary for asynchronous plugin installation.
// Runtime activation itself does not use callbacks.
type InstallCallbacks struct {
	Confirm    func(source string, p *plugin.Plugin, isRemote bool) bool
	Notify     func(string)
	SetPending func(bool)
	Done       func()
}

func (c InstallCallbacks) normalized() InstallCallbacks {
	if c.Notify == nil {
		c.Notify = func(string) {}
	}
	if c.SetPending == nil {
		c.SetPending = func(bool) {}
	}
	if c.Done == nil {
		c.Done = func() {}
	}
	if c.Confirm == nil {
		c.Confirm = func(string, *plugin.Plugin, bool) bool { return false }
	}
	return c
}

// RegisterCommands installs the /plugin command family.
func (m *Manager) RegisterCommands(p *command.Parser, callbacks InstallCallbacks) {
	callbacks = callbacks.normalized()
	p.Register("plugin", func(cmd *command.Command) (string, bool) {
		if len(cmd.Args) == 0 {
			return plugin.Usage(), true
		}
		switch cmd.Args[0] {
		case "install":
			return m.installPlugin(cmd.Args[1:], callbacks), true
		case "uninstall":
			return m.uninstallPlugin(cmd.Args[1:]), true
		case "list":
			return m.listPlugins(), true
		case "enable":
			return m.enablePlugin(cmd.Args[1:], true), true
		case "disable":
			return m.enablePlugin(cmd.Args[1:], false), true
		case "info":
			return m.pluginInfo(cmd.Args[1:]), true
		default:
			return fmt.Sprintf("Unknown subcommand: %s\n%s", cmd.Args[0], plugin.Usage()), true
		}
	})
}

func (m *Manager) installPlugin(args []string, cb InstallCallbacks) string {
	cb = cb.normalized()
	if len(args) == 0 {
		return plugin.InstallUsage
	}
	source := args[0]
	confirmed := len(args) >= 2 && args[1] == "--yes"
	if confirmed {
		cb.SetPending(true)
		go func() {
			defer cb.Done()
			m.install(source, cb)
		}()
		return fmt.Sprintf("Installing from %s ...", source)
	}

	if !plugin.IsLocalSource(source) {
		cb.SetPending(true)
		go func() {
			defer cb.Done()
			m.fetchAndConfirmRemote(source, cb)
		}()
		return fmt.Sprintf("Fetching plugin info from %s ...", source)
	}

	p, remote, err := m.plugins.Preview(source)
	if err != nil {
		return fmt.Sprintf("Preview failed: %v", err)
	}
	cb.SetPending(true)
	go func() {
		defer cb.Done()
		if cb.Confirm(source, p, remote) {
			m.install(source, cb)
		}
	}()
	return plugin.ConfirmSummary(p, remote)
}

func (m *Manager) fetchAndConfirmRemote(source string, cb InstallCallbacks) {
	p, remote, err := m.plugins.Preview(source)
	if err != nil {
		cb.Notify(fmt.Sprintf("%v\n\n/plugin install %s --yes  to skip preview.", err, source))
		return
	}
	if cb.Confirm(source, p, remote) {
		m.install(source, cb)
	}
}

func (m *Manager) uninstallPlugin(args []string) string {
	if len(args) == 0 {
		return "Usage: /plugin uninstall <name>"
	}

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
			m.activateLocked(p)
		}
		return fmt.Sprintf("Uninstall failed: %v", err)
	}
	m.skills.Reload(m.plugins.SkillDirs())
	return fmt.Sprintf("Uninstalled plugin %q.", name)
}

func (m *Manager) install(source string, cb InstallCallbacks) {
	p, err := m.plugins.InstallSource(source)
	if err != nil {
		cb.Notify(fmt.Sprintf("Install failed: %v", err))
		return
	}

	m.mu.Lock()
	if _, ok := m.active[p.Name]; ok {
		m.deactivateLocked(p.Name)
	}
	if p.Enabled {
		m.activateLocked(p)
	}
	m.skills.Reload(m.plugins.SkillDirs())
	m.mu.Unlock()

	cb.Notify(plugin.InstallResult(p))
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

func (m *Manager) enablePlugin(args []string, enabled bool) string {
	if len(args) == 0 {
		if enabled {
			return "Usage: /plugin enable <name>"
		}
		return "Usage: /plugin disable <name>"
	}
	name := args[0]
	changed, err := m.setPluginEnabled(name, enabled)
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
