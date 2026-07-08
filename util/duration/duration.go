// Package duration provides time-duration formatting helpers.
package duration

import "time"

// FormatDuration returns a human-readable duration string truncated to 0.1s.
// Returns "" for zero or negative durations, "0s" for sub-second.
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	if d < time.Second {
		return "0s"
	}
	return d.Truncate(100 * time.Millisecond).String()
}
