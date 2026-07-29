package app

import (
	"path/filepath"
	"sort"
	"strings"

	"nekocode/bot/extension/plugin"
	"nekocode/bot/extension/skill"
	commonview "nekocode/common/view"
)

// buildPluginViews projects installed plugins into view models for the
// management UI. The plugin package itself stays free of presentation
// concerns.
func buildPluginViews(plugins []*plugin.Plugin) []commonview.PluginView {
	out := make([]commonview.PluginView, 0, len(plugins))
	for _, p := range plugins {
		skillDirs := p.SkillDirs()
		out = append(out, commonview.PluginView{
			Name:        p.Name,
			Version:     p.Version,
			Description: p.Description,
			Source:      p.Source,
			Dir:         p.Dir,
			Enabled:     p.Enabled,
			Skills:      skillDirs,
			SkillNames:  baseNames(skillDirs),
			Agents:      baseNames(p.AgentPaths()),
			Commands:    commandNames(p.Manifest.Commands),
			MCPServers:  mcpServerNames(p.MCPServers()),
			HasHooks:    hasHooks(p),
		})
	}
	return out
}

// buildPluginMCPViews flattens the MCP servers declared by all installed
// plugins, attributed to their source plugin and its current enabled state.
func buildPluginMCPViews(plugins []*plugin.Plugin) []commonview.MCPServerView {
	out := make([]commonview.MCPServerView, 0)
	for _, p := range plugins {
		for name, cfg := range p.MCPServers() {
			cfg = plugin.ExpandPluginMCPConfig(cfg, p.Dir)
			out = append(out, commonview.NewMCPServerView(commonview.MCPServerViewInput{
				Name:    name,
				Plugin:  p.Name,
				Command: cfg.Command,
				Args:    cfg.Args,
				Enabled: p.Enabled,
			}))
		}
	}
	return out
}

// baseNames returns the base name of every non-empty path.
func baseNames(paths []string) []string {
	var names []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		names = append(names, filepath.Base(p))
	}
	return names
}

func commandNames(cmds []plugin.CommandEntry) []string {
	var names []string
	for _, c := range cmds {
		if c.Name == "" {
			continue
		}
		names = append(names, c.Name)
	}
	return names
}

func mcpServerNames(servers map[string]plugin.MCPServerConfig) []string {
	var names []string
	for name := range servers {
		names = append(names, name)
	}
	return names
}

func hasHooks(p *plugin.Plugin) bool {
	_, ok := p.HooksPath()
	return ok
}

// buildSkillViews projects skill.Manager's public snapshot (List + LoadedSet)
// into view models for the management UI. The skill package itself stays
// free of presentation concerns.
func buildSkillViews(skills []*skill.Skill, loaded map[string]bool, plugins []commonview.PluginView) []commonview.SkillView {
	out := make([]commonview.SkillView, 0, len(skills))
	for _, sk := range skills {
		kind, source, pluginName := sourceForDir(sk.Dir, plugins)
		files := append([]string(nil), sk.Files...)
		sort.Strings(files)
		out = append(out, commonview.SkillView{
			Name:        sk.Name,
			Description: sk.Description,
			Dir:         sk.Dir,
			Files:       files,
			Loaded:      loaded[sk.Name],
			Source:      source,
			SourceKind:  kind,
			Plugin:      pluginName,
		})
	}
	return out
}

// sourceForDir classifies a skill directory into one of three kinds:
//   - "builtin": embedded/bundled skill (empty dir)
//   - "plugin":  lives under a plugin's skill directory
//   - "local":   a standalone file-system skill (e.g. ~/.nekocode/skills/...)
//
// label is a Chinese display string ("内置" / "插件" / "本地"); kind is the
// stable machine-readable value.
func sourceForDir(dir string, plugins []commonview.PluginView) (kind, label, pluginName string) {
	if dir == "" {
		return "builtin", "内置", ""
	}
	absDir := absOr(dir)
	for _, p := range plugins {
		for _, skillDir := range p.Skills {
			absSkillDir := absOr(skillDir)
			if absDir == absSkillDir || strings.HasPrefix(absDir, absSkillDir+string(filepath.Separator)) {
				return "plugin", "插件", p.Name
			}
		}
	}
	return "local", "本地", ""
}

func absOr(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}
