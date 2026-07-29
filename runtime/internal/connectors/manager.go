package connectors

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"nekocode/runtime/internal/core"
)

type Runtime = core.Runtime
type ConnectView = core.ConnectView
type ConnectorView = core.ConnectorView
type ConnectorDeviceView = core.ConnectorDeviceView

type Connector interface {
	Name() string
	Start(ctx context.Context) error
	Stop() error
	HandleCommand(ctx context.Context, args []string) (string, error)
}

type ConnectorStatusViewer interface {
	ConnectorStatusView() ConnectorView
}

type ConnectorFactory func(rt Runtime) Connector

type Manager struct {
	mu         sync.Mutex
	runtime    Runtime
	factories  map[string]ConnectorFactory
	connectors map[string]Connector
}

func NewManager(rt Runtime) *Manager {
	return &Manager{
		runtime:    rt,
		factories:  make(map[string]ConnectorFactory),
		connectors: make(map[string]Connector),
	}
}

func (m *Manager) Register(name string, factory ConnectorFactory) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || factory == nil {
		return
	}
	m.mu.Lock()
	m.factories[name] = factory
	m.mu.Unlock()
}

func (m *Manager) Handle(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return m.usage(), nil
	}
	name := strings.ToLower(strings.TrimSpace(args[0]))
	if name == "" {
		return m.usage(), nil
	}
	conn, err := m.connector(ctx, name)
	if err != nil {
		return "", err
	}
	return conn.HandleCommand(ctx, args[1:])
}

func (m *Manager) Disconnect(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return m.usageFor("disconnect"), nil
	}
	m.mu.Lock()
	conn, ok := m.connectors[name]
	m.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("connector %q is not initialized", name)
	}
	if err := conn.Stop(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s connector stopped.", name), nil
}

func (m *Manager) Devices() string {
	view := m.View()
	if len(view.Connectors) == 0 {
		return "No connectors registered."
	}
	var lines []string
	for _, conn := range view.Connectors {
		status := conn.Status
		if status == "" {
			status = "registered"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", conn.Name, status))
		if len(conn.Devices) == 0 {
			continue
		}
		for _, device := range conn.Devices {
			label := device.Display
			if label == "" && device.Username != "" {
				label = "@" + device.Username
			}
			if label == "" {
				label = device.ID
			}
			lines = append(lines, "  - "+label)
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Manager) View() ConnectView {
	m.mu.Lock()
	names := m.namesLocked()
	connectors := make(map[string]Connector, len(m.connectors))
	for name, conn := range m.connectors {
		connectors[name] = conn
	}
	m.mu.Unlock()

	views := make([]ConnectorView, 0, len(names))
	for _, name := range names {
		conn := connectors[name]
		view := ConnectorView{
			Name:        name,
			Registered:  true,
			Initialized: conn != nil,
			Status:      "registered",
		}
		if viewer, ok := conn.(ConnectorStatusViewer); ok {
			view = viewer.ConnectorStatusView()
			view.Name = name
			view.Registered = true
			view.Initialized = true
		}
		views = append(views, view)
	}
	return ConnectView{Connectors: views}
}

func (m *Manager) connector(ctx context.Context, name string) (Connector, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if conn, ok := m.connectors[name]; ok {
		return conn, nil
	}
	factory, ok := m.factories[name]
	if !ok {
		return nil, fmt.Errorf("unknown connector %q. Available: %s", name, strings.Join(m.namesLocked(), ", "))
	}
	conn := factory(m.runtime)
	// Do NOT auto-start here: Start requires credentials that are themselves
	// configured via HandleCommand (e.g. /connect feishu add ...). Starting
	// lazily would deadlock first-time setup — the command never runs because
	// Start fails on missing config. Connectors start themselves from their
	// command handlers once configured.
	m.connectors[name] = conn
	return conn, nil
}

func (m *Manager) usage() string {
	m.mu.Lock()
	names := m.namesLocked()
	m.mu.Unlock()
	if len(names) == 0 {
		return "No connectors registered."
	}
	return "Usage: /connect <connector>\nAvailable: " + strings.Join(names, ", ")
}

func (m *Manager) usageFor(command string) string {
	m.mu.Lock()
	names := m.namesLocked()
	m.mu.Unlock()
	if len(names) == 0 {
		return "No connectors registered."
	}
	return "Usage: /" + command + " <connector>\nAvailable: " + strings.Join(names, ", ")
}

func (m *Manager) Close() error {
	m.mu.Lock()
	connectors := make(map[string]Connector, len(m.connectors))
	for name, conn := range m.connectors {
		connectors[name] = conn
	}
	m.connectors = make(map[string]Connector)
	m.factories = make(map[string]ConnectorFactory)
	m.mu.Unlock()

	var errs []error
	for _, conn := range connectors {
		if err := conn.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to stop connectors: %v", errs)
	}
	return nil
}

func (m *Manager) namesLocked() []string {
	names := make([]string, 0, len(m.factories))
	for name := range m.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
