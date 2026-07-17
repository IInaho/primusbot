package view

import commonview "nekocode/common/view"

type CmdResult = commonview.CmdResult

const (
	CmdNone           = commonview.CmdNone
	CmdHandled        = commonview.CmdHandled
	CmdConfirming     = commonview.CmdConfirming
	CmdSessionResumed = commonview.CmdSessionResumed
)

type RunCallbacks = commonview.RunCallbacks

type ConfirmRequest = commonview.ConfirmRequest
type ConfirmKind = commonview.ConfirmKind
type ConfirmReply = commonview.ConfirmReply
type ConfirmFunc = commonview.ConfirmFunc
type PhaseFunc = commonview.PhaseFunc

const (
	ConfirmKindPermission = commonview.ConfirmKindPermission
	ConfirmKindInstall    = commonview.ConfirmKindInstall
	PhaseReady            = commonview.PhaseReady
	PhaseWaiting          = commonview.PhaseWaiting
	PhaseThinking         = commonview.PhaseThinking
	PhaseReasoning        = commonview.PhaseReasoning
	PhaseRunning          = commonview.PhaseRunning
)

var (
	AllowOnce         = commonview.AllowOnce
	AllowRemembered   = commonview.AllowRemembered
	Deny              = commonview.Deny
	NewConfirmRequest = commonview.NewConfirmRequest
)

type QuestionOption = commonview.QuestionOption
type QuestionItem = commonview.QuestionItem
type QuestionReply = commonview.QuestionReply
type QuestionRequest = commonview.QuestionRequest
type QuestionFunc = commonview.QuestionFunc

var NewQuestionRequest = commonview.NewQuestionRequest

type TodoItem = commonview.TodoItem
type TodoFunc = commonview.TodoFunc

var (
	CountCompleted = commonview.CountCompleted
	TodoStatusIcon = commonview.TodoStatusIcon
)

type BotStats = commonview.BotStats
type BotStatsInput = commonview.BotStatsInput

var NewBotStats = commonview.NewBotStats

type ConfigView = commonview.ConfigView
type ModelConfig = commonview.ModelConfig
type ImageGenConfig = commonview.ImageGenConfig
type MCPServerConfig = commonview.MCPServerConfig
type PermissionsConfig = commonview.PermissionsConfig
type SandboxConfig = commonview.SandboxConfig
type WorkspaceConfig = commonview.WorkspaceConfig

type DisplayBlock = commonview.DisplayBlock
type ImageRef = commonview.ImageRef
type DisplayMessage = commonview.DisplayMessage
type SubSlot = commonview.SubSlot
type ContextSegment = commonview.ContextSegment
type ContextSnapshot = commonview.ContextSnapshot

type SessionMeta = commonview.SessionMeta

var (
	NewSessionMeta = commonview.NewSessionMeta
)

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
