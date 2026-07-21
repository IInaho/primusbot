package view

import (
	"nekocode/bot/config"
	"nekocode/bot/contextmgr"
	"nekocode/bot/provider/types"
	"nekocode/bot/session"
	commonview "nekocode/common/view"
)

// CmdResult tells the UI what to do after a command is executed.
type CmdResult = commonview.CmdResult

const (
	CmdNone           = commonview.CmdNone
	CmdHandled        = commonview.CmdHandled
	CmdConfirming     = commonview.CmdConfirming
	CmdSessionResumed = commonview.CmdSessionResumed
)

// ConfigView is the UI-facing view of the bot config.
type ConfigView = commonview.ConfigView

// MemoryScope is the scope of a memory view.
type MemoryScope = commonview.MemoryScope

const (
	MemoryScopeProject = commonview.MemoryScopeProject
)

// MemoryView is the UI-facing memory snapshot.
type MemoryView = commonview.MemoryView

// DisplayMessage is a lightweight message representation for UI history.
type DisplayMessage = commonview.DisplayMessage

// SessionMeta is a lightweight descriptor for a persisted session.
type SessionMeta = commonview.SessionMeta

var NewSessionMeta = commonview.NewSessionMeta

// ContextSnapshot is the structured context status consumed by UI surfaces.
type ContextSnapshot = commonview.ContextSnapshot

// BotStats carries runtime statistics from the bot to UI surfaces.
type BotStats = commonview.BotStats

// BotStatsInput is the raw input for building BotStats.
type BotStatsInput = commonview.BotStatsInput

var NewBotStats = commonview.NewBotStats

// DisplayBlock carries a persistent tool result for UI rendering.
type DisplayBlock = commonview.DisplayBlock

// ImageRef carries a generated image reference for UI rendering.
type ImageRef = commonview.ImageRef

// ContextSegment describes one visible part of the active context window.
type ContextSegment = commonview.ContextSegment

// RunCallbacks carries streaming callbacks from bot to UI.
type RunCallbacks = commonview.RunCallbacks

// ControlCallbacks carries control callbacks from UI to bot.
type ControlCallbacks = commonview.ControlCallbacks

// ConfirmRequest is sent to UI clients when a tool requires user approval.
type ConfirmRequest = commonview.ConfirmRequest

// ConfirmKind classifies a confirmation request.
type ConfirmKind = commonview.ConfirmKind

const (
	ConfirmKindPermission = commonview.ConfirmKindPermission
	ConfirmKindInstall    = commonview.ConfirmKindInstall
)

// ConfirmReply is the user's answer to a confirmation request.
type ConfirmReply = commonview.ConfirmReply

var (
	AllowOnce         = commonview.AllowOnce
	AllowRemembered   = commonview.AllowRemembered
	Deny              = commonview.Deny
	NewConfirmRequest = commonview.NewConfirmRequest
)

// ConfirmFunc asks the user to approve a tool call.
type ConfirmFunc = commonview.ConfirmFunc

// PhaseFunc is called when the agent's phase changes.
type PhaseFunc = commonview.PhaseFunc

const (
	PhaseReady     = commonview.PhaseReady
	PhaseWaiting   = commonview.PhaseWaiting
	PhaseThinking  = commonview.PhaseThinking
	PhaseReasoning = commonview.PhaseReasoning
	PhaseRunning   = commonview.PhaseRunning
)

// TodoFunc is called when the todo list changes.
type TodoFunc = commonview.TodoFunc

// TodoItem is a single todo entry.
type TodoItem = commonview.TodoItem

var (
	CountCompleted = commonview.CountCompleted
	TodoStatusIcon = commonview.TodoStatusIcon
)

// QuestionRequest is sent to UI clients when the agent needs user input.
type QuestionRequest = commonview.QuestionRequest

// QuestionReply carries one answer array per question.
type QuestionReply = commonview.QuestionReply

var NewQuestionRequest = commonview.NewQuestionRequest

// QuestionFunc asks the user one or more questions and returns the selected answers.
type QuestionFunc = commonview.QuestionFunc

// QuestionOption is one selectable answer shown to the user.
type QuestionOption = commonview.QuestionOption

// QuestionItem describes one question in a tool request.
type QuestionItem = commonview.QuestionItem

// SubSlot tracks an active sub-agent for rendering and slot management.
type SubSlot = commonview.SubSlot

// StepAction identifies the kind of step emitted during a run.
type StepAction = commonview.StepAction

const (
	StepActionChat           = commonview.StepActionChat
	StepActionThink          = commonview.StepActionThink
	StepActionToolStart      = commonview.StepActionToolStart
	StepActionToolBlocked    = commonview.StepActionToolBlocked
	StepActionToolPreview    = commonview.StepActionToolPreview
	StepActionExecuteTool    = commonview.StepActionExecuteTool
	StepActionSubToolStart   = commonview.StepActionSubToolStart
	StepActionSubExecuteTool = commonview.StepActionSubExecuteTool
	StepActionSubAgentStart  = commonview.StepActionSubAgentStart
	StepActionSubAgentEnd    = commonview.StepActionSubAgentEnd
)

