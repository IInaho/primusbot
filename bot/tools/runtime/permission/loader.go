package permission

import (
	"fmt"
	"path/filepath"
)

// RuleSources aggregates the three rule origins for a workspace:
//
//   - builtin: the default policy (sudo deny, rm ask, ls allow, ...)
//   - declared: user/project rules from config.json "permissions" block
//   - remembered: rules the user approved at an "ask" prompt (persisted)
//
// Evaluation precedence is deny → ask → allow across ALL sources (a deny
// from any source wins; a declared allow can broaden a builtin ask).
type RuleSources struct {
	Declared   PermissionsDecl // from config.json
	Workspace  string
	Remembered []Rule // loaded from the store
}

// PermissionsDecl is the declarative rule config (mirrors config.PermissionsConfig
// without the import cycle — the caller extracts the string slices).
type PermissionsDecl struct {
	Allow []string
	Ask   []string
	Deny  []string
}

// LoadRules builds the full ordered rule list for the engine. Order within the
// slice does not affect precedence (the engine applies deny→ask→allow), but we
// group by source for readability of diagnostics.
func LoadRules(src RuleSources) ([]Rule, error) {
	var rules []Rule

	// 1. builtin defaults
	rules = append(rules, BuiltinRules()...)

	// 2. declared rules from config (user/project)
	for _, s := range src.Declared.Deny {
		r, err := ParseRule(s, EffectDeny, "declared")
		if err != nil {
			return nil, fmt.Errorf("permissions.deny %q: %w", s, err)
		}
		rules = append(rules, r)
	}
	for _, s := range src.Declared.Ask {
		r, err := ParseRule(s, EffectAsk, "declared")
		if err != nil {
			return nil, fmt.Errorf("permissions.ask %q: %w", s, err)
		}
		rules = append(rules, r)
	}
	for _, s := range src.Declared.Allow {
		r, err := ParseRule(s, EffectAllow, "declared")
		if err != nil {
			return nil, fmt.Errorf("permissions.allow %q: %w", s, err)
		}
		rules = append(rules, r)
	}

	// 3. remembered rules (user approved at an ask prompt)
	for _, r := range src.Remembered {
		if r.Source == "" {
			r.Source = "remembered"
		}
		rules = append(rules, r)
	}

	return rules, nil
}

// NewEngineForWorkspace builds a ready engine: builtin + declared + remembered
// rules, with the standard matchers registered.
func NewEngineForWorkspace(decl PermissionsDecl, store *Store, workspace string) (*Engine, error) {
	workspace = filepath.Clean(workspace)
	var remembered []Rule
	if store != nil && workspace != "" {
		remembered = store.RememberedRules(workspace)
	}
	rules, err := LoadRules(RuleSources{
		Declared:   decl,
		Workspace:  workspace,
		Remembered: remembered,
	})
	if err != nil {
		return nil, err
	}
	e := NewEngine(DefaultMatchers())
	e.SetRules(rules)
	return e, nil
}

// CallInfoForBash builds the callInfo map for a bash command.
func CallInfoForBash(command string) map[string]any {
	return map[string]any{"command": command}
}

// CallInfoForPath builds the callInfo map for a file-path tool (read/edit/
// write/glob/grep/list/tree).
func CallInfoForPath(path, workspace, home string) map[string]any {
	return map[string]any{"path": path, "workspace": workspace, "home": home}
}

// CallInfoForDomain builds the callInfo map for web_fetch.
func CallInfoForDomain(domain string) map[string]any {
	return map[string]any{"domain": domain}
}
