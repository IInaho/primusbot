package permission

import (
	"path/filepath"
	"time"
)

// RememberRule persists a rule (typically an allow rule created when the user
// approved an "ask" prompt) for the given workspace. Duplicates are skipped.
func (s *Store) RememberRule(workspace string, rule Rule) error {
	workspace = filepath.Clean(workspace)
	if workspace == "" || rule.Tool == "" {
		return nil
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
		if existing.Tool == rule.Tool &&
			existing.Specifier == rule.Specifier &&
			existing.Effect == rule.Effect {
			return nil // already remembered
		}
	}
	rule.CreatedAt = time.Now().UTC()
	p.Rules = append(p.Rules, rule)
	f.Projects[workspace] = p
	return s.save(f)
}

// RememberedRules returns the rules remembered for the given workspace.
func (s *Store) RememberedRules(workspace string) []Rule {
	workspace = filepath.Clean(workspace)
	if workspace == "" {
		return nil
	}
	f, err := s.load()
	if err != nil {
		return nil
	}
	p, ok := f.Projects[workspace]
	if !ok {
		return nil
	}
	return append([]Rule(nil), p.Rules...)
}
