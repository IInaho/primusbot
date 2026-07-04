package permission

import (
	"fmt"
	"strings"
	"time"

	"mvdan.cc/sh/v3/syntax"
)

// RememberRule persists a rule (typically an allow rule created when the user
// approved an "ask" prompt) into the project's permissions file. The
// workspace argument is kept for signature compatibility but ignored — the
// store is already bound to a project root.
func (s *Store) RememberRule(_ string, rule Rule) error {
	if s.root == "" || rule.Tool == "" {
		return nil
	}
	return s.rememberRule(s.root, rule)
}

func (s *Store) rememberRule(workspace string, rule Rule) error {
	var err error
	rule, err = canonicalRememberedRule(rule)
	if err != nil {
		return err
	}
	rule.Source = "remembered"
	f, err := s.load()
	if err != nil {
		return err
	}
	if f.Version == 0 {
		f.Version = permissionStoreVersion
	}
	if f.Projects == nil {
		f.Projects = map[string]permissionProject{}
	}
	p := f.Projects[workspace]
	for _, existing := range p.Rules {
		existing, err := canonicalRememberedRule(existing)
		if err != nil {
			continue
		}
		if existing.Tool == rule.Tool && existing.Specifier == rule.Specifier && existing.Effect == rule.Effect {
			return nil // already remembered
		}
	}
	rule.CreatedAt = time.Now().UTC()
	p.Rules = append(p.Rules, rule)
	f.Projects[workspace] = p
	return s.save(f)
}

func canonicalRememberedRule(rule Rule) (Rule, error) {
	rule.Tool = strings.ToLower(strings.TrimSpace(rule.Tool))
	rule.Specifier = strings.TrimSpace(rule.Specifier)
	if rule.Tool == "" {
		return Rule{}, fmt.Errorf("remembered rule requires a tool")
	}
	if rule.Specifier == "" {
		return Rule{}, fmt.Errorf("remembered rule for %s requires a specifier", rule.Tool)
	}
	if rule.Effect != EffectAllow {
		return Rule{}, fmt.Errorf("remembered rule for %s must be allow", rule.Tool)
	}
	if rule.Tool == "bash" || rule.Tool == "shell" {
		if err := validateRememberedBashSpecifier(rule.Specifier); err != nil {
			return Rule{}, err
		}
	}
	return rule, nil
}

func validateRememberedBashSpecifier(spec string) error {
	if strings.HasSuffix(spec, ":*") {
		return fmt.Errorf("remembered bash rule must be exact, got %q", spec)
	}
	if strings.HasSuffix(spec, " *") && !isCommandWildcardSpecifier(spec) {
		return fmt.Errorf("remembered bash wildcard must be command-scoped, got %q", spec)
	}
	if _, err := syntax.NewParser().Parse(strings.NewReader(spec), ""); err != nil {
		return fmt.Errorf("remembered bash rule must be parseable: %w", err)
	}
	return nil
}

func isCommandWildcardSpecifier(spec string) bool {
	fields := strings.Fields(spec)
	if len(fields) != 2 || fields[1] != "*" {
		return false
	}
	name := fields[0]
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' || r == '/' {
			continue
		}
		return false
	}
	return true
}

// RememberedRules returns the rules persisted in the project's permissions
// file.
func (s *Store) RememberedRules(_ string) []Rule {
	if s.root == "" {
		return nil
	}
	f, err := s.load()
	if err != nil {
		return nil
	}
	p, ok := f.Projects[s.root]
	if !ok {
		return nil
	}
	return append([]Rule(nil), p.Rules...)
}
