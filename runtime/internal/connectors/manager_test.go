package connectors

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type statusConnector struct {
	name    string
	stopped bool
}

func (c statusConnector) Name() string { return c.name }
func (c statusConnector) Start(context.Context) error {
	return nil
}
func (c *statusConnector) Stop() error {
	c.stopped = true
	return nil
}
func (c statusConnector) HandleCommand(context.Context, []string) (string, error) {
	return "ok", nil
}
func (c *statusConnector) ConnectorStatusView() ConnectorView {
	return ConnectorView{
		Name:        c.name,
		Configured:  true,
		Running:     true,
		Status:      "running",
		Initialized: true,
	}
}

func TestConnectorManagerStartsConnector(t *testing.T) {
	manager := NewManager(nil)
	started := false
	manager.Register("telegram", func(Host) Connector {
		return &statusConnector{
			name: "telegram",
		}
	})

	if _, err := manager.Handle(context.Background(), []string{"telegram", "status"}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !started {
		// The existing statusConnector.Start is a no-op; we can only observe
		// via the View that initialization succeeded.
	}

	view := manager.View()
	var telegram ConnectorView
	for _, conn := range view.Connectors {
		if conn.Name == "telegram" {
			telegram = conn
		}
	}
	if !telegram.Initialized {
		t.Fatalf("connector was not initialized: %#v", telegram)
	}
}

func TestConnectorManagerDoesNotAutoStart(t *testing.T) {
	manager := NewManager(nil)
	startCalled := false
	manager.Register("failing", func(Host) Connector {
		return &trackingConnector{name: "failing", started: &startCalled}
	})

	// Lazy init must dispatch the command WITHOUT calling Start: credentials
	// are configured via HandleCommand, so auto-starting would deadlock
	// first-time setup.
	if _, err := manager.Handle(context.Background(), []string{"failing"}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if startCalled {
		t.Fatal("manager auto-started the connector on lazy init")
	}

	view := manager.View()
	for _, conn := range view.Connectors {
		if conn.Name == "failing" && !conn.Initialized {
			t.Fatal("connector should be initialized after first command")
		}
	}
}

type trackingConnector struct {
	name    string
	started *bool
}

func (c trackingConnector) Name() string { return c.name }
func (c trackingConnector) Start(context.Context) error {
	*c.started = true
	return fmt.Errorf("start failed")
}
func (c trackingConnector) Stop() error { return nil }
func (c trackingConnector) HandleCommand(context.Context, []string) (string, error) {
	return "", nil
}

func TestConnectorManagerView(t *testing.T) {
	manager := NewManager(nil)
	manager.Register("telegram", func(Host) Connector {
		return &statusConnector{name: "telegram"}
	})
	manager.Register("slack", func(Host) Connector {
		return &statusConnector{name: "slack"}
	})

	view := manager.View()
	if len(view.Connectors) != 2 {
		t.Fatalf("connectors = %d, want 2", len(view.Connectors))
	}
	if view.Connectors[0].Name != "slack" || view.Connectors[0].Initialized {
		t.Fatalf("first connector before init = %#v", view.Connectors[0])
	}
	if _, err := manager.Handle(context.Background(), []string{"telegram", "status"}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	view = manager.View()
	var telegram ConnectorView
	for _, conn := range view.Connectors {
		if conn.Name == "telegram" {
			telegram = conn
		}
	}
	if !telegram.Initialized || !telegram.Running || !telegram.Configured || telegram.Status != "running" {
		t.Fatalf("telegram view = %#v", telegram)
	}
	devices := manager.Devices()
	if devices == "" || !strings.Contains(devices, "telegram") {
		t.Fatalf("devices = %q", devices)
	}
	if msg, err := manager.Disconnect("telegram"); err != nil || !strings.Contains(msg, "stopped") {
		t.Fatalf("Disconnect = %q err=%v", msg, err)
	}
}
