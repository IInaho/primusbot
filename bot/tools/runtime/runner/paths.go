package runner

import (
	"fmt"
	"path/filepath"

	"nekocode/bot/tools/runtime/core"
)

func confirmArgs(_ string, args map[string]any) map[string]any {
	return args
}

func toolPaths(tc core.ToolCallItem) []string {
	if p, ok := tc.Args["path"].(string); ok && p != "" {
		return []string{p}
	}
	return nil
}

func validatePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("path resolution failed: %w", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		parent := filepath.Dir(abs)
		realParent, pErr := filepath.EvalSymlinks(parent)
		if pErr != nil {
			return abs, nil
		}
		return filepath.Join(realParent, filepath.Base(abs)), nil
	}
	return real, nil
}
