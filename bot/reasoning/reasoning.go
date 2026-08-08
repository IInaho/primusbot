// Package reasoning defines provider-neutral reasoning controls.
package reasoning

// ReplayPolicy describes which provider-issued reasoning must be sent back in
// later requests. Reasoning remains available in the local session regardless
// of this policy.
type ReplayPolicy uint8

const (
	ReplayNone ReplayPolicy = iota
	ReplayToolCalls
	ReplaySigned
)

// Settings is one coherent snapshot of model reasoning controls.
// Disabled is derived from either an explicit runtime override (compaction,
// subagents) or the portable "none" effort.
type Settings struct {
	Requested     string
	Effort        string
	Disabled      bool
	DisableEffort string
	ThinkingMode  string
	Replay        ReplayPolicy
}

func (s Settings) RequestedValue() string {
	if s.Requested == "" {
		return "auto"
	}
	return s.Requested
}

func (s Settings) EffectiveValue() string {
	if s.Disabled {
		if s.DisableEffort != "" || s.ThinkingMode != "" {
			return "none"
		}
		return "auto"
	}
	if s.Effort == "" {
		return "auto"
	}
	return s.Effort
}
