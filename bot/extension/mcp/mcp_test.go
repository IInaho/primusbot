package mcp

import (
	"context"
	"strings"
	"testing"
)

func TestManagerAddServer(t *testing.T) {
	mockTools := []toolDef{
		{Name: "alpha", Description: "tool alpha", InputSchema: inputSchema{Type: "object"}},
		{Name: "beta", Description: "tool beta", InputSchema: inputSchema{Type: "object"}},
	}
	cmd, cleanup := startMockMCP(t, mockTools)
	defer cleanup()

	m := New()
	if err := m.Add(context.Background(), "plugin:p:srv", "srv", ServerConfig{Command: cmd.Path}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	list := m.ListCapabilities()
	for _, want := range []string{"srv:", "- alpha", "- beta"} {
		if !strings.Contains(list, want) {
			t.Fatalf("capabilities missing %q: %s", want, list)
		}
	}

	h := m.Health()["srv"]
	if h.Status != StatusReady || h.ToolCount != 2 {
		t.Errorf("health = %+v, want ready with 2 tools", h)
	}

	m.Close()
	if got := m.ListCapabilities(); got != "No MCP servers configured." {
		t.Fatalf("after close, list = %q", got)
	}
}

func TestManagerAddServerStartFailure(t *testing.T) {
	m := New()
	if err := m.Add(context.Background(), "config:bad", "bad", ServerConfig{Command: "/nonexistent-mcp-server"}); err == nil {
		t.Fatal("Add should fail for a missing command")
	}

	h, ok := m.Health()["bad"]
	if !ok {
		t.Fatal("health should be recorded for failed server")
	}
	if h.Status != StatusError || h.Error == "" {
		t.Errorf("health = %+v, want error status with message", h)
	}
	if !strings.Contains(m.ListCapabilities(), "bad (error") {
		t.Fatal("errored server should be listed with its status")
	}
}

func TestManagerRemoveServer(t *testing.T) {
	cmd, cleanup := startMockMCP(t, []toolDef{
		{Name: "alpha", InputSchema: inputSchema{Type: "object"}},
	})
	defer cleanup()

	m := New()
	if err := m.Add(context.Background(), "plugin:p:srv", "srv", ServerConfig{Command: cmd.Path}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	m.Remove("plugin:p:srv")
	if _, ok := m.Health()["srv"]; ok {
		t.Error("health should be cleared after RemoveServer")
	}
	if _, err := m.CallServerTool(context.Background(), "srv", "alpha", nil); err == nil {
		t.Fatal("removed server must not route calls")
	}

	// Removing an unknown server is a no-op.
	m.Remove("plugin:p:srv")
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

	m := New()
	if err := m.Add(context.Background(), "plugin:p:srv", "srv", ServerConfig{Command: cmd1.Path}); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if err := m.Add(context.Background(), "plugin:p:srv", "srv", ServerConfig{Command: cmd2.Path}); err != nil {
		t.Fatalf("second Add: %v", err)
	}

	list := m.ListCapabilities()
	if !strings.Contains(list, "- gamma") || strings.Contains(list, "- alpha") {
		t.Fatalf("re-added server tools = %s, want only gamma", list)
	}
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

	m := New()
	if err := m.Add(context.Background(), "plugin:p:srv", "srv", ServerConfig{Command: cmd1.Path}); err != nil {
		t.Fatalf("add plugin server: %v", err)
	}
	if err := m.Add(context.Background(), "config:srv", "srv", ServerConfig{Command: cmd2.Path}); err == nil {
		t.Fatal("duplicate display name should be rejected")
	}

	// Removing an owner that never started must not affect the active server.
	m.Remove("config:srv")
	if _, err := m.CallServerTool(context.Background(), "srv", "alpha", nil); err != nil {
		t.Fatalf("active server broken by removing a non-owner: %v", err)
	}
}

func TestManagerClose(t *testing.T) {
	cmd, cleanup := startMockMCP(t, []toolDef{
		{Name: "alpha", InputSchema: inputSchema{Type: "object"}},
	})
	defer cleanup()

	m := New()
	if err := m.Add(context.Background(), "config:a", "a", ServerConfig{Command: cmd.Path}); err != nil {
		t.Fatalf("Add a: %v", err)
	}
	// A server that fails to start still holds health state that Close clears.
	_ = m.Add(context.Background(), "config:bad", "bad", ServerConfig{Command: "/nonexistent-mcp-server"})

	m.Close()
	if len(m.Health()) != 0 {
		t.Errorf("health should be empty, got %v", m.Health())
	}
}
