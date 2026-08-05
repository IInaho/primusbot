// Package mcp implements an MCP (Model Context Protocol) client over stdio.
//
// Manager is the package entry point: it owns MCP server processes, health,
// and tool discovery. Tools reach the model exclusively through the
// constant-schema capability proxy (capability.go) so the provider-visible
// tool list never changes with MCP inventory.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
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
	tools  []toolDef
}

// Manager owns the lifecycle of every MCP server connection: spawning
// processes, performing the initialize handshake, and holding the tools for
// the capability proxy to route to. Servers have a stable owner ID separate
// from their user-facing name.
type Manager struct {
	mu      sync.Mutex
	servers map[string]*server
	owners  map[string]string
	health  map[string]Health
}

// New creates an empty Manager that owns MCP server lifecycles.
func New() *Manager {
	return &Manager{
		servers: make(map[string]*server),
		owners:  make(map[string]string),
		health:  make(map[string]Health),
	}
}

// Add starts or replaces a server owned by id. Name remains the MCP tool
// prefix visible to the model.
func (m *Manager) Add(ctx context.Context, id, name string, cfg ServerConfig) error {
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

	if err := client.Start(ctx); err != nil {
		m.health[name] = Health{Status: StatusError, Error: err.Error()}
		return fmt.Errorf("start: %w", err)
	}

	defs, err := client.ListTools(ctx)
	if err != nil {
		_ = client.Close()
		m.health[name] = Health{Status: StatusError, Error: err.Error()}
		return fmt.Errorf("list tools: %w", err)
	}

	serverTools := make([]toolDef, 0, len(defs))
	for _, td := range defs {
		serverTools = append(serverTools, td)
	}
	m.servers[id].tools = serverTools
	m.health[name] = Health{Status: StatusReady, ToolCount: len(serverTools)}
	return nil
}

// Remove stops the server owned by id. Its tools disappear from the
// capability proxy's routing (the provider-visible schema is unaffected).
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeLocked(id)
}

// Close stops every managed server.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.servers {
		_ = s.client.Close()
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

func (m *Manager) removeLocked(id string) {
	s, ok := m.servers[id]
	if !ok {
		return
	}
	_ = s.client.Close()
	delete(m.servers, id)
	if m.owners[s.name] == id {
		delete(m.owners, s.name)
		delete(m.health, s.name)
	}
}

// ListCapabilities renders the available servers and their tools in a
// compact, stable form for the model.
func (m *Manager) ListCapabilities() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.health) == 0 {
		return "No MCP servers configured."
	}
	names := make([]string, 0, len(m.health))
	for name := range m.health {
		names = append(names, name)
	}
	sort.Strings(names)
	byName := make(map[string]*server, len(m.servers))
	for _, s := range m.servers {
		byName[s.name] = s
	}

	var b strings.Builder
	for _, name := range names {
		h := m.health[name]
		if h.Status != StatusReady {
			fmt.Fprintf(&b, "%s (%s%s)\n", name, h.Status, healthErrorSuffix(h))
			continue
		}
		fmt.Fprintf(&b, "%s:\n", name)
		tools := make([]string, 0, len(byName[name].tools))
		for _, tool := range byName[name].tools {
			tools = append(tools, toolLine(tool))
		}
		sort.Strings(tools)
		for _, line := range tools {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func healthErrorSuffix(h Health) string {
	if h.Error == "" {
		return ""
	}
	return ": " + h.Error
}

func toolLine(def toolDef) string {
	desc := strings.Join(strings.Fields(def.Description), " ")
	const maxDesc = 80
	if len([]rune(desc)) > maxDesc {
		desc = string([]rune(desc)[:maxDesc]) + "…"
	}
	if desc == "" {
		return "- " + def.Name
	}
	return fmt.Sprintf("- %s — %s", def.Name, desc)
}

// InspectTool returns one tool's full description and input schema.
func (m *Manager) InspectTool(serverName, toolName string) (string, error) {
	_, def, err := m.findTool(serverName, toolName)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "mcp__%s__%s\n", serverName, def.Name)
	if def.Description != "" {
		b.WriteString(def.Description)
		b.WriteString("\n")
	}
	schema, err := json.MarshalIndent(def.InputSchema, "", "  ")
	if err != nil {
		return "", err
	}
	b.Write(schema)
	return b.String(), nil
}

// CallServerTool invokes a tool on a ready server.
func (m *Manager) CallServerTool(ctx context.Context, serverName, toolName string, args map[string]any) (string, error) {
	client, def, err := m.findTool(serverName, toolName)
	if err != nil {
		return "", err
	}
	return client.CallTool(ctx, def.Name, args)
}

// findTool resolves a server.tool pair to its client and definition.
func (m *Manager) findTool(serverName, toolName string) (*client, toolDef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.servers {
		if s.name != serverName {
			continue
		}
		for _, tool := range s.tools {
			if tool.Name == toolName {
				return s.client, tool, nil
			}
		}
		return nil, toolDef{}, fmt.Errorf("server %q has no tool %q", serverName, toolName)
	}
	if m.health[serverName].Status == StatusError {
		return nil, toolDef{}, fmt.Errorf("server %q is in error state: %s", serverName, m.health[serverName].Error)
	}
	return nil, toolDef{}, fmt.Errorf("unknown MCP server %q", serverName)
}
