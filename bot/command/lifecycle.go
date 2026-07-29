package command

import (
	"fmt"
	"strings"

	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/extension/skill"
	"nekocode/bot/policy"
	"nekocode/bot/prompt/planmode"
	"nekocode/bot/tools"
	"nekocode/bot/tools/runtime/core"
	"nekocode/util/text"
)

// skillState tracks the selected skill's message range and pending hints;
// it is owned by Handler and never leaves the command package.
type skillState struct {
	MsgStart   int
	MsgEnd     int
	WantsAgent bool
	Hint       string
}

type PlanModeController interface {
	SetPlanMode(bool)
}

type SkillProvider interface {
	SkillCommands() []skill.Command
	Skill(name string) (skill.Command, bool)
	MarkSkillLoaded(name string)
	ClearLoadedSkills()
}

type skillLoadCallbackTool interface {
	SetOnLoad(func(string))
}

// Deps bundles services needed by registration and lifecycle operations.
type Deps struct {
	CtxMgr       *ctxmgr.Manager
	Ag           func() PlanModeController // dynamic: returns current agent
	Skills       SkillProvider
	ToolRegistry *tools.Registry
	Policy       *policy.Policy
	GetConfigFn  func() (provider, model string)           // dynamic config for /config and /model
	ListModelsFn func() []string                           // available model names for /model
	SwitchModel  func(name string) (string, string, error) // /model callback
}

// registerAll wires built-in and dynamic slash commands.
func registerAll(p *Parser, deps Deps, st *skillState) {
	RegisterDefaults(p, deps)

	// /plan: enter read-only exploration mode.
	p.Register("plan", func(cmd *Command) (string, bool) {
		if len(cmd.Args) == 0 {
			return "Usage: /plan <task>", true
		}
		deps.Ag().SetPlanMode(true)
		deps.CtxMgr.SetSystemPrompt(planmode.Prompt(strings.Join(cmd.Args, " ")))
		deps.CtxMgr.Add("user", strings.Join(cmd.Args, " "))
		st.WantsAgent = true
		return "", false
	})

	// $skill-name for each loaded skill.
	for _, sk := range deps.Skills.SkillCommands() {
		name := sk.Name
		p.RegisterDynamic(name, func(cmd *Command) (string, bool) {
			sk, ok := deps.Skills.Skill(name)
			if !ok {
				return fmt.Sprintf("Skill %q not found.", name), true
			}
			st.MsgStart = deps.CtxMgr.Len()
			deps.CtxMgr.Add("user", sk.Context)
			deps.Skills.MarkSkillLoaded(name)
			if len(cmd.Args) == 0 {
				st.MsgStart = -1
				return fmt.Sprintf("Loaded skill %q.", name), true
			}
			deps.CtxMgr.Add("user", strings.Join(cmd.Args, " "))
			st.MsgEnd = deps.CtxMgr.Len()
			st.Hint = name
			st.WantsAgent = true
			return "", false
		})
	}

	// Skill tool OnLoad callback.
	if t, err := deps.ToolRegistry.Get("skill"); err == nil {
		if loader, ok := t.(skillLoadCallbackTool); ok {
			loader.SetOnLoad(func(name string) {
				deps.Skills.MarkSkillLoaded(name)
			})
		}
	}
}

// SummarizeIfNeeded compacts context if usage exceeds budget.
func SummarizeIfNeeded(ctxMgr *ctxmgr.Manager) {
	if ctxMgr.NeedsSummarization() {
		_ = ctxMgr.Summarize()
	}
}

// ForceSummarize compacts context now. When force is true, bypasses
// the NeedsSummarization token-budget check (for explicit user invocation).
func ForceSummarize(ctxMgr *ctxmgr.Manager, force bool) (string, error) {
	count, tokens, hasSummary := ctxMgr.Stats()
	if count <= 2 {
		return "Conversation too short, nothing to compact.", nil
	}
	if !force && !ctxMgr.NeedsSummarization() {
		return fmt.Sprintf("Not needed: %d messages, ~%d tokens", count, tokens), nil
	}
	if err := ctxMgr.Summarize(); err != nil {
		return "", err
	}
	_, newTokens, _ := ctxMgr.Stats()
	if newTokens >= tokens {
		return fmt.Sprintf("Already compact: %d messages, ~%d tokens", count, tokens), nil
	}
	action := "Compacted"
	if hasSummary {
		action = "Summary updated"
	}
	return fmt.Sprintf("%s: %d messages, ~%d → ~%d tokens", action, count, tokens, newTokens), nil
}

// ContextStats returns a one-line conversation size summary with a colored bar.
func ContextStats(ctxMgr *ctxmgr.Manager) string {
	r := ctxMgr.Report()
	used := r.SystemPrompt + r.ToolDefTokens + r.TodoText + r.SkillList + r.Messages
	free := r.Budget - used
	if free < 0 {
		free = 0
	}
	bar := ctxmgr.BuildBar(r.Budget, []ctxmgr.BarSegment{
		{Size: r.SystemPrompt, Kind: "sys"},
		{Size: r.ToolDefTokens, Kind: "tools"},
		{Size: r.TodoText, Kind: "todo"},
		{Size: r.SkillList, Kind: "skills"},
		{Size: r.Messages, Kind: "msgs"},
		{Size: free, Kind: "free"},
	}, 20)
	return fmt.Sprintf("%s  %s / %s", bar, text.FormatTokens(used), text.FormatTokens(r.Budget))
}

// ContextReport returns a detailed context window breakdown.
func ContextReport(ctxMgr *ctxmgr.Manager, toolDescs []core.Descriptor) string {
	r := ctxMgr.Report()
	r.ToolDefCount = len(toolDescs)
	r.ToolDefTokens = EstimateToolDefTokens(toolDescs)
	return ctxmgr.FormatContextReport(r)
}

// ForceFreshStart archives current conversation and starts a new session.
func ForceFreshStart(ctxMgr *ctxmgr.Manager, skills SkillProvider, gov *policy.Policy) (string, error) {
	count, oldTokens, _ := ctxMgr.Stats()
	skills.ClearLoadedSkills()
	// Reset policy run state so hook latches do not cross /new boundaries.
	if gov != nil {
		gov.ResetRun()
	}
	if count <= 2 {
		ctxMgr.FreshStart()
		return "New session started.", nil
	}
	if ctxMgr.NeedsSummarization() {
		if err := ctxMgr.Summarize(); err != nil {
			return "", err
		}
	}
	ctxMgr.FreshStart()
	_, newTokens, hasSummary := ctxMgr.Stats()
	d := "no summary"
	if hasSummary {
		d = "with summary"
	}
	return fmt.Sprintf("%d messages, ~%d tokens → %s (~%d tokens)", count, oldTokens, d, newTokens), nil
}

// clearSkillContext removes skill messages from the previous turn.
func clearSkillContext(ctxMgr *ctxmgr.Manager, st *skillState) {
	if st.MsgStart < 0 || st.MsgEnd <= st.MsgStart {
		return
	}
	ctxMgr.RemoveMessages(st.MsgStart, st.MsgEnd-1)
	st.MsgStart = -1
	st.MsgEnd = 0
}

func EstimateToolDefTokens(descs []core.Descriptor) int {
	n := 0
	for _, d := range descs {
		n += len(d.Name) + len(d.Description) + 80
		for _, p := range d.Parameters {
			n += len(p.Name) + len(p.Description) + len(p.Type) + 20
		}
	}
	return n / 4
}
