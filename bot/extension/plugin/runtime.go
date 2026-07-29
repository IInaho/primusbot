package plugin

import (
	"strings"

	"nekocode/bot/policy"
	"nekocode/common/debug"
)

// pluginRuntime wires a plugin's extensions (agents, hooks, MCP servers)
// into the host application, and tears them down again on unload. All
// registration points are injected callbacks so this package stays free of
// host dependencies.
type pluginRuntime struct {
	Hooks               *policy.Registry
	Logf                func(string, ...any)
	RegisterAgentPath   func(path string) error
	UnregisterAgentPath func(path string)
	RegisterMCPServer   func(pluginDir, name string, cfg MCPServerConfig) error
	UnregisterMCPServer func(name string)
}

func (r pluginRuntime) Load(p *Plugin) {
	r.registerAgents(p)
	r.registerHooks(p)
	r.registerMCP(p)
}

func (r pluginRuntime) Unload(p *Plugin) {
	r.unregisterAgents(p)
	r.unregisterMCP(p)
	r.unregisterHooks(p)
}

func (r pluginRuntime) logf(format string, args ...any) {
	if r.Logf != nil {
		r.Logf(format, args...)
		return
	}
	debug.Log(format, args...)
}

func (r pluginRuntime) registerAgents(p *Plugin) {
	if r.RegisterAgentPath == nil {
		return
	}
	for _, agentPath := range p.AgentPaths() {
		if err := r.RegisterAgentPath(agentPath); err != nil {
			r.logf("plugin: agent %s: %v", agentPath, err)
		}
	}
}

func (r pluginRuntime) unregisterAgents(p *Plugin) {
	if r.UnregisterAgentPath == nil {
		return
	}
	for _, ap := range p.AgentPaths() {
		r.UnregisterAgentPath(ap)
	}
}

func (r pluginRuntime) registerHooks(p *Plugin) {
	if r.Hooks == nil {
		return
	}
	if hooksPath, ok := p.HooksPath(); ok {
		if pluginHooks, err := policy.LoadPluginHooks(p.Dir, hooksPath); err == nil {
			for _, h := range pluginHooks {
				r.Hooks.Register(h)
			}
		} else {
			r.logf("plugin: hooks %s: %v", hooksPath, err)
		}
	}
}

func (r pluginRuntime) unregisterHooks(p *Plugin) {
	if r.Hooks == nil {
		return
	}
	r.Hooks.UnregisterWhere(func(h policy.Hook) bool {
		return strings.HasPrefix(h.Name, "plugin:") && strings.Contains(h.Name, p.Dir)
	})
}

func (r pluginRuntime) registerMCP(p *Plugin) {
	if r.RegisterMCPServer == nil {
		return
	}
	for name, cfg := range p.MCPServers() {
		if err := r.RegisterMCPServer(p.Dir, name, cfg); err != nil {
			r.logf("plugin: mcp %s: %v", name, err)
		}
	}
}

func (r pluginRuntime) unregisterMCP(p *Plugin) {
	if r.UnregisterMCPServer == nil {
		return
	}
	for srvName := range p.MCPServers() {
		r.UnregisterMCPServer(srvName)
	}
}
