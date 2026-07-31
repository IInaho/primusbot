package command

import (
	"context"

	ctxmgr "nekocode/bot/contextmgr"
)

// Handler owns command registration, execution, and selected-skill state.
type Handler struct {
	parser *Parser
	skill  *skillState
}

// New creates a command handler and registers built-in and dynamic commands.
func New(deps Deps) *Handler {
	h := &Handler{parser: NewParser(), skill: &skillState{MsgStart: -1}}
	h.RegisterAll(deps)
	return h
}

// RegisterAll (re)registers built-in and dynamic skill commands, e.g. after
// a configuration reload. The skill state is preserved.
func (h *Handler) RegisterAll(deps Deps) {
	registerAll(h.parser, deps, h.skill)
}

// Parser exposes the command registry used by extension and session commands.
func (h *Handler) Parser() *Parser {
	return h.parser
}

// Names returns the sorted names of all registered commands.
func (h *Handler) Names() []string {
	return h.parser.Commands()
}

// Execute parses input and runs it as a command. handled is false when
// input is not a command — in that case any previous skill context is
// cleared from ctxMgr.
func (h *Handler) Execute(ctx context.Context, input string, ctxMgr *ctxmgr.Manager) (string, bool) {
	h.skill.WantsAgent = false
	cmd := h.parser.Parse(input)
	if cmd.Name == "" {
		clearSkillContext(ctxMgr, h.skill)
		return "", false
	}
	return h.parser.Execute(ctx, cmd)
}

// TakeContinuation returns and clears the command's optional agent
// continuation metadata.
func (h *Handler) TakeContinuation() (continueAgent bool) {
	continueAgent = h.skill.WantsAgent
	h.skill.WantsAgent = false
	return continueAgent
}

// SelectSkill loads a skill context into the conversation and records its
// message range so it can be removed later.
func (h *Handler) SelectSkill(ctxMgr *ctxmgr.Manager, context string) {
	clearSkillContext(ctxMgr, h.skill)
	h.skill.MsgStart = ctxMgr.Len()
	ctxMgr.Add("user", context)
	h.skill.MsgEnd = ctxMgr.Len()
}

// ClearSkill removes the selected skill's messages and resets its state.
func (h *Handler) ClearSkill(ctxMgr *ctxmgr.Manager) {
	clearSkillContext(ctxMgr, h.skill)
	h.skill.WantsAgent = false
}

// ResetSkill forgets selected-skill bookkeeping after a session context has
// been replaced.
func (h *Handler) ResetSkill() {
	h.skill.MsgStart = -1
	h.skill.MsgEnd = 0
	h.skill.WantsAgent = false
}
