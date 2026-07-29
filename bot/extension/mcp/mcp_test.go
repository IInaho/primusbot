package mcp

import (
	"testing"

	"nekocode/bot/tools"
)

func assertNames(t *testing.T, registry *tools.Registry, want ...string) {
	t.Helper()
	names := registry.Names()
	if len(names) != len(want) {
		t.Fatalf("tool names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("tool names = %v, want %v", names, want)
		}
	}
}

func TestManagerAddServer(t *testing.T) {
	mockTools := []toolDef{
		{Name: "alpha", Description: "tool alpha", InputSchema: inputSchema{Type: "object"}},
		{Name: "beta", Description: "tool beta", InputSchema: inputSchema{Type: "object"}},
	}
	cmd, cleanup := startMockMCP(t, mockTools)
	defer cleanup()

	registry := tools.NewRegistry()
	m := New(registry)

	if err := m.Add("plugin:p:srv", "srv", ServerConfig{Command: cmd.Path}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	assertNames(t, registry, "srv__alpha", "srv__beta")

	h := m.Health()["srv"]
	if h.Status != StatusReady || h.ToolCount != 2 {
		t.Errorf("health = %+v, want ready with 2 tools", h)
	}

	m.Close()
	assertNames(t, registry)
}

func TestManagerAddServerStartFailure(t *testing.T) {
	registry := tools.NewRegistry()
	m := New(registry)

	if err := m.Add("config:bad", "bad", ServerConfig{Command: "/nonexistent-mcp-server"}); err == nil {
		t.Fatal("Add should fail for a missing command")
	}
	assertNames(t, registry)

	h, ok := m.Health()["bad"]
	if !ok {
		t.Fatal("health should be recorded for failed server")
	}
	if h.Status != StatusError || h.Error == "" {
		t.Errorf("health = %+v, want error status with message", h)
	}
}

func TestManagerRemoveServer(t *testing.T) {
	cmd, cleanup := startMockMCP(t, []toolDef{
		{Name: "alpha", InputSchema: inputSchema{Type: "object"}},
	})
	defer cleanup()

	registry := tools.NewRegistry()
	m := New(registry)

	if err := m.Add("plugin:p:srv", "srv", ServerConfig{Command: cmd.Path}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	m.Remove("plugin:p:srv")
	assertNames(t, registry)
	if _, ok := m.Health()["srv"]; ok {
		t.Error("health should be cleared after RemoveServer")
	}

	// Removing an unknown server is a no-op.
	m.Remove("plugin:p:srv")
	assertNames(t, registry)
}

func TestManagerReAddReplacesTools(t *testing.T) {
	// First generation exposes alpha+beta; second exposes only gamma.
	cmd1, cleanup1 := startMockMCP(t, []toolDef{
		{Name: "alpha", InputSchema: inputSchema{Type: "object"}},
		{Name: "beta", InputSchema: inputSchema{Type: "object"}},
	})
	defer cleanup1()
	cmd2, cleanup2 := startMockMCP(t, []toolDef{
		{Name: "gamma", InputSchema: inputSchema{Type: "object"}},
	})
	defer cleanup2()

	registry := tools.NewRegistry()
	m := New(registry)

	if err := m.Add("plugin:p:srv", "srv", ServerConfig{Command: cmd1.Path}); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if err := m.Add("plugin:p:srv", "srv", ServerConfig{Command: cmd2.Path}); err != nil {
		t.Fatalf("second Add: %v", err)
	}

	assertNames(t, registry, "srv__gamma")

	m.Close()
	assertNames(t, registry)
}

func TestManagerRejectsDuplicateNameFromDifferentOwner(t *testing.T) {
	cmd1, cleanup1 := startMockMCP(t, []toolDef{
		{Name: "alpha", InputSchema: inputSchema{Type: "object"}},
	})
	defer cleanup1()
	cmd2, cleanup2 := startMockMCP(t, []toolDef{
		{Name: "beta", InputSchema: inputSchema{Type: "object"}},
	})
	defer cleanup2()

	registry := tools.NewRegistry()
	m := New(registry)
	if err := m.Add("plugin:p:srv", "srv", ServerConfig{Command: cmd1.Path}); err != nil {
		t.Fatalf("add plugin server: %v", err)
	}
	if err := m.Add("config:srv", "srv", ServerConfig{Command: cmd2.Path}); err == nil {
		t.Fatal("duplicate display name should be rejected")
	}
	assertNames(t, registry, "srv__alpha")

	// Removing an owner that never started must not affect the active server.
	m.Remove("config:srv")
	assertNames(t, registry, "srv__alpha")

	m.Remove("plugin:p:srv")
	assertNames(t, registry)
}

func TestManagerClose(t *testing.T) {
	cmd, cleanup := startMockMCP(t, []toolDef{
		{Name: "alpha", InputSchema: inputSchema{Type: "object"}},
	})
	defer cleanup()

	registry := tools.NewRegistry()
	m := New(registry)

	if err := m.Add("config:a", "a", ServerConfig{Command: cmd.Path}); err != nil {
		t.Fatalf("Add a: %v", err)
	}
	// A server that fails to start still holds health state that Close clears.
	_ = m.Add("config:bad", "bad", ServerConfig{Command: "/nonexistent-mcp-server"})

	m.Close()
	assertNames(t, registry)
	if len(m.Health()) != 0 {
		t.Errorf("health should be empty, got %v", m.Health())
	}
}
