package runtime

import "strings"

const redactedSecret = "[redacted]"

// RedactInputText masks credentials in connector configuration commands before
// they are persisted or shown in local interaction history.
func RedactInputText(input string) string {
	fields := strings.Fields(strings.TrimSpace(input))
	if len(fields) < 4 || !strings.EqualFold(strings.TrimPrefix(fields[0], "/"), "connect") {
		return input
	}

	switch {
	case strings.EqualFold(fields[2], "token"):
		return strings.Join(append(fields[:3], redactedSecret), " ")
	case strings.EqualFold(fields[2], "add"):
		keep := 3
		if len(fields) >= 5 {
			keep = 4
		}
		return strings.Join(append(fields[:keep], redactedSecret), " ")
	default:
		return input
	}
}
