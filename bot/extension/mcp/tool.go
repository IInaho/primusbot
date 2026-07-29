package mcp

import (
	"context"
	"fmt"
	"slices"

	"nekocode/bot/tools/runtime/core"
)

// mcpTool adapts an MCP server tool to the tools.Tool interface.
type mcpTool struct {
	client   *client
	def      toolDef
	fullName string
}

// newMCPTool creates a tool adapter.
func newMCPTool(client *client, def toolDef) *mcpTool {
	return &mcpTool{
		client:   client,
		def:      def,
		fullName: client.name + "__" + def.Name,
	}
}

func (t *mcpTool) Name() string { return t.fullName }
func (t *mcpTool) Description() string {
	return fmt.Sprintf("[MCP:%s] %s", t.client.name, t.def.Description)
}

func (t *mcpTool) Parameters() []core.Parameter {
	var params []core.Parameter
	for name, prop := range t.def.InputSchema.Properties {
		p := core.Parameter{
			Name:        name,
			Type:        prop.Type,
			Description: prop.Description,
			Required:    slices.Contains(t.def.InputSchema.Required, name),
		}
		params = append(params, p)
	}
	return params
}

func (t *mcpTool) ExecutionMode(args map[string]any) core.ExecutionMode {
	return core.ModeSequential
}

func (t *mcpTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	return t.client.CallTool(t.def.Name, args)
}
