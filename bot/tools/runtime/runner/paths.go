package runner

import (
	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/workspace"
)

func toolPaths(tc core.ToolCallItem) []string {
	if p, ok := tc.Args["path"].(string); ok && p != "" {
		return []string{p}
	}
	return nil
}

func resolvePath(path string) (string, error) {
	return workspace.Resolve(path)
}
