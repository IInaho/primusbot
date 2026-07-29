// Package plugin manages installed plugins: discovery from plugin
// directories, install/uninstall, enable/disable state, and wiring of
// plugin-provided extensions (skills, agents, hooks, MCP servers) into the
// host application.
//
// Manager is the package entry point; the registry, install pipeline and
// extension runtime are internal implementation details.
package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nekocode/bot/policy"
	"nekocode/common/debug"
)

// Plugin represents an installed plugin instance.
type Plugin struct {
	Manifest
	Dir            string
	Source         string
	Enabled        bool
	InstalledAt    time.Time
	HasInstallStub bool
}

// SkillDirs returns the absolute skill directories for this plugin.
func (p *Plugin) SkillDirs() []string {
	if len(p.Manifest.Skills) == 0 {
		return p.autoDiscoverSkills()
	}
	dirs := make([]string, 0, len(p.Manifest.Skills))
	for _, s := range p.Manifest.Skills {
		dirs = append(dirs, resolvePath(p.Dir, s))
	}
	return dirs
}

// AgentPaths returns agent .md file paths declared or auto-discovered.
func (p *Plugin) AgentPaths() []string {
	if len(p.Manifest.Agents) > 0 {
		var paths []string
		for _, a := range p.Manifest.Agents {
			paths = append(paths, resolvePath(p.Dir, a))
		}
		return paths
	}
	return p.autoDiscoverAgents()
}

// HooksPath returns the hooks.json path and whether it exists.
func (p *Plugin) HooksPath() (string, bool) {
	if p.Manifest.Hooks != nil && p.Manifest.Hooks.Source != "" {
		return p.Manifest.Hooks.Source, true
	}
	return p.autoDiscoverHooks()
}

// MCPServers returns MCP server configs declared or auto-discovered.
func (p *Plugin) MCPServers() map[string]MCPServerConfig {
	if len(p.Manifest.MCPServers) > 0 {
		return p.Manifest.MCPServers
	}
	return p.autoDiscoverMCP()
}

// --- Auto-discovery ---
//
// When the manifest does not declare an extension explicitly, these methods
// find it by walking the plugin directory.

func (p *Plugin) autoDiscoverSkills() []string {
	return walkFind(p.Dir, "skills", true)
}

func (p *Plugin) autoDiscoverAgents() []string {
	var paths []string
	dirs := walkFind(p.Dir, "agents", true)
	for _, dir := range dirs {
		ents, _ := os.ReadDir(dir)
		for _, e := range ents {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				paths = append(paths, filepath.Join(dir, e.Name()))
			}
		}
	}
	return paths
}

func (p *Plugin) autoDiscoverHooks() (string, bool) {
	found := walkFind(p.Dir, "hooks.json", false)
	if len(found) > 0 {
		rel, _ := filepath.Rel(p.Dir, found[0])
		return rel, true
	}
	return "", false
}

func (p *Plugin) autoDiscoverMCP() map[string]MCPServerConfig {
	found := walkFind(p.Dir, ".mcp.json", false)
	for _, path := range found {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg struct {
			MCPServers map[string]MCPServerConfig `json:"mcpServers"`
		}
		if json.Unmarshal(data, &cfg) == nil && len(cfg.MCPServers) > 0 {
			return cfg.MCPServers
		}
	}
	return nil
}

// walkFind returns the paths under root whose base name matches matchName
// (case-insensitive). matchDir selects whether to match directories or
// files; the first match stops the descent into that subtree.
func walkFind(root string, matchName string, matchDir bool) []string {
	var results []string
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if matchDir && !d.IsDir() {
			return nil
		}
		if !matchDir && d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), matchName) {
			results = append(results, path)
			if matchDir {
				return filepath.SkipDir
			}
			return filepath.SkipAll
		}
		return nil
	})
	return results
}

func resolvePath(base, rel string) string {
	rel = strings.TrimPrefix(rel, "./")
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(base, rel)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Manager owns the plugin lifecycle: loading from disk, install/uninstall,
// enable/disable, and wiring plugin extensions into the host. Construct it
// with NewManager and call LoadAll once the surrounding wiring is in place.
type Manager struct {
	reg           *registry
	pluginRuntime pluginRuntime
	onInstall     func(*Plugin)
	onChanged     func()
}

type ManagerOptions struct {
	Hooks               *policy.Registry
	Logf                func(string, ...any)
	OnInstall           func(*Plugin)
	OnChanged           func()
	RegisterAgentPath   func(path string) error
	UnregisterAgentPath func(path string)
	RegisterMCPServer   func(pluginDir, name string, cfg MCPServerConfig) error
	UnregisterMCPServer func(name string)
}

func NewManager(opts ManagerOptions) *Manager {
	reg := newRegistry(defaultDirs())
	reg.Logf = opts.Logf
	if reg.Logf == nil {
		reg.Logf = debug.Log
	}
	return &Manager{
		reg:       reg,
		onInstall: opts.OnInstall,
		onChanged: opts.OnChanged,
		pluginRuntime: pluginRuntime{
			Hooks:               opts.Hooks,
			Logf:                reg.Logf,
			RegisterAgentPath:   opts.RegisterAgentPath,
			UnregisterAgentPath: opts.UnregisterAgentPath,
			RegisterMCPServer:   opts.RegisterMCPServer,
			UnregisterMCPServer: opts.UnregisterMCPServer,
		},
	}
}

// LoadAll scans the plugin directories and wires every enabled plugin's
// extensions into the host.
func (m *Manager) LoadAll() {
	m.reg.LoadAll()
	for _, p := range m.reg.List() {
		if p.Enabled {
			m.loadExtensions(p)
		}
	}
}

// Reload rescans the plugin directories and fires the OnChanged callback.
func (m *Manager) Reload() {
	m.reg.LoadAll()
	m.notifyChanged()
}

// ListPlugins returns all installed plugins sorted by name.
func (m *Manager) ListPlugins() []*Plugin {
	return m.reg.List()
}

// SkillDirs returns all skill directories from enabled plugins.
func (m *Manager) SkillDirs() []string {
	return m.reg.SkillDirs()
}

func (m *Manager) notifyChanged() {
	if m.onChanged != nil {
		m.onChanged()
	}
}

func (m *Manager) loadExtensions(p *Plugin) {
	m.pluginRuntime.Load(p)
}

func (m *Manager) unloadExtensions(p *Plugin) {
	m.pluginRuntime.Unload(p)
}
