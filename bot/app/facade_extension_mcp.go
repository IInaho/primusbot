package app

import (
	"nekocode/bot/config"
	"nekocode/bot/extension/mcp"
	"nekocode/bot/extension/plugin"
	"nekocode/bot/tools/runtime/core"
	"nekocode/common/debug"
	commonview "nekocode/common/view"
)

// InitConfigMCPServers starts the MCP servers declared in the user config
// and records their views for the skill-management panel.
func (e *extensionFacade) InitConfigMCPServers(servers map[string]config.MCPServerConfig) {
	e.configMCP = nil
	for name, cfg := range servers {
		e.configMCP = append(e.configMCP, commonview.NewMCPServerView(commonview.MCPServerViewInput{
			Name:    name,
			Plugin:  "配置",
			Command: cfg.Command,
			Args:    cfg.Args,
			Enabled: cfg.Enabled,
		}))
		if !cfg.Enabled {
			continue
		}
		added, removed, err := e.mcp.AddServer(name, mcp.ServerConfig{
			Command: cfg.Command,
			Args:    append([]string(nil), cfg.Args...),
			Env:     cfg.Env,
		})
		e.applyMCPTools(added, removed)
		if err != nil {
			debug.Log("config mcp %s: %v", name, err)
		}
	}
}

// registerPluginMCPServer adapts plugin.Manager's callback: the plugin
// package expands relative paths against the plugin root, the mcp package
// owns the server lifecycle, and the facade owns tool registration.
func (e *extensionFacade) registerPluginMCPServer(pluginDir, name string, cfg plugin.MCPServerConfig) error {
	added, removed, err := e.mcp.AddServer(name, plugin.ExpandPluginMCPConfig(cfg, pluginDir))
	e.applyMCPTools(added, removed)
	return err
}

func (e *extensionFacade) unregisterPluginMCPServer(name string) {
	e.applyMCPTools(nil, e.mcp.RemoveServer(name))
}

// applyMCPTools syncs the tool registry with the tools an mcp.Manager
// operation exposed or dropped.
func (e *extensionFacade) applyMCPTools(added, removed []core.Tool) {
	for _, t := range removed {
		e.toolRegistry.Unregister(t.Name())
	}
	for _, t := range added {
		e.toolRegistry.Register(t)
	}
}

// mcpHealthViews converts mcp.Health snapshots to the view layer's type.
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
