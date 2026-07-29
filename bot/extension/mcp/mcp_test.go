package mcp

import (
	"sort"
	"testing"

	"nekocode/bot/tools/runtime/core"
)

func toolNames(tools []core.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name())
	}
	sort.Strings(names)
	return names
}

func assertNames(t *testing.T, got []core.Tool, want ...string) {
	t.Helper()
	names := toolNames(got)
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

	m := NewManager()

	added, removed, err := m.AddServer("srv", ServerConfig{Command: cmd.Path})
	if err != nil {
		t.Fatalf("AddServer: %v", err)
	}
	assertNames(t, added, "srv__alpha", "srv__beta")
	if len(removed) != 0 {
		t.Errorf("removed = %v, want empty on first add", toolNames(removed))
	}

	h := m.Health()["srv"]
	if h.Status != StatusReady || h.ToolCount != 2 {
		t.Errorf("health = %+v, want ready with 2 tools", h)
	}

	m.CloseAll()
}

func TestManagerAddServerStartFailure(t *testing.T) {
	m := NewManager()

	added, _, err := m.AddServer("bad", ServerConfig{Command: "/nonexistent-mcp-server"})
	if err == nil {
		t.Fatal("AddServer should fail for a missing command")
	}
	if len(added) != 0 {
		t.Errorf("added = %v, want empty on failure", toolNames(added))
	}

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

	m := NewManager()

	if _, _, err := m.AddServer("srv", ServerConfig{Command: cmd.Path}); err != nil {
		t.Fatalf("AddServer: %v", err)
	}

	removed := m.RemoveServer("srv")
	assertNames(t, removed, "srv__alpha")
	if _, ok := m.Health()["srv"]; ok {
		t.Error("health should be cleared after RemoveServer")
	}

	// Removing an unknown server is a no-op.
	if got := m.RemoveServer("srv"); got != nil {
		t.Errorf("RemoveServer on unknown name = %v, want nil", toolNames(got))
	}
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

	m := NewManager()

	if _, _, err := m.AddServer("srv", ServerConfig{Command: cmd1.Path}); err != nil {
		t.Fatalf("first AddServer: %v", err)
	}
	added, removed, err := m.AddServer("srv", ServerConfig{Command: cmd2.Path})
	if err != nil {
		t.Fatalf("second AddServer: %v", err)
	}

	assertNames(t, added, "srv__gamma")
	assertNames(t, removed, "srv__alpha", "srv__beta")

	m.CloseAll()
}

func TestManagerCloseAll(t *testing.T) {
	cmd, cleanup := startMockMCP(t, []toolDef{
		{Name: "alpha", InputSchema: inputSchema{Type: "object"}},
	})
	defer cleanup()

	m := NewManager()

	if _, _, err := m.AddServer("a", ServerConfig{Command: cmd.Path}); err != nil {
		t.Fatalf("AddServer a: %v", err)
	}
	// A server that fails to start still holds state that CloseAll must clear.
	_, _, _ = m.AddServer("bad", ServerConfig{Command: "/nonexistent-mcp-server"})

	removed := m.CloseAll()
	assertNames(t, removed, "a__alpha")
	if len(m.Health()) != 0 {
		t.Errorf("health should be empty, got %v", m.Health())
	}
}
