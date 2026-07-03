package mcp

import (
	"context"
	"fmt"

	"nekocode/bot/tools/runtime/core"
)

// MCPTool adapts an MCP server tool to the tools.Tool interface.
type MCPTool struct {
	client   *Client
	def      ToolDef
	fullName string
}

// NewMCPTool creates a tool adapter.
func NewMCPTool(client *Client, def ToolDef) *MCPTool {
	return &MCPTool{
		client:   client,
		def:      def,
		fullName: client.Name + "__" + def.Name,
	}
}

func (t *MCPTool) Name() string { return t.fullName }
func (t *MCPTool) Description() string {
	return fmt.Sprintf("[MCP:%s] %s", t.client.Name, t.def.Description)
}

func (t *MCPTool) Parameters() []core.Parameter {
	var params []core.Parameter
	for name, prop := range t.def.InputSchema.Properties {
		p := core.Parameter{
			Name:        name,
			Type:        prop.Type,
			Description: prop.Description,
			Required:    isRequired(t.def.InputSchema.Required, name),
		}
		params = append(params, p)
	}
	return params
}

func (t *MCPTool) ExecutionMode(args map[string]any) core.ExecutionMode {
	return core.ModeSequential
}

func (t *MCPTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	return t.client.CallTool(t.def.Name, args)
}

func isRequired(required []string, name string) bool {
	for _, r := range required {
		if r == name {
			return true
		}
	}
	return false
}
