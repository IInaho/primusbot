package subagent

import (
	_ "embed"
	"fmt"
	"os"

	goyaml "gopkg.in/yaml.v3"

	"nekocode/bot/extension/tool/runtime/execution"
	"nekocode/bot/policy"
	"nekocode/bot/prompt"
	providertypes "nekocode/bot/provider/types"
	"nekocode/protocol"
	"nekocode/util/registry"
	"nekocode/util/yaml"
)

//go:embed prompts/executor.md
var executorPrompt string

//go:embed prompts/verify.md
var verifyPrompt string

//go:embed prompts/researcher.md
var researcherPrompt string

type AgentType struct {
	Name         string
	SystemPrompt string
	Tools        []string
}

// ToolCallEvent is fired for each tool executed inside a sub-agent.
type ToolCallEvent struct {
	Action   protocol.StepAction
	CallID   string
	ToolName string
	ToolArgs string
	Output   string
	IsError  bool
}

type RunConfig struct {
	Prompt             string
	AgentType          AgentType
	Thoroughness       string
	ContextWindow      int
	AutoCompactPercent int
	OnPhase            func(phase string)
	AddTokens          func(prompt, compl int)
	RecordLLMUsage     func(usage providertypes.StreamUsage)
	ConfirmFn          protocol.ConfirmFunc
	// FullAccess, when non-nil and returning true, puts the sub-agent's tool
	// executor into full-takeover mode (no approval prompts), mirroring the
	// main agent's permission mode.
	FullAccess func() bool
	Handoff    string                 // unverified prior-agent evidence prepended to the delegated task
	OnToolCall func(ev ToolCallEvent) // sub-agent tool execution callback
	ToolState  *execution.ExecutionState
	// Environment, when non-nil, is evaluated for every model call so roots
	// approved while the parent run is active become visible immediately.
	Environment prompt.EnvironmentProvider
	// Policy, when non-nil, is the main agent's shared governance handle:
	// sub-agent tool calls are recorded into the same ledger and
	// exploration budget.
	Policy *policy.Policy
}

var (
	builtins = registry.New[AgentType](func(a AgentType) string { return a.Name })
	plugins  = registry.New[AgentType](func(a AgentType) string { return a.Name })
)

func register(a AgentType) { builtins.Register(a) }

func init() {
	register(AgentType{
		Name: "executor", SystemPrompt: executorPrompt,
		Tools: []string{"read", "write", "edit", "shell", "process", "grep", "glob", "list"},
	})
	register(AgentType{
		Name: "verify", SystemPrompt: verifyPrompt,
		Tools: []string{"read", "grep", "glob", "list", "shell", "process", "web_search", "web_fetch"},
	})
	register(AgentType{
		Name: "researcher", SystemPrompt: researcherPrompt,
		Tools: []string{"read", "grep", "glob", "list", "web_search", "web_fetch"},
	})
}

// RegisterPlugin registers a plugin-provided agent type.
func RegisterPlugin(a AgentType) { plugins.Register(a) }

// UnregisterPlugin removes a plugin-provided agent type by name.
func UnregisterPlugin(name string) { plugins.Unregister(name) }

// Get looks up an agent type by name, checking builtins first, then plugins.
func Get(name string) (AgentType, bool) {
	if a, ok := builtins.Get(name); ok {
		return a, ok
	}
	return plugins.Get(name)
}

// AgentDef is parsed from an agents/*.md file (Claude Code format).
type AgentDef struct {
	Name         string   `yaml:"name"`
	Tools        []string `yaml:"tools"`
	SystemPrompt string   // markdown body (after frontmatter)
}

// ParseAgentMD parses a single agents/*.md file.
func ParseAgentMD(path string) (*AgentDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent file: %w", err)
	}
	yamlBytes, body, err := yaml.ParseYAMLFrontmatter(string(data))
	if err != nil {
		return nil, err
	}
	var def AgentDef
	if err := goyaml.Unmarshal(yamlBytes, &def); err != nil {
		return nil, fmt.Errorf("invalid frontmatter: %w", err)
	}
	if def.Name == "" {
		return nil, fmt.Errorf("missing required field: name")
	}
	def.SystemPrompt = body
	return &def, nil
}

// ToAgentType converts an AgentDef to AgentType for the subagent engine.
func (d *AgentDef) ToAgentType() AgentType {
	return AgentType{
		Name:         d.Name,
		SystemPrompt: d.SystemPrompt,
		Tools:        d.Tools,
	}
}
