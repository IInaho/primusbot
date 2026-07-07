package toolutil

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"time"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/editcore"
	"nekocode/bot/tools/runtime/execution"
	"nekocode/bot/tools/runtime/workspace"
	"nekocode/common"
)

var ansiRegex = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

func StripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func ValidatePath(path string) (string, error) {
	return workspace.Resolve(path)
}

func ValidatePathReadable(path string) (string, error) {
	safePath, _, ok, err := workspace.CheckRead(path)
	if err != nil {
		return "", err
	}
	if ok {
		return safePath, nil
	}
	return "", fmt.Errorf("path %s is outside readable workspaces (roots: %v)", safePath, workspace.Snapshot())
}

func ValidatePathWritable(path string) (string, error) {
	safePath, _, ok, err := workspace.CheckWrite(path)
	if err != nil {
		return "", err
	}
	if ok {
		return safePath, nil
	}
	return "", fmt.Errorf("path %s is outside writable workspaces (roots: %v)", safePath, workspace.Snapshot())
}

func NormalizeText(text string) string {
	text = StripAnsi(text)
	return editcore.NormalizeToLF(text)
}

func ReadSafeFile(path string) ([]byte, error) {
	safePath, err := ValidatePathReadable(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(safePath)
}

func NewToolHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: common.SharedTransport,
		Timeout:   timeout,
	}
}

func IsAllExploratory(calls []core.ToolCallItem) bool {
	if len(calls) == 0 {
		return false
	}
	for _, c := range calls {
		switch c.Name {
		case "read", "grep", "glob", "list", "web_search", "web_fetch":
			continue
		default:
			return false
		}
	}
	return true
}

func RecordSnapshot(path, content string) string {
	return recordSnapshot(execution.GetGlobalSnapshotStore(), path, content)
}

func RecordSnapshotInContext(ctx context.Context, path, content string) string {
	return recordSnapshot(execution.SnapshotStoreFromContext(ctx), path, content)
}

func recordSnapshot(store *editcore.SnapshotStore, path, content string) string {
	if store == nil {
		return ""
	}
	return store.Record(path, content)
}
