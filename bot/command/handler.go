package command

import (
	ctxmgr "nekocode/bot/contextmgr"
)

// Handler owns the command parser and skill state: command registration,
// execution, and skill selection in one facade.
type Handler struct {
	parser *Parser
	skill  *skillState
}

// NewHandler creates the parser and skill state and registers all built-in
// and dynamic skill commands from deps.
func NewHandler(deps Deps) *Handler {
	h := &Handler{parser: NewParser(), skill: &skillState{MsgStart: -1}}
	h.RegisterAll(deps)
	return h
}

// RegisterAll (re)registers built-in and dynamic skill commands, e.g. after
// a configuration reload. The skill state is preserved.
func (h *Handler) RegisterAll(deps Deps) {
	registerAll(h.parser, deps, h.skill)
}

// Parser exposes the command parser so extension facades can register
// plugin and session commands.
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
func (h *Handler) Execute(input string, ctxMgr *ctxmgr.Manager) (resp string, handled bool) {
	h.skill.WantsAgent = false
	cmd := h.parser.Parse(input)
	if cmd.Name == "" {
		clearSkillContext(ctxMgr, h.skill)
		return "", false
	}
	resp, _ = h.parser.Execute(cmd)
	return resp, true
}

// DrainSkillHint returns and clears the pending skill hint and the
// wants-agent flag.
func (h *Handler) DrainSkillHint() (hint string, wantsAgent bool) {
	hint = h.skill.Hint
	wantsAgent = h.skill.WantsAgent
	h.skill.Hint = ""
	h.skill.WantsAgent = false
	return hint, wantsAgent
}

// SelectSkill loads a skill context into the conversation and records its
// message range so it can be removed later.
func (h *Handler) SelectSkill(ctxMgr *ctxmgr.Manager, context, name string) {
	clearSkillContext(ctxMgr, h.skill)
	h.skill.MsgStart = ctxMgr.Len()
	ctxMgr.Add("user", context)
	h.skill.MsgEnd = ctxMgr.Len()
	h.skill.Hint = name
}

// ClearSkill removes the selected skill's messages and resets its state.
func (h *Handler) ClearSkill(ctxMgr *ctxmgr.Manager) {
	clearSkillContext(ctxMgr, h.skill)
	h.skill.Hint = ""
	h.skill.WantsAgent = false
}
