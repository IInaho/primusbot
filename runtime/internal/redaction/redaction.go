package redaction

import "strings"

const redactedSecret = "[redacted]"

func RedactInputText(input string) string {
	fields := strings.Fields(strings.TrimSpace(input))
	if len(fields) < 4 ||
		!strings.EqualFold(strings.TrimPrefix(fields[0], "/"), "connect") ||
		!strings.EqualFold(fields[1], "telegram") {
		return input
	}
	switch {
	case strings.EqualFold(fields[2], "token"):
		return strings.Join(append(fields[:3], redactedSecret), " ")
	case strings.EqualFold(fields[2], "add") && len(fields) == 4:
		return strings.Join(append(fields[:3], redactedSecret), " ")
	case strings.EqualFold(fields[2], "add") && len(fields) >= 5:
		return strings.Join(append(fields[:4], redactedSecret), " ")
	}
	return input
}
