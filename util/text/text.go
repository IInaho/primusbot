// Package text provides string formatting and parsing utilities.
package text

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// NormalizeTerminalOutput converts terminal-oriented output into stable plain text.
func NormalizeTerminalOutput(value string) string {
	value = ansi.Strip(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\n\r", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

// TruncateByRune truncates s to max runes.
func TruncateByRune(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// SplitPairs splits on commas that are not inside double-quoted segments.
func SplitPairs(s string) []string {
	var pairs []string
	start := 0
	inQuote := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '\\':
			if inQuote && i+1 < len(s) {
				i++ // skip escaped char
			}
		case ',':
			if !inQuote {
				pairs = append(pairs, s[start:i])
				start = i + 1
			}
		}
	}
	pairs = append(pairs, s[start:])
	return pairs
}

// FormatTokens formats a token count for display (e.g. 1200 → "1.2k", 1500000 → "1.5m").
func FormatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fm", float64(n)/1_000_000)
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// LooksLikeGit returns true if s looks like a "user/repo" git reference.
func LooksLikeGit(s string) bool {
	parts := strings.Split(s, "/")
	return len(parts) == 2 && !strings.Contains(parts[0], ".") && !strings.Contains(parts[0], ":")
}
