// Package permission provides a unified, declarative permission rule engine
// for all tools — inspired by Claude Code's `allow`/`ask`/`deny` model.
//
// Rules follow the shape  Tool(specifier)  where the specifier is matched by
// a tool-specific SpecifierMatcher (bash parses command globs, edit matches
// gitignore path patterns, web_fetch matches domains, ...). The engine itself
// is tool-agnostic: it only knows Effect (deny/ask/allow) and the evaluation
// order deny → ask → allow, first match wins.
//
// Sandbox (OS isolation) is a separate, bash-only layer; this permission
// layer is the universal "can the tool run at all / should we prompt" gate
// that applies to every tool call before execution.

package permission

import (
	"fmt"
	"strings"
	"time"
)

// Effect is the outcome of a permission rule.
type Effect int

const (
	EffectDeny  Effect = iota // block the call outright
	EffectAsk                 // prompt the user; remember on approval
	EffectAllow               // run without prompting
)

func (e Effect) String() string {
	switch e {
	case EffectDeny:
		return "deny"
	case EffectAsk:
		return "ask"
	case EffectAllow:
		return "allow"
	}
	return "unknown"
}

// Rule is a single permission rule: "tool(specifier)" with an effect.
// An empty specifier matches every call of the tool (bare "Tool" form).
type Rule struct {
	Tool      string    `json:"tool"`
	Specifier string    `json:"specifier,omitempty"` // raw specifier text (e.g. "npm run *", "/src/**", "domain:github.com")
	Effect    Effect    `json:"effect"`
	Source    string    `json:"source,omitempty"`    // "builtin" | "user" | "project" | "remembered"
	Workspace string    `json:"workspace,omitempty"` // project root scope; empty → global (legacy)
	CreatedAt time.Time `json:"createdAt,omitempty"` // set when persisted
}

// SpecifierMatcher decodes a rule's specifier and decides whether a concrete
// tool call matches it. Each tool registers one matcher; the engine looks it
// up by tool name. The call descriptor is tool-specific (command string for
// bash, path for edit, domain for web_fetch, ...).
type SpecifierMatcher interface {
	// Match reports whether the call (described by callInfo) matches the rule
	// specifier. callInfo keys are tool-defined (e.g. {"command":"rm -rf /"}).
	Match(specifier string, callInfo map[string]any) (bool, error)
}

// Engine evaluates permission rules for tool calls.
type Engine struct {
	matchers     map[string]SpecifierMatcher
	rules        []Rule // ordered; evaluation is deny→ask→allow within the matched tool
	sandboxRules []SandboxRule
}

// NewEngine creates an engine with the given matchers (tool name → matcher).
func NewEngine(matchers map[string]SpecifierMatcher) *Engine {
	return &Engine{matchers: matchers}
}

// SetRules replaces the engine's rule set. Rules are kept in source order;
// Evaluate applies the deny→ask→allow precedence across them.
func (e *Engine) SetRules(rules []Rule) {
	e.rules = rules
}

// SetSandboxRules replaces the sandbox profile rule set. Sandbox rules never
// decide allow/ask/deny; callers use them only to attach an explicit sandbox
// request to a matching shell command before normal permission evaluation.
func (e *Engine) SetSandboxRules(rules []SandboxRule) {
	e.sandboxRules = rules
}

// SandboxFor returns the most specific sandbox profile matching a tool call.
func (e *Engine) SandboxFor(toolName string, callInfo map[string]any) (SandboxProfile, bool) {
	var best SandboxRule
	bestScore := -1
	for _, sr := range e.sandboxRules {
		if !e.ruleApplies(sr.Rule, toolName, callInfo) {
			continue
		}
		score := sandboxRuleSpecificity(sr.Rule)
		if score > bestScore {
			best = sr
			bestScore = score
		}
	}
	if bestScore >= 0 {
		return best.Profile, true
	}
	return SandboxProfile{}, false
}

