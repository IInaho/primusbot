package plugin

import "fmt"

// Usage returns the /plugin command usage text.
func Usage() string {
	return "Usage: /plugin <subcommand> [args]\n\nSubcommands:\n  install <source>   Install from GitHub URL, user/repo, or local path\n  uninstall <name>   Remove a plugin\n  list               List installed plugins\n  enable <name>      Enable a disabled plugin\n  disable <name>     Disable a plugin (keeps files)\n  info <name>        Show plugin details"
}

const installUsage = "Usage: /plugin install <source>\n  source: GitHub URL | user/repo | ./local-path"

// InstallCallbacks wires the install flow's user interaction points
// (preview confirmation, async result notification) to the host UI.
type InstallCallbacks struct {
	Confirm    func(source string, p *Plugin, isRemote bool) bool
	Notify     func(string)
	SetPending func(bool)
	Unblock    func()
}

type installArgs struct {
	Source    string
	Confirmed bool
	OK        bool
}

type lookupResult struct {
	Plugin  *Plugin
	Message string
	OK      bool
}

func (m *Manager) Install(args []string, cb InstallCallbacks) string {
	parsed := parseInstallArgs(args)
	if !parsed.OK {
		return installUsage
	}
	source := parsed.Source

	if isLocalPath(source) {
		return m.installLocal(source, parsed.Confirmed, cb)
	}
	if !parsed.Confirmed {
		if cb.SetPending != nil {
			cb.SetPending(true)
		}
		go m.fetchAndConfirmRemote(source, cb)
		return fmt.Sprintf("Fetching plugin info from %s ...", source)
	}

	go m.installAsync(source, cb)
	return fmt.Sprintf("Installing from %s ...", source)
}

func (m *Manager) Uninstall(args []string) string {
	if len(args) == 0 {
		return "Usage: /plugin uninstall <name>"
	}
	name := args[0]
	if p, ok := m.reg.Get(name); ok {
		m.unloadExtensions(p)
	}
	if err := m.reg.Uninstall(name); err != nil {
		return uninstallFailed(err)
	}
	m.notifyChanged()
	return uninstalled(name)
}

func (m *Manager) List(args []string) string {
	return formatList(m.reg.List())
}

func (m *Manager) Enable(args []string) string {
	lookup := requirePlugin(args, m.reg.Get, "Usage: /plugin enable <name>")
	if !lookup.OK {
		return lookup.Message
	}
	if lookup.Plugin.Enabled {
		return alreadyEnabled(lookup.Plugin.Name)
	}
	if _, err := m.setEnabled(lookup.Plugin, true); err != nil {
		return enableFailed(err)
	}
	return enabled(lookup.Plugin.Name)
}

func (m *Manager) Disable(args []string) string {
	lookup := requirePlugin(args, m.reg.Get, "Usage: /plugin disable <name>")
	if !lookup.OK {
		return lookup.Message
	}
	if !lookup.Plugin.Enabled {
		return alreadyDisabled(lookup.Plugin.Name)
	}
	if _, err := m.setEnabled(lookup.Plugin, false); err != nil {
		return disableFailed(err)
	}
	return disabled(lookup.Plugin.Name)
}

func (m *Manager) Info(args []string) string {
	lookup := requirePlugin(args, m.reg.Get, "Usage: /plugin info <name>")
	if !lookup.OK {
		return lookup.Message
	}
	return formatInfo(lookup.Plugin)
}

// SetEnabled flips a plugin's enabled state by name (API form of the
// /plugin enable|disable commands).
func (m *Manager) SetEnabled(name string, enable bool) (*Plugin, error) {
	p, ok := m.reg.Get(name)
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}
	return m.setEnabled(p, enable)
}

// setEnabled persists the new state, then wires or unwires the plugin's
// extensions accordingly.
func (m *Manager) setEnabled(p *Plugin, enable bool) (*Plugin, error) {
	if enable {
		if err := m.reg.Enable(p.Name); err != nil {
			return nil, err
		}
		if next, ok := m.reg.Get(p.Name); ok {
			m.loadExtensions(next)
			p = next
		}
	} else {
		if err := m.reg.Disable(p.Name); err != nil {
			return nil, err
		}
		m.unloadExtensions(p)
	}
	m.notifyChanged()
	return p, nil
}

