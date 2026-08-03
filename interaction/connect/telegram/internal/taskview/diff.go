package taskview

import "strings"

// cleanDiffPreview strips internal headers and the structured trailer from
// a tool diff preview, leaving the plain diff stored on the run card.
func cleanDiffPreview(preview string) string {
	lines := strings.Split(strings.TrimRight(preview, "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "EDIT_PREVIEW_JSON_B64 ") {
			if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "---" {
				out = out[:len(out)-1]
			}
			break
		}
		if isInternalDiffHeader(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func isInternalDiffHeader(line string) bool {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return false
	}
	return strings.Contains(line, "#") || strings.HasPrefix(line, "[write ")
}