func sandboxRuleSpecificity(r Rule) int {
	return len(strings.TrimSpace(r.Specifier))*2 + len(strings.TrimSpace(r.Tool))
}

// Decision is the outcome of evaluating a call.
type Decision struct {
	Effect Effect
	Rule   Rule // the rule that decided the outcome (zero value if no match)
}

// Evaluate decides what to do with a tool call. Precedence (highest first):
//
//  1. deny from ANY source (a deny can never be overridden)
//  2. ask declared by the user (declared/remembered) — the user explicitly
//     wants to be prompted
//  3. allow declared by the user (declared/remembered) — a remembered allow
//     overrides a builtin ask so "yes, don't ask again" actually sticks
//  4. builtin ask
//  5. builtin allow
//  6. defaultEffect (no rule matched)
//
// Tool names are compared case-insensitively so users can write "Bash(...)"
// (claude-code style) while the engine keys on the lowercase canonical name.
func (e *Engine) Evaluate(toolName string, callInfo map[string]any, defaultEffect Effect) Decision {
	// 1. deny from any source
	for _, r := range e.rules {
		if r.Effect == EffectDeny && e.ruleApplies(r, toolName, callInfo) {
			return Decision{Effect: EffectDeny, Rule: r}
		}
	}
	// 2. user-declared ask
	for _, r := range e.rules {
		if r.Effect == EffectAsk && !r.isBuiltin() && e.ruleApplies(r, toolName, callInfo) {
			return Decision{Effect: EffectAsk, Rule: r}
		}
	}
	// 3. user-declared allow (covers remembered allows → beats builtin ask)
	if d, ok := e.evaluateBashAllowCoverage(toolName, callInfo, false); ok {
		return d
	}
	for _, r := range e.rules {
		if r.Effect == EffectAllow && !r.isBuiltin() && e.ruleApplies(r, toolName, callInfo) {
			return Decision{Effect: EffectAllow, Rule: r}
		}
	}
	// 4. builtin ask
	for _, r := range e.rules {
		if r.Effect == EffectAsk && r.isBuiltin() && e.ruleApplies(r, toolName, callInfo) {
			return Decision{Effect: EffectAsk, Rule: r}
		}
	}
	// 5. builtin allow
	if d, ok := e.evaluateBashAllowCoverage(toolName, callInfo, true); ok {
		return d
	}
	for _, r := range e.rules {
		if r.Effect == EffectAllow && r.isBuiltin() && e.ruleApplies(r, toolName, callInfo) {
			return Decision{Effect: EffectAllow, Rule: r}
		}
	}
	return Decision{Effect: defaultEffect}
}

func (e *Engine) evaluateBashAllowCoverage(toolName string, callInfo map[string]any, builtinOnly bool) (Decision, bool) {
	if !strings.EqualFold(toolName, "bash") && !strings.EqualFold(toolName, "shell") {
		return Decision{}, false
	}
	cmd, _ := callInfo["command"].(string)
	subcmds := shellCommands(cmd)
	if len(subcmds) <= 1 {
		return Decision{}, false
	}
	// Bare "allow Bash" rules (empty specifier) match every command. Such a
	// rule — user-declared or builtin — covers all subcommands; without this
	// branch a declared "Bash" allow silently failed to cover compound
	// commands, leaving the engine to fall through to builtin ask rules and
	// re-prompting the user for a call the user had already allowed as a
	// whole.
	var bareAllow *Rule
	var allowRules []Rule
	for _, r := range e.rules {
		if r.Effect != EffectAllow || !toolNameMatches(r.Tool, toolName) {
			continue
		}
		if builtinOnly != r.isBuiltin() {
			continue
		}
		if r.Specifier == "" {
			if bareAllow == nil {
				rr := r
				bareAllow = &rr
			}
			continue
		}
		allowRules = append(allowRules, r)
	}
	if bareAllow != nil {
		return Decision{Effect: EffectAllow, Rule: *bareAllow}, true
	}
	if !builtinOnly {
		for _, r := range e.rules {
			if r.Effect == EffectAllow && r.isBuiltin() && toolNameMatches(r.Tool, toolName) && r.Specifier != "" {
				allowRules = append(allowRules, r)
			}
		}
	}
	if len(allowRules) == 0 {
		return Decision{}, false
	}
	var deciding Rule
	for _, sub := range subcmds {
		covered := false
		for _, r := range allowRules {
			if matchBashPattern(normalizeBashSpec(r.Specifier), sub) {
				covered = true
				if deciding.Tool == "" {
					deciding = r
				}
				break
			}
		}
		if !covered {
			return Decision{}, false
		}
	}
	return Decision{Effect: EffectAllow, Rule: deciding}, true
}

