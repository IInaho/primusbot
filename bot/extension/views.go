package extension

// SkillManagementView is the top-level DTO for the skill/plugin/MCP management UI.
type SkillManagementView struct {
	Skills  []SkillView     `json:"skills"`
	Plugins []PluginView    `json:"plugins"`
	MCP     []MCPServerView `json:"mcp"`
}

type SkillView struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Dir         string   `json:"dir,omitempty"`
	Files       []string `json:"files,omitempty"`
	Loaded      bool     `json:"loaded"`
	Source      string   `json:"source"`
	SourceKind  string   `json:"sourceKind"`
	Plugin      string   `json:"plugin,omitempty"`
}

type PluginView struct {
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	Source      string   `json:"source,omitempty"`
	Dir         string   `json:"dir,omitempty"`
	Enabled     bool     `json:"enabled"`
	Skills      []string `json:"skills,omitempty"`
	SkillNames  []string `json:"skillNames,omitempty"`
	Agents      []string `json:"agents,omitempty"`
	Commands    []string `json:"commands,omitempty"`
	MCPServers  []string `json:"mcpServers,omitempty"`
	HasHooks    bool     `json:"hasHooks,omitempty"`
}

type MCPServerView struct {
	Name          string   `json:"name"`
	Plugin        string   `json:"plugin"`
	Command       string   `json:"command"`
	Args          []string `json:"args,omitempty"`
	PluginEnabled bool     `json:"pluginEnabled"`
	Status        string   `json:"status,omitempty"`
	Error         string   `json:"error,omitempty"`
	ToolCount     int      `json:"toolCount,omitempty"`
}

type MCPServerViewInput struct {
	Name    string
	Plugin  string
	Command string
	Args    []string
	Enabled bool
}

type MCPHealth struct {
	Status    string
	Error     string
	ToolCount int
}

func NewMCPServerView(in MCPServerViewInput) MCPServerView {
	view := MCPServerView{
		Name:          in.Name,
		Plugin:        in.Plugin,
		Command:       in.Command,
		Args:          append([]string(nil), in.Args...),
		PluginEnabled: in.Enabled,
	}
	if !in.Enabled {
		view.Status = "disabled"
	}
	return view
}

func ApplyMCPHealth(servers []MCPServerView, health map[string]MCPHealth) {
	for i := range servers {
		if !servers[i].PluginEnabled {
			servers[i].Status = "disabled"
			continue
		}
		h, ok := health[servers[i].Name]
		if !ok {
			servers[i].Status = "unknown"
			continue
		}
		servers[i].Status = h.Status
		servers[i].Error = h.Error
		servers[i].ToolCount = h.ToolCount
	}
}
