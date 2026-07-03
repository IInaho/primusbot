package mcp

import (
	"context"
	"strings"
	"testing"

	"nekocode/bot/llm/types"
)

func TestMCPToolAdapter(t *testing.T) {
	mockTools := []ToolDef{
		{
			Name:        "search-files",
			Description: "Search for files matching a pattern",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]types.Property{
					"pattern": {Type: "string", Description: "Glob pattern to match"},
					"dir":     {Type: "string", Description: "Directory to search"},
				},
				Required: []string{"pattern"},
			},
		},
	}
	cmd, cleanup := startMockMCP(t, mockTools)
	defer cleanup()

	c := NewClient("search-mcp", ServerConfig{Command: cmd.Path})
	if err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	tools, err := c.ListTools()
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	mt := NewMCPTool(c, tools[0])

	if !strings.HasPrefix(mt.Name(), "search-mcp__") {
		t.Errorf("Name = %q, should have prefix search-mcp__", mt.Name())
	}
	if !strings.Contains(mt.Description(), "[MCP:search-mcp]") {
		t.Errorf("Description = %q, should contain [MCP:search-mcp]", mt.Description())
	}

	params := mt.Parameters()
	if len(params) != 2 {
		t.Fatalf("params len = %d, want 2", len(params))
	}
	paramMap := make(map[string]bool)
	for _, p := range params {
		paramMap[p.Name] = p.Required
	}
	if !paramMap["pattern"] {
		t.Error("pattern should be required")
	}
	if paramMap["dir"] {
		t.Error("dir should not be required")
	}
}

func TestMCPToolExecute(t *testing.T) {
	mockTools := []ToolDef{
		{Name: "echo", Description: "Echo back", InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]types.Property{"msg": {Type: "string"}},
		}},
	}
	cmd, cleanup := startMockMCP(t, mockTools)
	defer cleanup()

	c := NewClient("echo-mcp", ServerConfig{Command: cmd.Path})
	if err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	tools, _ := c.ListTools()
	mt := NewMCPTool(c, tools[0])

	result, err := mt.Execute(context.Background(), map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "ok: echo" {
		t.Errorf("result = %q, want 'ok: echo'", result)
	}
}
