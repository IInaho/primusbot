package extension

import commonview "nekocode/common/view"

// SkillManagementView is the top-level DTO for the skill/plugin/MCP management UI.
type SkillManagementView = commonview.SkillManagementView
type SkillView = commonview.SkillView
type PluginView = commonview.PluginView
type MCPServerView = commonview.MCPServerView
type MCPServerViewInput = commonview.MCPServerViewInput
type MCPHealth = commonview.MCPHealth

var (
	NewMCPServerView = commonview.NewMCPServerView
	ApplyMCPHealth   = commonview.ApplyMCPHealth
)
