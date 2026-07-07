package shell

import (
	"context"
	"strconv"
	"strings"
	"time"

	"nekocode/bot/tools/builtin/toolhelpers"
	"nekocode/bot/tools/runtime/core"
)

// defaultBashTimeout is the default timeout for a bash call. Keep it in
// sync with the timeout_ms parameter description below (also 120000ms) —
// 10s was too short for legitimate one-shot commands (medium npm builds,
// curl of a slow mirror, `go test` on a large pkg) and inconsistent with
// the documented default. Long-running foreground processes (dev servers,
// watch tasks, REPLs) MUST NOT be run via this tool — there is no background
// mode; the call blocks until completion or the timeout kills it.
const defaultBashTimeout = 120 * time.Second

type BashTool struct{}

func (t *BashTool) Name() string                                    { return "bash" }
func (t *BashTool) ExecutionMode(map[string]any) core.ExecutionMode { return core.ModeSequential }

func (t *BashTool) Description() string {
	return "Execute shell commands in an isolated sandbox. " +
		strconv.Itoa(int(defaultBashTimeout.Seconds())) + "s timeout by default, configurable via timeout_ms parameter (max 600s). " +
		"Shell process state is NOT preserved between calls: cwd changes from cd, shell variables, exported env vars, aliases, and functions are lost. " +
		"Workspace filesystem changes ARE persistent across calls because the workspace is bind-mounted read-write; /tmp is isolated per call and not persistent. " +
		"Use && to chain dependent shell steps, and use absolute paths instead of relying on prior cd. " +
		"Prefer dedicated tools (Read, Edit, Write, Grep, Glob) — bash is a last resort when no dedicated tool fits. " +
		"Confirm OS compatibility before running distro-specific commands. " +
		"Exploratory bash (ls, cat, grep, find, git diff/log/status) consumes read quota and may be blocked when budget exhausted. " +
		"Never git push --force or skip hooks. " +
		"\n\nDo NOT use bash to run long-running foreground processes. " +
		"Commands that keep running until killed (dev servers like `npm run dev`/`vite`/`webpack serve`/`next dev`, watch tasks like `tsc -w`/`npm run watch`, REPLs like `node`/`python`/`irb`, `tail -f`, `npm start --host`, etc.) will block this tool until the timeout kills them, then return a truncated 'command timed out' error — the server is NOT kept alive in the background. " +
		"For such commands use the bg tool instead (e.g. `bg(action=\"start\", command=\"npm run dev\")`), or tell the user to run it in their own terminal. " +
		"You CAN run one-shot variants (build, test, lint, generate) that exit on their own — those are fine. " +
		"\n\nSandbox isolation: by default the command runs in sandbox_mode=\"workspace-write\" with NO outbound network and only the workspace directory writable; " +
		"system directories are read-only and /tmp is isolated. Use sandbox_mode=\"read-only\" for commands that should not write the workspace. " +
		"Extra access is explicit: network=true requests outbound network, writable_roots requests specific workspace-external writable directories, and sandbox_mode=\"host\" requests completely unsandboxed host execution. " +
		"These requests go through the permission UI; the runtime never infers them from command names or output text and never automatically retries with broader access. " +
		"If a 'command timed out' error occurs, the command was a long-running process; do not retry it with a longer timeout — instead tell the user to run it in their terminal."
}

func (t *BashTool) Parameters() []core.Parameter {
	return []core.Parameter{
		{Name: "command", Type: "string", Required: true, Description: "The command to execute"},
		{Name: "sandbox_mode", Type: "string", Required: false,
			Description: "Sandbox mode: read-only, workspace-write (default), or host. host is unsandboxed and prompts every time."},
		{Name: "network", Type: "boolean", Required: false,
			Description: "Request outbound network access. This is explicit and requires permission; it is never inferred from command output."},
		{Name: "writable_roots", Type: "array", Required: false,
			Description: "Extra writable directories outside the workspace (absolute paths or ~/-prefixed). Requires permission."},
		{Name: "timeout_ms", Type: "number", Required: false, Description: "Timeout in milliseconds (default 120000, max 600000). Do not raise this to keep dev servers / watch tasks alive — those are long-running foreground processes that should be run by the user in their terminal, not via this tool."},
	}
}

func (t *BashTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	cmdStr, err := toolhelpers.RequireStringArg(args, "command")
	if err != nil {
		return "", err
	}
	return runCommand(ctx, strings.TrimSpace(cmdStr), sandboxRequestFromArgs(args), bashTimeout(args))
}

func (t *BashTool) ExecuteWithPermission(ctx context.Context, args map[string]any, grant core.PermissionRequest) (string, error) {
	cmdStr, err := toolhelpers.RequireStringArg(args, "command")
	if err != nil {
		return "", err
	}
	return runCommandWithPermission(ctx, strings.TrimSpace(cmdStr), sandboxRequestFromArgs(args), bashTimeout(args), grant)
}

func sandboxRequestFromArgs(args map[string]any) sandboxRequest {
	writableRoots := optStringSliceArg(args, "writable_roots")
	req := sandboxRequest{
		Mode:          optStringArg(args, "sandbox_mode"),
		Network:       optBoolArg(args, "network"),
		WritableRoots: writableRoots,
	}
	return req
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

func optStringArg(args map[string]any, key string) string {
	raw, ok := args[key]
	if !ok || raw == nil {
		return ""
	}
	s, _ := raw.(string)
	return strings.TrimSpace(s)
}

func optBoolArg(args map[string]any, key string) bool {
	raw, ok := args[key]
	if !ok || raw == nil {
		return false
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	}
	return false
}
