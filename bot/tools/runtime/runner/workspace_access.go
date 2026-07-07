package runner

import (
	"fmt"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/bot/tools/runtime/workspace"
	"nekocode/common"
)

func (e *Executor) ensureWorkspaceAccess(tc core.ToolCallItem, confirmFn common.ConfirmFunc) (core.ToolCallItem, string, bool) {
	access, ok := fileToolAccess(tc.Name)
	if !ok {
		return tc, "", true
	}
	path := fileToolPath(tc)
	if path == "" {
		return tc, "", true
	}

	var safePath string
	var allowed bool
	var err error
	if access == workspace.AccessReadWrite {
		safePath, _, allowed, err = workspace.CheckWrite(path)
	} else {
		safePath, _, allowed, err = workspace.CheckRead(path)
	}
	if err != nil {
		return tc, err.Error(), false
	}
	if allowed {
		tc.Args = setToolPath(tc.Args, safePath)
		return tc, "", true
	}
	if confirmFn == nil {
		return tc, fmt.Sprintf("workspace access required for %s: %s", access, safePath), false
	}

	rootPath, err := workspace.CandidateRoot(path)
	if err != nil {
		return tc, err.Error(), false
	}
	req := common.NewConfirmRequest("workspace", map[string]any{
		"path":              rootPath,
		"access":            string(access),
		"requested_path":    safePath,
		"permission_reason": fmt.Sprintf("add %s workspace for %s", access, tc.Name),
	}, common.ConfirmKindPermission)
	reply := confirmFn(req)
	if !reply.Allowed {
		return tc, "cancelled", false
	}
	root, err := workspace.AddSessionRoot(rootPath, access)
	if err != nil {
		return tc, err.Error(), false
	}
	if reply.Remember {
		e.fnMu.RLock()
		store := e.permStore
		e.fnMu.RUnlock()
		if store != nil {
			_ = store.RememberWorkspaceRoot(permission.WorkspaceRoot{
				Path:   root.Path,
				Access: string(root.Access),
			})
		}
	}
	tc.Args = setToolPath(tc.Args, safePath)
	return tc, "", true
}

func fileToolAccess(toolName string) (workspace.Access, bool) {
	switch toolName {
	case "read", "list", "tree", "grep", "glob", "diff":
		return workspace.AccessReadOnly, true
	case "write", "edit":
		return workspace.AccessReadWrite, true
	default:
		return "", false
	}
}

func fileToolPath(tc core.ToolCallItem) string {
	if p, ok := tc.Args["path"].(string); ok && p != "" {
		return p
	}
	switch tc.Name {
	case "grep", "glob":
		return "."
	}
	return ""
}

func setToolPath(args map[string]any, path string) map[string]any {
	next := make(map[string]any, len(args)+1)
	for k, v := range args {
		next[k] = v
	}
	next["path"] = path
	return next
}