func (m *Manager) installLocal(source string, confirmed bool, cb InstallCallbacks) string {
	p, err := m.reg.PreviewFromPath(source)
	if err != nil {
		return fmt.Sprintf("Preview failed: %v", err)
	}
	if confirmed {
		return m.installSync(source)
	}

	if cb.SetPending != nil {
		cb.SetPending(true)
	}
	go func() {
		if cb.Confirm != nil && cb.Confirm(source, p, false) {
			result := m.installSync(source)
			if cb.Notify != nil {
				cb.Notify(result)
			}
		}
	}()
	return formatInstallPreview(p)
}

func (m *Manager) fetchAndConfirmRemote(source string, cb InstallCallbacks) {
	p, err := fetchRemotePreview(source, fetchURL)
	if err != nil {
		if cb.Notify != nil {
			cb.Notify(fmt.Sprintf("%v\n\n/plugin install %s --yes  to skip preview.", err, source))
		}
		if cb.Unblock != nil {
			cb.Unblock()
		}
		return
	}
	if cb.Confirm != nil && cb.Confirm(source, p, true) {
		m.installAsync(source, cb)
	}
}

func (m *Manager) installSync(source string) string {
	p, err := m.reg.Install(source)
	if err != nil {
		return installFailed(err)
	}
	return m.registerExtensions(p)
}

func (m *Manager) installAsync(source string, cb InstallCallbacks) {
	p, err := m.reg.Install(source)
	if err != nil {
		if cb.Notify != nil {
			cb.Notify(installFailed(err))
		}
		return
	}
	result := m.registerExtensions(p)
	if cb.Notify != nil {
		cb.Notify(result)
	}
}

func (m *Manager) registerExtensions(p *Plugin) string {
	if m.onInstall != nil {
		m.onInstall(p)
	}
	m.loadExtensions(p)
	m.notifyChanged()
	return formatInstallResult(p)
}

func parseInstallArgs(args []string) installArgs {
	if len(args) == 0 {
		return installArgs{}
	}
	return installArgs{
		Source:    args[0],
		Confirmed: len(args) >= 2 && args[1] == "--yes",
		OK:        true,
	}
}

// fetchRemotePreview fetches a remote plugin's manifest for the install
// confirmation preview, without installing anything.
func fetchRemotePreview(source string, fetch func(string) ([]byte, error)) (*Plugin, error) {
	rawURL := sourceToRawURL(source)
	if rawURL == "" {
		return nil, fmt.Errorf("fetch plugin info: preview URL not available for %s", source)
	}
	data, err := fetch(rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch plugin info: %w", err)
	}
	m, err := parseManifestData(data)
	if err != nil {
		return nil, fmt.Errorf("invalid plugin.json: %w", err)
	}
	return &Plugin{Manifest: *m, Dir: "", Source: source}, nil
}

// ConfirmSummary renders the install confirmation preview; remote installs
// carry an extra note that install.sh is never executed automatically.
func ConfirmSummary(p *Plugin, isRemote bool) string {
	summary := formatInstallPreview(p)
	if isRemote {
		summary += "\n(install.sh will not be executed automatically)"
	}
	return summary
}

func requirePlugin(args []string, lookup func(string) (*Plugin, bool), usage string) lookupResult {
	if len(args) == 0 {
		return lookupResult{Message: usage}
	}
	p, ok := lookup(args[0])
	if !ok {
		return lookupResult{Message: fmt.Sprintf("Plugin %q not found.", args[0])}
	}
	return lookupResult{Plugin: p, OK: true}
}

func alreadyEnabled(name string) string  { return fmt.Sprintf("Plugin %q is already enabled.", name) }
func alreadyDisabled(name string) string { return fmt.Sprintf("Plugin %q is already disabled.", name) }
func enabled(name string) string         { return fmt.Sprintf("Enabled plugin %q.", name) }
func disabled(name string) string        { return fmt.Sprintf("Disabled plugin %q.", name) }
func uninstalled(name string) string     { return fmt.Sprintf("Uninstalled plugin %q.", name) }
func installFailed(err error) string     { return fmt.Sprintf("Install failed: %v", err) }
func uninstallFailed(err error) string   { return fmt.Sprintf("Uninstall failed: %v", err) }
func enableFailed(err error) string      { return fmt.Sprintf("Enable failed: %v", err) }
func disableFailed(err error) string     { return fmt.Sprintf("Disable failed: %v", err) }
