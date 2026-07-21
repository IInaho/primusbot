package redaction

import "strings"

const redactedSecret = "[redacted]"

// RedactInputText masks sensitive tokens in well-known command patterns.
// Supported patterns:
//   - /connect <provider> token <secret>
//   - /connect <provider> add <secret>
//   - /connect <provider> add <name> <secret>
func RedactInputText(input string) string {
	fields := strings.Fields(strings.TrimSpace(input))
	if len(fields) < 4 {
		return input
	}
	if !strings.EqualFold(strings.TrimPrefix(fields[0], "/"), "connect") {
		return input
	}

	switch {
	case strings.EqualFold(fields[2], "token"):
		// /connect <provider> token <secret>
		return strings.Join(append(fields[:3], redactedSecret), " ")
	case strings.EqualFold(fields[2], "add"):
		// /connect <provider> add <secret>
		// /connect <provider> add <name> <secret>
		keep := 3
		if len(fields) >= 5 {
			keep = 4
		}
		return strings.Join(append(fields[:keep], redactedSecret), " ")
	}
	return input
}
