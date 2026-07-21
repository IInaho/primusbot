package taskview

import (
	"fmt"
	"html"
	"strings"

	"nekocode/util/text"
)

const maxTelegramBody = 3600

// HTMLEscape escapes s for Telegram's HTML parse mode.
func HTMLEscape(s string) string {
	return html.EscapeString(s)
}

func htmlTitle(s string) string {
	return "<b>" + HTMLEscape(s) + "</b>"
}

func htmlCode(s string) string {
	return "<code>" + HTMLEscape(s) + "</code>"
}

func htmlPre(s string) string {
	return "<pre>" + HTMLEscape(s) + "</pre>"
}

func htmlBody(s string, max int) string {
	return HTMLEscape(truncateRunes(strings.TrimSpace(s), max))
}

func labelText(label, value string) string {
	if strings.TrimSpace(value) == "" {
		return HTMLEscape(label + ":")
	}
	return HTMLEscape(label + ": " + value)
}

func labelCode(label, value string) string {
	if strings.TrimSpace(value) == "" {
		return HTMLEscape(label + ":")
	}
	return HTMLEscape(label+": ") + htmlCode(value)
}

func compactMessage(parts ...string) string {
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			if len(lines) > 0 && lines[len(lines)-1] != "" {
				lines = append(lines, "")
			}
			continue
		}
		lines = append(lines, part)
	}
	out := strings.TrimSpace(strings.Join(lines, "\n"))
	if out == "" {
		return ""
	}
	return truncateRunes(out, maxTelegramBody)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	line := strings.SplitN(s, "\n", 2)[0]
	return text.TruncateByRune(line, 120)
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

func stringArg(args map[string]any, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	default:
		return fmt.Sprint(t), true
	}
}
