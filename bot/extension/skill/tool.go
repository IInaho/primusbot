package skill

import (
	"context"
	"fmt"
	"nekocode/bot/tools/runtime/core"
)

// skillTool implements tools.Tool to let the model load skills by name.
type skillTool struct {
	registry *registry
	onLoad   func(name string) // called after a skill is loaded via tool
}

// newSkillTool creates a skill tool bound to the given registry.
func newSkillTool(r *registry) *skillTool {
	return &skillTool{registry: r}
}

// SetOnLoad sets a callback invoked after a skill is successfully loaded via this tool.
func (t *skillTool) SetOnLoad(fn func(name string)) { t.onLoad = fn }

func (t *skillTool) Name() string { return "skill" }
func (t *skillTool) Description() string {
	return "Load a skill's instructions and workflows by name. Use when a task matches an available skill. If its <skill_content> is already present, follow it without reloading; a loaded skill may be reloaded only when compaction removed that content."
}

func (t *skillTool) Parameters() []core.Parameter {
	return []core.Parameter{
		{
			Name:        "name",
			Type:        "string",
			Required:    true,
			Description: "The skill name to load, from the available skills list",
			Enum:        t.registry.Names(),
		},
	}
}

func (t *skillTool) ExecutionMode(args map[string]any) core.ExecutionMode {
	return core.ModeSequential
}

func (t *skillTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("skill name is required")
	}

	sk, ok := t.registry.Get(name)
	if !ok {
		return "", fmt.Errorf("skill not found: %s (available: %s)", name, t.registry.namesString())
	}

	// Keep loaded skills reloadable: compaction can remove the original tool
	// result while the registry's conversation-level loaded flag survives.
	if !t.registry.IsLoaded(name) && t.onLoad != nil {
		t.onLoad(name)
	}

	return FormatForContext(sk), nil
}
