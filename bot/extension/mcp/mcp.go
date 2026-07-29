// Package mcp implements an MCP (Model Context Protocol) client over stdio.
//
// Manager is the package entry point: it owns MCP server processes, health,
// tool discovery, and tool registration.
package mcp

import (
	"fmt"
	"maps"
	"sync"

	"nekocode/bot/tools"
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
	name   string
	client *client
	tools  []core.Tool
}

// Manager owns the lifecycle of every MCP server connection: spawning
// processes, performing the initialize handshake, and registering tools.
// Servers have a stable owner ID separate from their user-facing name.
type Manager struct {
	mu       sync.Mutex
	servers  map[string]*server
	owners   map[string]string
	health   map[string]Health
	registry *tools.Registry
}

// New creates an empty Manager that owns MCP tool registration.
func New(registry *tools.Registry) *Manager {
	return &Manager{
		servers:  make(map[string]*server),
		owners:   make(map[string]string),
		health:   make(map[string]Health),
		registry: registry,
	}
}

// Add starts or replaces a server owned by id. Name remains the MCP tool
// prefix visible to the model.
func (m *Manager) Add(id, name string, cfg ServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if owner := m.owners[name]; owner != "" && owner != id {
		return fmt.Errorf("server name %q is already owned by %s", name, owner)
	}
	m.removeLocked(id)

	client := newClient(name, cfg)
	m.servers[id] = &server{name: name, client: client}
	m.owners[name] = id
	m.health[name] = Health{Status: StatusStarting}

	if err := client.Start(); err != nil {
		m.health[name] = Health{Status: StatusError, Error: err.Error()}
		return fmt.Errorf("start: %w", err)
	}

	defs, err := client.ListTools()
	if err != nil {
		_ = client.Close()
		m.health[name] = Health{Status: StatusError, Error: err.Error()}
		return fmt.Errorf("list tools: %w", err)
	}

	serverTools := make([]core.Tool, 0, len(defs))
	for _, td := range defs {
		serverTools = append(serverTools, newMCPTool(client, td))
	}
	m.servers[id].tools = serverTools
	m.registerTools(serverTools)
	m.health[name] = Health{Status: StatusReady, ToolCount: len(serverTools)}
	return nil
}

// Remove stops the server owned by id and unregisters its tools.
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeLocked(id)
}

// Close stops every managed server and unregisters all MCP tools.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.servers {
		_ = s.client.Close()
		m.unregisterTools(s.tools)
		delete(m.servers, id)
	}
	clear(m.owners)
	clear(m.health)
}

// Health returns a snapshot of per-server health keyed by server name.
func (m *Manager) Health() map[string]Health {
	m.mu.Lock()
	defer m.mu.Unlock()
	return maps.Clone(m.health)
}

func (m *Manager) registerTools(serverTools []core.Tool) {
	if m.registry == nil {
		return
	}
	for _, tool := range serverTools {
		m.registry.Register(tool)
	}
}

func (m *Manager) unregisterTools(serverTools []core.Tool) {
	if m.registry == nil {
		return
	}
	for _, tool := range serverTools {
		m.registry.Unregister(tool.Name())
	}
}

func (m *Manager) removeLocked(id string) {
	s, ok := m.servers[id]
	if !ok {
		return
	}
	_ = s.client.Close()
	m.unregisterTools(s.tools)
	delete(m.servers, id)
	if m.owners[s.name] == id {
		delete(m.owners, s.name)
		delete(m.health, s.name)
	}
}
