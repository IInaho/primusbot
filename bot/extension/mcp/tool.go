package mcp

import (
	"context"
	"fmt"
	"slices"

	"nekocode/bot/provider/types"
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
			Name:               name,
			Type:               prop.Type,
			Description:        prop.Description,
			Enum:               prop.Enum,
			Items:              mcpSchema(prop.Items),
			Properties:         mcpProperties(prop.Properties),
			RequiredProperties: prop.Required,
			Required:           slices.Contains(t.def.InputSchema.Required, name),
		}
		params = append(params, p)
	}
	return params
}

func mcpSchema(prop *types.Property) *core.Schema {
	if prop == nil {
		return nil
	}
	return &core.Schema{
		Type: prop.Type, Description: prop.Description, Enum: prop.Enum,
		Items: mcpSchema(prop.Items), Properties: mcpProperties(prop.Properties), Required: prop.Required,
	}
}

func mcpProperties(properties map[string]types.Property) map[string]core.Schema {
	if len(properties) == 0 {
		return nil
	}
	out := make(map[string]core.Schema, len(properties))
	for name, prop := range properties {
		out[name] = *mcpSchema(&prop)
	}
	return out
}

func (t *mcpTool) ExecutionMode(args map[string]any) core.ExecutionMode {
	return core.ModeSequential
}

func (t *mcpTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	return t.client.CallTool(ctx, t.def.Name, args)
}
