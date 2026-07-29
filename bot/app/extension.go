package app

import (
	"nekocode/bot/config"
	"nekocode/bot/extension"
	"nekocode/bot/extension/mcp"
	"nekocode/common/debug"
	commonview "nekocode/common/view"
)

func (b *Bot) initExtensions() {
	b.ext = extension.New(b.ctxMgr, b.toolbox.Registry, b.policy, b.cfg.ContextWindow)
	// Config MCP owns its declared names; plugin MCP with a duplicate name is
	// rejected instead of replacing a user-configured server.
	b.initConfigMCPServers()
	b.ext.Load()
}

func (b *Bot) skillManagementView() commonview.SkillManagementView {
	snapshot := b.ext.Snapshot()
	pluginViews := buildPluginViews(snapshot.Plugins)
	mcpServers := buildPluginMCPViews(snapshot.Plugins)
	mcpServers = append(mcpServers, buildConfigMCPViews(b.cfg.MCPServers)...)
	commonview.ApplyMCPHealth(mcpServers, mcpHealthViews(snapshot.MCPHealth))
	return commonview.SkillManagementView{
		Skills:  buildSkillViews(snapshot.Skills, snapshot.LoadedSkills, pluginViews),
		Plugins: pluginViews,
		MCP:     mcpServers,
	}
}

func (b *Bot) initConfigMCPServers() {
	for name, cfg := range b.cfg.MCPServers {
		if !cfg.Enabled {
			continue
		}
		err := b.ext.AddMCPServer(name, mcp.ServerConfig{
			Command: cfg.Command,
			Args:    append([]string(nil), cfg.Args...),
			Env:     cfg.Env,
		})
		if err != nil {
			debug.Log("config mcp %s: %v", name, err)
		}
	}
}

func buildConfigMCPViews(servers map[string]config.MCPServerConfig) []commonview.MCPServerView {
	out := make([]commonview.MCPServerView, 0, len(servers))
	for name, cfg := range servers {
		out = append(out, commonview.NewMCPServerView(commonview.MCPServerViewInput{
			Name:    name,
			Plugin:  "配置",
			Command: cfg.Command,
			Args:    cfg.Args,
			Enabled: cfg.Enabled,
		}))
	}
	return out
}

func mcpHealthViews(health map[string]mcp.Health) map[string]commonview.MCPHealth {
	out := make(map[string]commonview.MCPHealth, len(health))
	for name, h := range health {
		out[name] = commonview.MCPHealth{
			Status:    h.Status,
			Error:     h.Error,
			ToolCount: h.ToolCount,
		}
	}
	return out
}
