package command

import (
	"context"
	"fmt"
	"strings"

	"nekocode/bot/config"
	ctxmgr "nekocode/bot/contextmgr"
	"nekocode/bot/extension/skill"
	"nekocode/bot/policy"
	"nekocode/bot/tools"
	"nekocode/bot/tools/runtime/core"
	"nekocode/util/text"
)

// skillState tracks the selected skill's message range and continuation;
// it is owned by Handler and never leaves the command package.
type skillState struct {
	MsgStart   int
	MsgEnd     int
	WantsAgent bool
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
	GetConfigFn  func() config.ModelConfig
	ListModelsFn func() []string
	SwitchModel  func(string) error
}

// registerAll wires built-in and dynamic slash commands.
func registerAll(p *Parser, deps Deps, st *skillState) {
	RegisterDefaults(p, deps)

	// /plan: enter read-only exploration mode.
	p.Register("plan", func(_ context.Context, cmd *Command) (string, bool) {
		if len(cmd.Args) == 0 {
			return "Usage: /plan <task>", true
		}
		deps.Ag().SetPlanMode(true)
		deps.CtxMgr.SetSystemPrompt(planModePrompt(strings.Join(cmd.Args, " ")))
		deps.CtxMgr.Add("user", strings.Join(cmd.Args, " "))
		st.WantsAgent = true
		return "", false
	})

	// $skill-name for each loaded skill.
	for _, sk := range deps.Skills.SkillCommands() {
		name := sk.Name
		p.RegisterDynamic(name, func(_ context.Context, cmd *Command) (string, bool) {
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

func planModePrompt(task string) string {
	return `<plan-mode>
You are in PLAN MODE. You are a software architect performing READ-ONLY analysis.

AVAILABLE TOOLS: read, grep, glob, list, web_search, web_fetch (read-only tools).
BLOCKED: write, edit, bash (writing/modifying), task(executor).

Your task:
` + task + `

WORKFLOW:
1. Explore the codebase — understand the architecture, identify key files
2. Design an implementation approach — prefer the SIMPLEST design that meets
   the request. Avoid speculative abstractions, unrequested configurability,
   and defensive code for impossible cases.
3. Present your plan clearly:
   - Summary of what needs to change
   - Files to create / modify / delete (with paths)
   - Step-by-step implementation order
   - Per-step verification check (e.g. "after step 2, run: go test ./...")
   - Risks, edge cases
   - Critical Files for Implementation (3-5 most important files)
   - Explicit assumptions: list any assumption you're making; if multiple
     interpretations exist, present them and ask the user to pick
4. Surgical scope: touch only what the request requires — flag any adjacent
   cleanup you intentionally did NOT do.

After presenting the plan, say "Ready to implement — approve?" or similar.
Once the user approves, you will exit plan mode and can write code.
Do NOT write any code in plan mode — design only.
</plan-mode>`
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

// ContextReport returns a detailed context window breakdown.
func ContextReport(ctxMgr *ctxmgr.Manager, toolDescs []core.Descriptor) string {
	r := ctxMgr.Report()
	r.ToolDefCount = len(toolDescs)
	r.ToolDefTokens = EstimateToolDefTokens(toolDescs)
	return formatContextReport(r)
}

type barSegment struct {
	size int
	kind string
}

var barChars = map[string]string{
	"sys": "⛁", "tools": "⛁", "todo": "⛀", "skills": "⛀", "msgs": "⛁", "free": "⛶",
	"cache": "⛂", "sub": "⛃",
}

func formatContextReport(r ctxmgr.ContextReport) string {
	used := r.SystemPrompt + r.ToolDefTokens + r.TodoText + r.SkillList + r.Messages
	free := max(r.Budget-used, 0)
	pct := func(n int) string {
		if r.Budget == 0 {
			return ""
		}
		return fmt.Sprintf("(%.0f%%)", float64(n)/float64(r.Budget)*100)
	}
	item := func(ch, label string, n int) string {
		return barChars[ch] + " " + label + ": " + text.FormatTokens(n) + " " + pct(n)
	}

	bar := buildBar(r.Budget, []barSegment{
		{size: r.SystemPrompt, kind: "sys"},
		{size: r.ToolDefTokens + r.TodoText, kind: "tools"},
		{size: r.SkillList, kind: "skills"},
		{size: r.Messages, kind: "msgs"},
		{size: free, kind: "free"},
	}, 20)
	out := fmt.Sprintf("%s  %s / %s\n\n%s  %s\n%s  %s\n\n%s",
		bar, text.FormatTokens(used), text.FormatTokens(r.Budget),
		item("sys", "System", r.SystemPrompt),
		item("tools", "Tools", r.ToolDefTokens),
		item("msgs", "Messages", r.Messages),
		item("skills", "Skills", r.SkillList),
		fmt.Sprintf("%d tools · %d msgs · %d archived  %s Free: %s",
			r.ToolDefCount, r.UserMessages+r.AssistantMsgs+r.ToolResults, r.Archived,
			text.FormatTokens(free), pct(free)),
	)
	if r.CacheHitTokens > 0 || r.CacheMissTokens > 0 {
		out += fmt.Sprintf("\n%s Cache: hit %s / miss %s · %.0f%%",
			barChars["cache"],
			text.FormatTokens(r.CacheHitTokens), text.FormatTokens(r.CacheMissTokens),
			r.CacheHitRatio*100)
	}
	if r.SubCount > 0 {
		subRatio := ""
		if total := r.SubCacheHit + r.SubCacheMiss; total > 0 {
			subRatio = fmt.Sprintf(" · hit %.0f%%", float64(r.SubCacheHit)/float64(total)*100)
		}
		out += fmt.Sprintf("\n%s Subagents: %d runs · %s tokens · hit %s / miss %s%s",
			barChars["sub"], r.SubCount,
			text.FormatTokens(r.SubTokens), text.FormatTokens(r.SubCacheHit),
			text.FormatTokens(r.SubCacheMiss), subRatio)
	}
	return out
}

func buildBar(total int, segments []barSegment, width int) string {
	if total <= 0 {
		return ""
	}
	allocated := make([]int, len(segments))
	remaining := width
	for i, segment := range segments {
		if segment.size <= 0 {
			continue
		}
		allocated[i] = max(segment.size*width/total, 1)
		remaining -= allocated[i]
	}
	for i := len(segments) - 1; i >= 0 && remaining > 0; i-- {
		if segments[i].size > 0 {
			allocated[i] += remaining
			break
		}
	}
	var out strings.Builder
	for i, segment := range segments {
		char := barChars[segment.kind]
		if char == "" {
			char = " "
		}
		for range allocated[i] {
			fmt.Fprintf(&out, "%s ", char)
		}
	}
	return out.String()
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
