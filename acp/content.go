package acp

import (
	"fmt"
	"strings"
)

func promptText(blocks []contentBlock) (string, *wireError) {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case "text":
			parts = append(parts, block.Text)
		case "resource_link":
			if block.Name == "" || block.URI == "" {
				return "", rpcError(-32602, "resource_link requires name and uri")
			}
			link := fmt.Sprintf("[%s](%s)", block.Name, block.URI)
			if block.Description != "" {
				link += " — " + block.Description
			}
			parts = append(parts, link)
		default:
			return "", rpcError(-32602, "unsupported prompt content type %q", block.Type)
		}
	}
	text := strings.TrimSpace(strings.Join(parts, "\n\n"))
	if text == "" {
		return "", rpcError(-32602, "prompt is empty")
	}
	return text, nil
}
