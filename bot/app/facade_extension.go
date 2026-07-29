package app

import (
	"nekocode/bot/command"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/extension/mcp"
	"nekocode/bot/extension/plugin"
	"nekocode/bot/extension/skill"
	"nekocode/bot/policy"
	"nekocode/bot/tools"
	commonview "nekocode/common/view"
)

type extensionFacade struct {
	skills    *skill.Manager
	plugins   *plugin.Manager
	mcp       *mcp.Manager
	configMCP []commonview.MCPServerView

	ctxMgr        *ctxmgr.Manager
	toolRegistry  *tools.Registry
	hookReg       *policy.Registry
	contextWindow int
}

func newExtensionFacade(ctxMgr *ctxmgr.Manager, toolRegistry *tools.Registry, hookReg *policy.Registry, contextWindow int) *extensionFacade {
	return &extensionFacade{
		ctxMgr:        ctxMgr,
		toolRegistry:  toolRegistry,
		hookReg:       hookReg,
		contextWindow: contextWindow,
		mcp:           mcp.NewManager(),
	}
}

func (e *extensionFacade) InitSkills() {
	e.skills = skill.NewManager(skill.ManagerOptions{
		Context:       e.ctxMgr,
		Tools:         e.toolRegistry,
		ContextWindow: e.contextWindow,
		PluginSkillDirs: func() []string {
			if e.plugins == nil {
				return nil
			}
			return e.plugins.SkillDirs()
		},
	})
	e.skills.Init()
}

func (e *extensionFacade) ReloadSkills() {
	if e.skills != nil {
		e.skills.ReloadPreservingLoaded()
	}
}

func (e *extensionFacade) RefreshPluginSkills() {
	e.ReloadSkills()
}

func (e *extensionFacade) RefreshSkillList() {
	if e.skills != nil {
		e.skills.RefreshList()
	}
}

// SkillManagementView assembles the management UI snapshot here in the
// composition layer: skill/plugin domain packages supply plain data, the
// facade projects it into view models.
func (e *extensionFacade) SkillManagementView() commonview.SkillManagementView {
	plugins := e.plugins.ListPlugins()
	pluginViews := buildPluginViews(plugins)
	mcpServers := buildPluginMCPViews(plugins)
	mcpServers = append(mcpServers, e.configMCP...)
	commonview.ApplyMCPHealth(mcpServers, mcpHealthViews(e.mcp.Health()))
	return commonview.SkillManagementView{
		Skills:  buildSkillViews(e.skills.List(), e.skills.LoadedSet(), pluginViews),
		Plugins: pluginViews,
		MCP:     mcpServers,
	}
}

func (e *extensionFacade) SetPluginEnabled(name string, enabled bool) (commonview.SkillManagementView, error) {
	if _, err := e.plugins.SetEnabled(name, enabled); err != nil {
		return commonview.SkillManagementView{}, err
	}
	return e.SkillManagementView(), nil
}

func (e *extensionFacade) RefreshSkillManagement() commonview.SkillManagementView {
	e.plugins.Reload()
	return e.SkillManagementView()
}

type skillCommandProvider struct {
	manager *skill.Manager
}

func (p skillCommandProvider) ListForCommands() []command.SkillCommand {
	if p.manager == nil {
		return nil
	}
	skills := p.manager.List()
	out := make([]command.SkillCommand, 0, len(skills))
	for _, sk := range skills {
		out = append(out, command.SkillCommand{
			Name:    sk.Name,
			Context: skill.FormatForContext(sk),
		})
	}
	return out
}

func (p skillCommandProvider) GetForCommand(name string) (command.SkillCommand, bool) {
	if p.manager == nil {
		return command.SkillCommand{}, false
	}
	sk, ok := p.manager.Get(name)
	if !ok {
		return command.SkillCommand{}, false
	}
	return command.SkillCommand{
		Name:    sk.Name,
		Context: skill.FormatForContext(sk),
	}, true
}

func (p skillCommandProvider) MarkLoaded(name string) {
	if p.manager != nil {
		p.manager.MarkLoaded(name)
	}
}

func (p skillCommandProvider) ClearLoaded() {
	if p.manager != nil {
		p.manager.ClearLoaded()
	}
}
