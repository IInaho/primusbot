// Package yaml provides YAML frontmatter parsing.
package yaml

import (
	"fmt"
	"strings"
)

// ParseYAMLFrontmatter extracts YAML frontmatter between --- delimiters.
// Returns the YAML bytes and the body text after the closing ---.
func ParseYAMLFrontmatter(content string) (yamlBytes []byte, body string, err error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return nil, "", fmt.Errorf("missing frontmatter (---)")
	}
	rest := content[3:]
	yamlText, body, found := strings.Cut(rest, "\n---")
	if !found {
		return nil, "", fmt.Errorf("unclosed frontmatter")
	}
	return []byte(yamlText), strings.TrimSpace(body), nil
}
