package plugin

import (
	"path/filepath"

	commonview "nekocode/common/view"
)

func (m *Manager) Views() []commonview.PluginView {
	plugins := m.reg.List()
	out := make([]commonview.PluginView, 0, len(plugins))
	for _, p := range plugins {
		out = append(out, viewFor(p))
	}
	return out
}

func viewFor(p *Plugin) commonview.PluginView {
	return commonview.PluginView{
		Name:        p.Name,
		Version:     p.Version,
		Description: p.Description,
		Source:      p.Source,
		Dir:         p.Dir,
		Enabled:     p.Enabled,
		Skills:      p.SkillDirs(),
		SkillNames:  skillNamesFromDirs(p.SkillDirs()),
		Agents:      agentBaseNames(p.AgentPaths()),
		Commands:    commandNames(p.Manifest.Commands),
		MCPServers:  mcpServerNames(p.MCPServers()),
		HasHooks:    hooksExists(p),
	}
}

// MCPServersFor flattens the MCP servers declared by a plugin.
func MCPServersFor(p *Plugin) []commonview.MCPServerView {
	servers := p.MCPServers()
	out := make([]commonview.MCPServerView, 0, len(servers))
	for name, cfg := range servers {
		cfg = ExpandPluginMCPConfig(cfg, p.Dir)
		out = append(out, commonview.NewMCPServerView(commonview.MCPServerViewInput{
			Name:    name,
			Plugin:  p.Name,
			Command: cfg.Command,
			Args:    cfg.Args,
			Enabled: p.Enabled,
		}))
	}
	return out
}

func skillNamesFromDirs(dirs []string) []string {
	var names []string
	for _, d := range dirs {
		if d == "" {
			continue
		}
		names = append(names, filepath.Base(d))
	}
	return names
}

func agentBaseNames(paths []string) []string {
	var names []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		names = append(names, filepath.Base(p))
	}
	return names
}

func commandNames(cmds []CommandEntry) []string {
	var names []string
	for _, c := range cmds {
		if c.Name == "" {
			continue
		}
		names = append(names, c.Name)
	}
	return names
}

func mcpServerNames(servers map[string]MCPServerConfig) []string {
	var names []string
	for name := range servers {
		names = append(names, name)
	}
	return names
}

func hooksExists(p *Plugin) bool {
	_, ok := p.HooksPath()
	return ok
}