// isBuiltin reports whether a rule came from the baked-in default policy.
func (r Rule) isBuiltin() bool { return r.Source == "builtin" }

// ruleApplies reports whether a rule matches a given tool call: the tool name
// must match (case-insensitive, or "*" wildcard), and if the rule has a
// specifier, the tool's matcher must accept it.
func (e *Engine) ruleApplies(r Rule, toolName string, callInfo map[string]any) bool {
	if !toolNameMatches(r.Tool, toolName) {
		return false
	}
	if r.Specifier == "" {
		return true
	}
	m, ok := e.matchers[strings.ToLower(toolName)]
	if !ok {
		m, ok = e.matchers[toolName]
	}
	if !ok && strings.HasPrefix(strings.ToLower(toolName), "mcp__") {
		m, ok = e.matchers["mcp"]
	}
	if !ok {
		return false
	}
	if _, isBash := m.(BashMatcher); isBash {
		mode := MatchAllSubcommands
		if r.Effect == EffectDeny || r.Effect == EffectAsk {
			mode = MatchAnySubcommand
		}
		return BashRuleMatches(r.Specifier, callInfo, mode)
	}
	match, err := m.Match(r.Specifier, callInfo)
	if err != nil || !match {
		return false
	}
	return true
}

func toolNameMatches(ruleTool, callTool string) bool {
	if ruleTool == "*" {
		return true
	}
	rule := strings.ToLower(ruleTool)
	call := strings.ToLower(callTool)
	if rule == call {
		return true
	}
	if strings.HasSuffix(rule, "*") {
		return strings.HasPrefix(call, strings.TrimSuffix(rule, "*"))
	}
	return false
}

// ParseRule parses a "Tool(specifier)" string into a Rule with the given
// effect. "Tool" (no parens) parses to an empty specifier (match-all).
// Examples:
//
//	"Bash(npm run *)"   → {Tool:"bash", Specifier:"npm run *"}
//	"Read(./.env)"      → {Tool:"read", Specifier:"./.env"}
//	"Bash"              → {Tool:"bash", Specifier:""}  (match-all)
//	"mcp__github__*"    → {Tool:"mcp__github__*"}      (bare, match-all)
func ParseRule(s string, effect Effect, source string) (Rule, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Rule{}, fmt.Errorf("empty rule")
	}
	// Bare tool name (no parentheses) — match-all for that tool.
	// Reject anything that contains a stray ')' to keep the grammar tight.
	if !strings.Contains(s, "(") {
		if strings.ContainsAny(s, ")") || strings.ContainsAny(s, " \t") {
			return Rule{}, fmt.Errorf("malformed rule %q: unexpected character", s)
		}
		return Rule{Tool: s, Effect: effect, Source: source}, nil
	}
	open := strings.IndexByte(s, '(')
	close := strings.LastIndexByte(s, ')')
	if open <= 0 || close != len(s)-1 {
		return Rule{}, fmt.Errorf("malformed rule %q: expected Tool(specifier)", s)
	}
	tool := s[:open]
	spec := s[open+1 : close]
	if tool == "" {
		return Rule{}, fmt.Errorf("malformed rule %q: empty tool name", s)
	}
	spec = strings.TrimSpace(spec)
	return Rule{Tool: tool, Specifier: spec, Effect: effect, Source: source}, nil
}
