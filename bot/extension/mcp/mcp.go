// Package mcp implements an MCP (Model Context Protocol) client over stdio.
//
// Manager is the package entry point: it owns the lifecycle of every MCP
// server connection (spawning processes, the initialize handshake, tool
// discovery) and hands discovered tools back to the caller, which decides
// how to register them. The connection handle (client) is an internal
// implementation detail.
package mcp

import (
	"fmt"
	"maps"
	"sync"

	"nekocode/bot/tools/runtime/core"
)

// Server health statuses reported by Manager.Health.
const (
	StatusStarting = "starting"
	StatusReady    = "ready"
	StatusError    = "error"
)

// Health reports the runtime state of one managed server.
type Health struct {
	Status    string
	Error     string
	ToolCount int
}

// server is a managed connection and the tools discovered on it.
type server struct {
	client *client
	tools  []core.Tool
}

// Manager owns the lifecycle of every MCP server connection: spawning
// processes, performing the initialize handshake, and discovering tools.
// It does not know how tools are registered — AddServer/RemoveServer/
// CloseAll return the affected tools and leave that to the caller.
type Manager struct {
	mu      sync.Mutex
	servers map[string]*server
	health  map[string]Health
}

// NewManager creates an empty Manager.
func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*server),
		health:  make(map[string]Health),
	}
}

// AddServer (re)starts the named server: it spawns the process, performs the
// initialize handshake and discovers the server's tools. It returns the
// tools the server now exposes (added) and, when replacing a previous
// incarnation of the same name, the tools that incarnation exposed (removed)
// so the caller can unregister them. On failure the server's health is
// recorded with StatusError and the error is returned.
func (m *Manager) AddServer(name string, cfg ServerConfig) (added, removed []core.Tool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if old, ok := m.servers[name]; ok {
		removed = old.tools
		_ = old.client.Close()
		delete(m.servers, name)
		delete(m.health, name)
	}

	client := newClient(name, cfg)
	m.servers[name] = &server{client: client}
	m.health[name] = Health{Status: StatusStarting}

	if err := client.Start(); err != nil {
		m.health[name] = Health{Status: StatusError, Error: err.Error()}
		return nil, removed, fmt.Errorf("start: %w", err)
	}

	defs, err := client.ListTools()
	if err != nil {
		_ = client.Close()
		delete(m.servers, name)
		m.health[name] = Health{Status: StatusError, Error: err.Error()}
		return nil, removed, fmt.Errorf("list tools: %w", err)
	}

	tools := make([]core.Tool, 0, len(defs))
	for _, td := range defs {
		tools = append(tools, newMCPTool(client, td))
	}
	m.servers[name].tools = tools
	m.health[name] = Health{Status: StatusReady, ToolCount: len(tools)}
	return tools, removed, nil
}

// RemoveServer stops the named server and returns the tools it exposed so
// the caller can unregister them. Unknown names return nil.
func (m *Manager) RemoveServer(name string) []core.Tool {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.servers[name]
	if !ok {
		return nil
	}
	_ = s.client.Close()
	delete(m.servers, name)
	delete(m.health, name)
	return s.tools
}

// CloseAll stops every managed server and returns all tools that were
// exposed, so the caller can unregister them.
func (m *Manager) CloseAll() []core.Tool {
	m.mu.Lock()
	defer m.mu.Unlock()
	var all []core.Tool
	for name, s := range m.servers {
		_ = s.client.Close()
		all = append(all, s.tools...)
		delete(m.servers, name)
		delete(m.health, name)
	}
	return all
}

// Health returns a snapshot of per-server health keyed by server name.
func (m *Manager) Health() map[string]Health {
	m.mu.Lock()
	defer m.mu.Unlock()
	return maps.Clone(m.health)
}
