// Package extension composes plugins, skills, hooks, agents, and MCP servers
// into one runtime owned by the bot.
package extension

import (
	"fmt"
	"sync"

	"nekocode/bot/agent/subagent"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/extension/mcp"
	"nekocode/bot/extension/plugin"
	"nekocode/bot/extension/skill"
	"nekocode/bot/policy"
	"nekocode/bot/tools"
	"nekocode/common/debug"
)

// Manager is the public extension entry point. Child managers remain private
// so extension activation always follows one lifecycle.
type Manager struct {
	mu      sync.Mutex
	skills  *skill.Manager
	plugins *plugin.Manager
	mcp     *mcp.Manager
	policy  *policy.Policy
	active  map[string]activePlugin
}

// Snapshot is the read-only state used by management views.
type Snapshot struct {
	Skills       []*skill.Skill
	LoadedSkills map[string]bool
	Plugins      []*plugin.Plugin
	MCPHealth    map[string]mcp.Health
}

type activePlugin struct {
	plugin     *plugin.Plugin
	agentNames []string
	mcpIDs     []string
}

// New creates an extension runtime from its four concrete dependencies.
func New(ctx *ctxmgr.Manager, toolRegistry *tools.Registry, gov *policy.Policy, contextWindow int) *Manager {
	return &Manager{
		skills:  skill.New(ctx, toolRegistry, contextWindow),
		plugins: plugin.New(),
		mcp:     mcp.New(toolRegistry),
		policy:  gov,
		active:  make(map[string]activePlugin),
	}
}

// Load discovers plugins, activates enabled extensions, and loads the
// resulting skill set.
func (m *Manager) Load() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.plugins.Load()
	for _, p := range m.plugins.ListPlugins() {
		if p.Enabled {
			m.activateLocked(p)
		}
	}
	m.skills.Load(m.plugins.SkillDirs())
}

// Reload rebuilds plugin runtime state from disk and preserves loaded skills.
func (m *Manager) Reload() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deactivateAllLocked()
	m.plugins.Reload()
	for _, p := range m.plugins.ListPlugins() {
		if p.Enabled {
			m.activateLocked(p)
		}
	}
	m.skills.Reload(m.plugins.SkillDirs())
}

// Close stops plugin runtime extensions and every MCP process.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deactivateAllLocked()
	m.mcp.Close()
}

// Snapshot returns all management state under one lock.
func (m *Manager) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Snapshot{
		Skills:       m.skills.List(),
		LoadedSkills: m.skills.LoadedSet(),
		Plugins:      m.plugins.ListPlugins(),
		MCPHealth:    m.mcp.Health(),
	}
}

// Skill returns the compact command representation of one skill.
func (m *Manager) Skill(name string) (skill.Command, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sk, ok := m.skills.Get(name)
	if !ok {
		return skill.Command{}, false
	}
	return skill.Command{Name: sk.Name, Context: skill.FormatForContext(sk)}, true
}

// SkillCommands returns all skills available to command registration.
func (m *Manager) SkillCommands() []skill.Command {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.skills.List()
	out := make([]skill.Command, 0, len(list))
	for _, sk := range list {
		out = append(out, skill.Command{Name: sk.Name, Context: skill.FormatForContext(sk)})
	}
	return out
}

func (m *Manager) MarkSkillLoaded(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.skills.MarkLoaded(name)
}

func (m *Manager) ClearLoadedSkills() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.skills.ClearLoaded()
}

// SetPluginEnabled persists and applies one plugin state transition.
func (m *Manager) SetPluginEnabled(name string, enabled bool) error {
	_, err := m.setPluginEnabled(name, enabled)
	return err
}

func (m *Manager) setPluginEnabled(name string, enabled bool) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.plugins.Get(name)
	if !ok {
		return false, fmt.Errorf("plugin not found: %s", name)
	}
	if current.Enabled == enabled {
		return false, nil
	}
	next, err := m.plugins.SetEnabled(name, enabled)
	if err != nil {
		return false, err
	}
	if enabled {
		m.activateLocked(next)
	} else {
		m.deactivateLocked(name)
	}
	m.skills.Reload(m.plugins.SkillDirs())
	return true, nil
}

// AddMCPServer starts a host-configured MCP server.
func (m *Manager) AddMCPServer(name string, cfg mcp.ServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mcp.Add("config:"+name, name, cfg)
}

func (m *Manager) activateLocked(p *plugin.Plugin) {
	state := activePlugin{plugin: p}
	for _, path := range p.AgentPaths() {
		def, err := subagent.ParseAgentMD(path)
		if err != nil {
			debug.Log("plugin: agent %s: %v", path, err)
			continue
		}
		subagent.RegisterPlugin(def.ToAgentType())
		state.agentNames = append(state.agentNames, def.Name)
	}

	if hooksPath, ok := p.HooksPath(); ok && m.policy != nil {
		hooks, err := policy.LoadPluginHooks(p.Dir, hooksPath)
		if err != nil {
			debug.Log("plugin: hooks %s: %v", hooksPath, err)
		} else {
			for _, hook := range hooks {
				m.policy.Register(hook)
			}
		}
	}

	for name, cfg := range p.MCPServers() {
		id := "plugin:" + p.Name + ":" + name
		if err := m.mcp.Add(id, name, plugin.ExpandPluginMCPConfig(cfg, p.Dir)); err != nil {
			debug.Log("plugin: mcp %s: %v", name, err)
		}
		state.mcpIDs = append(state.mcpIDs, id)
	}
	m.active[p.Name] = state
}

func (m *Manager) deactivateLocked(name string) {
	state, ok := m.active[name]
	if !ok {
		return
	}
	for _, agentName := range state.agentNames {
		subagent.UnregisterPlugin(agentName)
	}
	for _, id := range state.mcpIDs {
		m.mcp.Remove(id)
	}
	if m.policy != nil {
		m.policy.UnregisterPrefix("plugin:" + state.plugin.Dir + ":")
	}
	delete(m.active, name)
}

func (m *Manager) deactivateAllLocked() {
	for name := range m.active {
		m.deactivateLocked(name)
	}
}
