// Package ui holds data-transfer types shared between bot and UI layers.
package ui

// ConfigView is the UI-facing view of the bot config.
type ConfigView struct {
	Path           string            `json:"path"`
	Exists         bool              `json:"exists"`
	Active         string            `json:"active"`
	ContextWindow  int               `json:"context_window"`
	FlashModel     string            `json:"flash_model,omitempty"`
	Models         []ModelConfig     `json:"models"`
	ImageGenModels []ImageGenConfig  `json:"image_gen_models,omitempty"`
	MCPServers     map[string]MCPServerConfig `json:"mcp_servers,omitempty"`
	Permissions    *PermissionsConfig `json:"permissions,omitempty"`
	Workspaces     []WorkspaceConfig `json:"workspaces,omitempty"`
}

type ModelConfig struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}

type ImageGenConfig struct {
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	APIKey    string `json:"api_key"`
	SecretKey string `json:"secret_key"`
	BaseURL   string `json:"base_url,omitempty"`
	Model     string `json:"model,omitempty"`
}

type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Enabled bool              `json:"enabled"`
}

type PermissionsConfig struct {
	Allow   []string                 `json:"allow,omitempty"`
	Ask     []string                 `json:"ask,omitempty"`
	Deny    []string                 `json:"deny,omitempty"`
	Sandbox map[string]SandboxConfig `json:"sandbox,omitempty"`
}

type SandboxConfig struct {
	SandboxMode   string   `json:"sandbox_mode,omitempty"`
	Network       bool     `json:"network,omitempty"`
	WritableRoots []string `json:"writable_roots,omitempty"`
}


type WorkspaceConfig struct {
	Path   string `json:"path"`
	Access string `json:"access,omitempty"`
}
