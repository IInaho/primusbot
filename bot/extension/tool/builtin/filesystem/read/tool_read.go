package read

import (
	"context"
	"path/filepath"
	"strings"

	"nekocode/bot/extension/tool/runtime/core"
	"nekocode/bot/extension/tool/runtime/toolutil"
)

type ReadTool struct {
	toolutil.SafeReadOnlyTool
}

func (t *ReadTool) Name() string { return "read" }
func (t *ReadTool) Description() string {
	return "Read file contents (text, images, PDF). Absolute path required. Use startLine/endLine for range, max 500 lines. Text output includes [path#TAG], a VIEW line with rev/window metadata for JSON edit intents, then clean lineNo:content lines."
}

func (t *ReadTool) Parameters() []core.Parameter {
	return []core.Parameter{
		{Name: "path", Type: "string", Required: true, Description: "File path (absolute)"},
		{Name: "startLine", Type: "integer", Required: true, Description: "First line to read (1-based)"},
		{Name: "endLine", Type: "integer", Required: true, Description: "Last line to read (inclusive, >= startLine)"},
	}
}

const maxReadLines = 500

func (t *ReadTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	path, err := toolutil.RequireStringArg(args, "path")
	if err != nil {
		return "", err
	}
	safePath, err := toolutil.ValidatePathReadableContext(ctx, path)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(filepath.Ext(safePath)) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return t.readImage(safePath)
	case ".pdf":
		return t.readPDF(safePath)
	default:
		return t.readTextCached(ctx, safePath, args)
	}
}