// SkillManagementView is the top-level DTO for the skill/plugin/MCP management UI.
type SkillManagementView = commonview.SkillManagementView

// SkillView is a single skill in the management view.
type SkillView = commonview.SkillView

// PluginView is a single plugin in the management view.
type PluginView = commonview.PluginView

// MCPServerView is a single MCP server in the management view.
type MCPServerView = commonview.MCPServerView

// MCPServerViewInput is the input for building an MCP server view.
type MCPServerViewInput = commonview.MCPServerViewInput

// MCPHealth represents the health state of an MCP server.
type MCPHealth = commonview.MCPHealth

var (
	NewMCPServerView = commonview.NewMCPServerView
	ApplyMCPHealth   = commonview.ApplyMCPHealth
)

// NewConfigView builds a ConfigView from the loaded config.
func NewConfigView(cfg config.Config) ConfigView {
	return ConfigView{
		Path:           config.Path(),
		Exists:         config.Exists(),
		Active:         cfg.Active,
		ContextWindow:  cfg.ContextWindow,
		FlashModel:     cfg.FlashModel,
		Models:         modelConfigsToView(cfg.Models),
		ImageGenModels: imageGenConfigsToView(cfg.ImageGenModels),
		MCPServers:     mcpServerConfigsToView(cfg.MCPServers),
		Permissions:    permissionsConfigToView(cfg.Permissions),
		Workspaces:     workspaceConfigsToView(cfg.Workspaces),
	}
}

// ToConfig converts a ConfigView back to the config model.
func ToConfig(v ConfigView) config.Config {
	return config.Config{
		Active:         v.Active,
		ContextWindow:  v.ContextWindow,
		FlashModel:     v.FlashModel,
		Models:         modelConfigsFromView(v.Models),
		ImageGenModels: imageGenConfigsFromView(v.ImageGenModels),
		MCPServers:     mcpServerConfigsFromView(v.MCPServers),
		Permissions:    permissionsConfigFromView(v.Permissions),
		Workspaces:     workspaceConfigsFromView(v.Workspaces),
	}
}

// SessionMetas converts session metadata to view DTOs.
func SessionMetas(list []session.Meta) []SessionMeta {
	out := make([]SessionMeta, 0, len(list))
	for _, m := range list {
		out = append(out, commonview.NewSessionMeta(m.ID, m.CWD, m.CreatedAt, m.UpdatedAt, m.MsgCount))
	}
	return out
}

// NewContextSnapshot builds a ContextSnapshot from a context report.
func NewContextSnapshot(r contextmgr.ContextReport) ContextSnapshot {
	used := r.SystemPrompt + r.ToolDefTokens + r.TodoText + r.SkillList + r.Messages
	free := max(r.Budget-used, 0)
	percentUsed := 0.0
	if r.Budget > 0 {
		percentUsed = min(float64(used)/float64(r.Budget), 1)
	}

	return ContextSnapshot{
		Budget:          r.Budget,
		Used:            used,
		Free:            free,
		PercentUsed:     percentUsed,
		SystemPrompt:    r.SystemPrompt,
		ToolDefTokens:   r.ToolDefTokens,
		TodoText:        r.TodoText,
		SkillList:       r.SkillList,
		MessageTokens:   r.Messages,
		ToolDefCount:    r.ToolDefCount,
		MessageCount:    r.UserMessages + r.AssistantMsgs + r.ToolResults,
		UserMessages:    r.UserMessages,
		AssistantMsgs:   r.AssistantMsgs,
		ToolResults:     r.ToolResults,
		Archived:        r.Archived,
		CompactCount:    r.CompactCount,
		TrimCount:       r.TrimCount,
		CacheHitTokens:  r.CacheHitTokens,
		CacheMissTokens: r.CacheMissTokens,
		CacheHitRatio:   r.CacheHitRatio,
		SubCount:        r.SubCount,
		SubTokens:       r.SubTokens,
		SubCacheHit:     r.SubCacheHit,
		SubCacheMiss:    r.SubCacheMiss,
		Segments: []ContextSegment{
			{Key: "system", Label: "系统提示", Tokens: r.SystemPrompt, Tone: "muted"},
			{Key: "tools", Label: "工具定义", Tokens: r.ToolDefTokens, Tone: "blue"},
			{Key: "todo", Label: "待办", Tokens: r.TodoText, Tone: "orange"},
			{Key: "skills", Label: "Skills", Tokens: r.SkillList, Tone: "yellow"},
			{Key: "messages", Label: "对话消息", Tokens: r.Messages, Tone: "violet"},
			{Key: "free", Label: "剩余", Tokens: free, Tone: "free"},
		},
	}
}

// DisplayMessages converts provider messages to UI display messages.
func DisplayMessages(messages []types.Message, compactBoundary int) []DisplayMessage {
	return displayMessages(messages, compactBoundary)
}

// NewMemoryView builds a MemoryView from raw content.
func NewMemoryView(scope MemoryScope, path, content string) MemoryView {
	return commonview.NewMemoryView(scope, path, content)
}
