package taskview

import "strings"

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

func diffPath(preview string) string {
	for _, line := range strings.Split(preview, "\n") {
		switch {
		case isInternalDiffHeader(line):
			header := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			if hash := strings.LastIndexByte(header, '#'); hash > 0 {
				return header[:hash]
			}
			if strings.HasPrefix(header, "write ") {
				return strings.TrimSpace(strings.TrimPrefix(header, "write "))
			}
		case strings.HasPrefix(line, "--- "):
			path := strings.TrimSpace(strings.TrimPrefix(line, "--- "))
			path = strings.TrimPrefix(path, "a/")
			if path != "" && path != "/dev/null" {
				return path
			}
		case strings.HasPrefix(line, "+++ "):
			path := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			path = strings.TrimPrefix(path, "b/")
			if path != "" && path != "/dev/null" {
				return path
			}
		}
	}
	return ""
}

func diffLineCounts(preview string) (add, del int) {
	for _, line := range strings.Split(preview, "\n") {
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if strings.HasPrefix(line, "+") {
			add++
			continue
		}
		if strings.HasPrefix(line, "-") {
			del++
			continue
		}
		if colon := strings.IndexByte(line, ':'); colon > 0 {
			prefix := strings.TrimSpace(line[:colon])
			if strings.HasPrefix(prefix, "+") {
				add++
			} else if strings.HasPrefix(prefix, "-") {
				del++
			}
		}
	}
	return add, del
}
