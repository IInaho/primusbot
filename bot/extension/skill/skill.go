// Package skill provides a file-based skill system: skills are discovered
// from SKILL.md files (built-in, plugin, and user directories), exposed to
// the model through a "skill" tool, and injected into the conversation
// context on demand.
//
// Manager is the package entry point; the skill registry and the tool
// adapter are internal implementation details.
package skill

import (
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/tools"
)

// Skill represents a loaded skill definition: the parsed content of one
// SKILL.md file plus its auxiliary files.
type Skill struct {
	Name        string
	Description string
	Content     string
	Dir         string
	Files       []string

	Context                string
	AgentType              string
	AllowedTools           []string
	MaxSteps               int
	ContextWindow          int
	DisableModelInvocation bool
}

// Manager owns skill discovery, loaded-state tracking, context injection,
// and registration of the "skill" tool. Construct it with NewManager and
// call Init once the surrounding wiring (context manager, tool registry)
// is in place.
type Manager struct {
	reg             *registry
	ctx             *ctxmgr.Manager
	tools           *tools.Registry
	contextWindow   int
	pluginSkillDirs func() []string
}

type ManagerOptions struct {
	Context         *ctxmgr.Manager
	Tools           *tools.Registry
	ContextWindow   int
	PluginSkillDirs func() []string
}

func NewManager(opts ManagerOptions) *Manager {
	return &Manager{
		ctx:             opts.Context,
		tools:           opts.Tools,
		contextWindow:   opts.ContextWindow,
		pluginSkillDirs: opts.PluginSkillDirs,
	}
}

func (m *Manager) Init() {
	m.Reload(nil)
}

func (m *Manager) Reload(loaded map[string]bool) {
	m.reg = newRegistry()
	m.reg.RegisterAll(bundledSkills())
	dirs := defaultDirs()
	if m.pluginSkillDirs != nil {
		dirs = append(dirs, m.pluginSkillDirs()...)
	}
	m.reg.Load(dirs)
	for name := range loaded {
		if m.reg.Has(name) {
			m.reg.MarkLoaded(name)
		}
	}
	m.RefreshList()
	m.RegisterTool()
}

func (m *Manager) ReloadPreservingLoaded() {
	m.Reload(m.LoadedSet())
}

func (m *Manager) LoadPluginSkillDirs(dirs []string) {
	if m == nil || m.reg == nil {
		return
	}
	m.reg.Load(dirs)
	m.RefreshList()
	m.RegisterTool()
}

func (m *Manager) List() []*Skill {
	if m == nil || m.reg == nil {
		return nil
	}
	return m.reg.List()
}

func (m *Manager) Get(name string) (*Skill, bool) {
	if m == nil || m.reg == nil {
		return nil, false
	}
	return m.reg.Get(name)
}

func (m *Manager) LoadedSet() map[string]bool {
	if m == nil || m.reg == nil {
		return nil
	}
	return m.reg.LoadedSet()
}

func (m *Manager) MarkLoaded(name string) {
	if m == nil || m.reg == nil {
		return
	}
	m.reg.MarkLoaded(name)
	m.RefreshList()
}

func (m *Manager) ClearLoaded() {
	if m == nil || m.reg == nil {
		return
	}
	m.reg.ClearLoaded()
	m.RefreshList()
}

func (m *Manager) RefreshList() {
	if m == nil || m.ctx == nil || m.reg == nil {
		return
	}
	m.ctx.SetSkillList(buildSkillListText(m.reg.List(), m.reg.LoadedSet(), m.contextWindow))
}

func (m *Manager) RegisterTool() {
	if m == nil || m.tools == nil || m.reg == nil {
		return
	}
	m.tools.Unregister("skill")
	m.tools.Register(newSkillTool(m.reg))
}
