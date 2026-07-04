package shell

import (
	"context"
	"strconv"
	"strings"
	"time"

	"nekocode/bot/tools/builtin/toolhelpers"
	"nekocode/bot/tools/runtime/core"
)

const defaultBashTimeout = 120 * time.Second

type BashTool struct{}

func (t *BashTool) Name() string                                    { return "bash" }
func (t *BashTool) ExecutionMode(map[string]any) core.ExecutionMode { return core.ModeSequential }

func (t *BashTool) Description() string {
	return "Execute shell commands in an isolated sandbox. " +
		strconv.Itoa(int(defaultBashTimeout.Seconds())) + "s timeout by default, configurable via timeout_ms parameter (max 600s). " +
		"Shell state NOT preserved between calls (use && to chain, absolute paths instead of cd). " +
		"Prefer dedicated tools (Read, Edit, Write, Grep, Glob) — bash is a last resort when no dedicated tool fits. " +
		"Confirm OS compatibility before running distro-specific commands. " +
		"Exploratory bash (ls, cat, grep, find, git diff/log/status) consumes read quota and may be blocked when budget exhausted. " +
		"Never git push --force or skip hooks. " +
		"\n\nSandbox isolation: by default the command runs with NO outbound network and only the workspace directory is writable; " +
		"system directories are read-only and /tmp is isolated. If the command needs more, declare it via the `capabilities` parameter: " +
		"\"net.outbound\" (curl, git clone, npm install, etc.), " +
		"\"fs.write.cache\" (package manager caches like ~/.npm, ~/go/pkg/mod, ~/.cargo), " +
		"\"fs.write.path\" (workspace-external writable directories, use with `write_paths`), " +
		"\"process.host\" (completely unsandboxed host execution — use ONLY when the sandbox cannot satisfy the command, e.g. needs TTY or Docker socket; prompts every time). " +
		"If a command fails inside the sandbox, read the error (e.g. 'Network is unreachable' → declare net.outbound; 'Read-only file system' → declare fs.write.path with write_paths) and retry with the appropriate capabilities."
}

func (t *BashTool) Parameters() []core.Parameter {
	return []core.Parameter{
		{Name: "command", Type: "string", Required: true, Description: "The command to execute"},
		{Name: "capabilities", Type: "array", Required: false,
			Description: "Capabilities to authorize for this command. Each opens a specific sandbox boundary: " +
				"net.outbound (outbound network), fs.write.cache (package manager caches), " +
				"fs.write.path (extra writable dirs, use with write_paths), process.host (unsandboxed host execution). " +
				"Omit for default sandbox (no network, workspace-only writes)."},
		{Name: "write_paths", Type: "array", Required: false,
			Description: "Extra writable directories outside the workspace (absolute paths or ~/-prefixed). " +
				"Only effective when capabilities includes fs.write.path."},
		{Name: "timeout_ms", Type: "number", Required: false, Description: "Timeout in milliseconds (default 120000, max 600000)"},
	}
}

func (t *BashTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	cmdStr, err := toolhelpers.RequireStringArg(args, "command")
	if err != nil {
		return "", err
	}
	caps := optStringSliceArg(args, "capabilities")
	writePaths := optStringSliceArg(args, "write_paths")
	return runCommand(ctx, strings.TrimSpace(cmdStr), caps, writePaths, bashTimeout(args))
}

func (t *BashTool) ExecuteWithPermission(ctx context.Context, args map[string]any, grant core.PermissionRequest) (string, error) {
	cmdStr, err := toolhelpers.RequireStringArg(args, "command")
	if err != nil {
		return "", err
	}
	writePaths := optStringSliceArg(args, "write_paths")
	return runCommandWithPermission(ctx, strings.TrimSpace(cmdStr), writePaths, bashTimeout(args), grant)
}

func bashTimeout(args map[string]any) time.Duration {
	timeout := defaultBashTimeout
	if timeoutMs, ok := args["timeout_ms"].(float64); ok && timeoutMs > 0 {
		timeout = time.Duration(timeoutMs) * time.Millisecond
		if timeout > 600*time.Second {
			timeout = 600 * time.Second
		}
	}
	return timeout
}

// optStringSliceArg extracts an optional []string from args[key]. Accepts
// []any, []string, or nil/absent (returns nil).
func optStringSliceArg(args map[string]any, key string) []string {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
