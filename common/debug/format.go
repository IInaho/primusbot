package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func callerFileLine(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "?:?"
	}
	return fmt.Sprintf("%s:%d", trimPath(file), line)
}

func trimPath(path string) string {
	path = filepath.ToSlash(path)
	if cwd, err := os.Getwd(); err == nil {
		cwd = filepath.ToSlash(cwd)
		if rel, err := filepath.Rel(cwd, path); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	if idx := strings.LastIndex(path, "/NekoCode/"); idx >= 0 {
		return path[idx+len("/NekoCode/"):]
	}
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return path
}
