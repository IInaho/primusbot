// Package reasoning defines provider-neutral reasoning controls.
package reasoning

// Settings is one coherent snapshot of model reasoning controls.
// Disabled is derived from either an explicit runtime override (compaction,
// subagents) or the portable "none" effort.
type Settings struct {
	Requested      string
	Effort         string
	Disabled       bool
	DisableEffort  string
	ThinkingToggle bool
}

func (s Settings) RequestedValue() string {
	if s.Requested == "" {
		return "auto"
	}
	return s.Requested
}

func (s Settings) EffectiveValue() string {
	if s.Disabled {
		if s.DisableEffort != "" || s.ThinkingToggle {
			return "none"
		}
		return "auto"
	}
	if s.Effort == "" {
		return "auto"
	}
	return s.Effort
}
