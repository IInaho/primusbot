package view

// CmdResult tells the UI what to do after a command is executed.
type CmdResult int

const (
	CmdNone           CmdResult = iota // no command matched, start agent
	CmdHandled                         // command handled, no further action
	CmdConfirming                      // command handled, wait for confirmation
	CmdSessionResumed                  // session resumed, UI should reload messages
)

// SessionMeta is a lightweight descriptor for a persisted session.
type SessionMeta struct {
	ID        string `json:"id"`
	CWD       string `json:"cwd"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
	MsgCount  int    `json:"msgCount"`
}

// ConfigView is the UI-facing view of the bot config.
type ConfigView struct {
	Path           string                     `json:"path"`
	Exists         bool                       `json:"exists"`
	Active         string                     `json:"active"`
	ContextWindow  int                        `json:"context_window"`
	FlashModel     string                     `json:"flash_model,omitempty"`
	Models         []ModelConfig              `json:"models"`
	ImageGenModels []ImageGenConfig           `json:"image_gen_models,omitempty"`
	MCPServers     map[string]MCPServerConfig `json:"mcp_servers,omitempty"`
	Permissions    *PermissionsConfig         `json:"permissions,omitempty"`
	Workspaces     []WorkspaceConfig          `json:"workspaces,omitempty"`
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

// DisplayBlock carries a persistent tool result for UI rendering.
type DisplayBlock struct {
	ToolName string
	Args     string
	Content  string
	IsError  bool
}

// ImageRef carries a generated image reference for UI rendering.
type ImageRef struct {
	Path   string
	URL    string
	Width  int
	Height int
}

// DisplayMessage is a lightweight message representation for UI history.
type DisplayMessage struct {
	Role    string
	Content string
	Blocks  []DisplayBlock
	Images  []ImageRef
}

// SubSlot tracks an active sub-agent for rendering and slot management.
type SubSlot struct {
	ID       string
	SubType  string
	ColorIdx int
}

// BotStats carries runtime statistics from the bot to UI surfaces.
type BotStats struct {
	PromptTokens, CompletionTokens int
	TurnPrompt, TurnCompletion     int
	ContextTokens, CompactCount    int
	Duration                       string
}

// ContextSegment describes one visible part of the active context window.
type ContextSegment struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Tokens int    `json:"tokens"`
	Tone   string `json:"tone"`
}

// ContextSnapshot is the structured context status consumed by UI surfaces.
type ContextSnapshot struct {
	Budget          int              `json:"budget"`
	Used            int              `json:"used"`
	Free            int              `json:"free"`
	PercentUsed     float64          `json:"percentUsed"`
	SystemPrompt    int              `json:"systemPrompt"`
	ToolDefTokens   int              `json:"toolDefTokens"`
	TodoText        int              `json:"todoText"`
	SkillList       int              `json:"skillList"`
	MessageTokens   int              `json:"messageTokens"`
	ToolDefCount    int              `json:"toolDefCount"`
	MessageCount    int              `json:"messageCount"`
	UserMessages    int              `json:"userMessages"`
	AssistantMsgs   int              `json:"assistantMsgs"`
	ToolResults     int              `json:"toolResults"`
	Archived        int              `json:"archived"`
	CompactCount    int              `json:"compactCount"`
	TrimCount       int              `json:"trimCount"`
	CacheHitTokens  int              `json:"cacheHitTokens"`
	CacheMissTokens int              `json:"cacheMissTokens"`
	CacheHitRatio   float64          `json:"cacheHitRatio"`
	SubCount        int              `json:"subCount"`
	SubTokens       int              `json:"subTokens"`
	SubCacheHit     int              `json:"subCacheHit"`
	SubCacheMiss    int              `json:"subCacheMiss"`
	Segments        []ContextSegment `json:"segments"`
}
