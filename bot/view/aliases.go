package view

import (
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/session"
	commonview "nekocode/common/view"
)

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

func SessionMetas(list []session.Meta) []SessionMeta {
	out := make([]SessionMeta, 0, len(list))
	for _, m := range list {
		out = append(out, NewSessionMeta(m.ID, m.CWD, m.CreatedAt, m.UpdatedAt, m.MsgCount))
	}
	return out
}

func NewContextSnapshot(r ctxmgr.ContextReport) ContextSnapshot {
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
