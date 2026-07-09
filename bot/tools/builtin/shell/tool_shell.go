package shell

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"nekocode/bot/tools/runtime/toolhelpers"
	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/bot/tools/runtime/sandbox"
)

const (
	defaultShellYield   = 10 * time.Second
	defaultShellTimeout = 120 * time.Second
	maxShellTimeout     = 600 * time.Second
)

// SandboxProfiler recommends a sandbox profile for a tool call. The permission
// engine (permission.Engine) satisfies this interface via its SandboxFor method.
type SandboxProfiler interface {
	SandboxFor(toolName string, callInfo map[string]any) (permission.SandboxProfile, bool)
}

type ShellTool struct {
	mu       sync.Once
	registry *TaskRegistry

	permProfiler SandboxProfiler
}

func (t *ShellTool) registryOnce() *TaskRegistry {
	t.mu.Do(func() {
		t.registry = NewTaskRegistry()
	})
	return t.registry
}

// SetSandboxProfiler injects the permission engine used to look up sandbox
// rules. Without it, builtin sandbox profiles (e.g. pnpm dev → network) are
// never applied.
func (t *ShellTool) SetSandboxProfiler(p SandboxProfiler) {
	t.permProfiler = p
}

func (t *ShellTool) Shutdown() []error {
	return t.registryOnce().StopAll()
}

func (t *ShellTool) Name() string { return "shell" }

func (t *ShellTool) ExecutionMode(args map[string]any) core.ExecutionMode {
	action := shellAction(args)
	if action == "run" || action == "stop" {
		return core.ModeSequential
	}
	return core.ModeParallel
}

func (t *ShellTool) Description() string {
	return "Execute shell commands in a session-based sandbox. " +
		"Default action is run. A run waits up to yield_time_ms (default 10000ms); if the command is still running, it returns session_id and the process continues until timeout_ms (default 120000ms, max 600000ms). " +
		"Use action=\"wait\" to wait for a session, action=\"poll\" to read recent output without waiting, action=\"stop\" to terminate a session, and action=\"list\" to list sessions. " +
		"Sandbox defaults to workspace-write with network disabled (loopback only). Set network=true for any command that uses the network stack — outbound (curl, npm install, git clone) OR inbound (listening on a port, dev servers). Builtin rules auto-enable network for known commands (pnpm dev, npm install, etc.); override only when a command fails without it. " +
		"Each shell run creates an independent sandbox instance. Only the workspace, writable_roots directories (bind-mounted from host), and system dirs (/usr, /bin, /lib, /etc, /nix/store, read-only) are visible. Other paths (/home subdirs, ~/.local, $GOPATH, etc.) are not accessible. Use host mode to bypass sandbox isolation."
}

func (t *ShellTool) Parameters() []core.Parameter {
	return []core.Parameter{
		{Name: "action", Type: "string", Required: false, Description: "Action: run (default), wait, poll, stop, or list."},
		{Name: "command", Type: "string", Required: false, Description: "Shell command. Required for action=run."},
		{Name: "session_id", Type: "number", Required: false, Description: "Session id returned by action=run. Required for wait, poll, and stop."},
		{Name: "yield_time_ms", Type: "number", Required: false, Description: "How long this tool call waits before returning a running session (default 10000)."},
		{Name: "timeout_ms", Type: "number", Required: false, Description: "Total command lifetime before runtime kills it (default 120000, max 600000)."},
		{Name: "sandbox_mode", Type: "string", Required: false, Description: "Sandbox mode for run: read-only, workspace-write (default), or host."},
		{Name: "network", Type: "boolean", Required: false, Description: "Default false (loopback only). Builtin rules auto-enable for known commands. Set true for any command using the network stack — outbound (curl, npm install) or inbound (dev servers listening on a port)."},
		{Name: "writable_roots", Type: "array", Required: false, Description: "Extra writable directories outside the workspace for run. Requires permission."},
	}
}

func (t *ShellTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	return t.run(ctx, args, core.PermissionRequest{})
}

func (t *ShellTool) ExecuteWithPermission(ctx context.Context, args map[string]any, grant core.PermissionRequest) (string, error) {
	return t.run(ctx, args, grant)
}

func (t *ShellTool) run(ctx context.Context, args map[string]any, grant core.PermissionRequest) (string, error) {
	registry := t.registryOnce()
	switch shellAction(args) {
	case "run":
		return t.start(ctx, args, registry, grant)
	case "wait":
		id, err := shellSessionID(args)
		if err != nil {
			return "", err
		}
		logs, running, err := registry.Wait(id, shellYield(args))
		if err != nil {
			return "", err
		}
		return formatShellSession(registry, id, logs, running), nil
	case "poll":
		id, err := shellSessionID(args)
		if err != nil {
			return "", err
		}
		logs, running, err := registry.Logs(id)
		if err != nil {
			return "", err
		}
		return formatShellSession(registry, id, logs, running), nil
	case "stop":
		id, err := shellSessionID(args)
		if err != nil {
			return "", err
		}
		return registry.Stop(id)
	case "list":
		return formatShellList(registry.List()), nil
	default:
		return "", fmt.Errorf("unknown action %q: must be run, wait, poll, stop, or list", shellAction(args))
	}
}

func (t *ShellTool) start(ctx context.Context, args map[string]any, registry *TaskRegistry, grant core.PermissionRequest) (string, error) {
	command, err := toolhelpers.RequireStringArg(args, "command")
	if err != nil {
		return "", err
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("command is required when action is \"run\"")
	}
	workspace, _ := os.Getwd()
	reqProfile := sandboxRequestFromArgs(args)
	requestedCaps := reqProfile.permissionCapabilities()
	if hasCapability(requestedCaps, core.CapProcessHost) && !hasCapability(grant.Capabilities, core.CapProcessHost) {
		return "", permissionRequired(
			"shell command requests unsandboxed host execution",
			[]string{core.CapProcessHost},
			"once",
			workspace,
			reqProfile.WritableRoots,
		)
	}
	if len(requestedCaps) > 0 && !containsAllCapabilities(grant.Capabilities, requestedCaps) {
		return "", permissionRequired(
			fmt.Sprintf("shell command requests sandbox profile: %s", strings.Join(requestedCaps, ", ")),
			requestedCaps,
			"project",
			workspace,
			reqProfile.WritableRoots,
		)
	}
	host := hasCapability(grant.Capabilities, core.CapProcessHost)
	var profile sandbox.Profile
	if !host {
		profile, err = buildProfileFromRequest(workspace, reqProfile, grant.Capabilities)
		if err != nil {
			return "", err
		}
		// Apply builtin/user sandbox rules as defaults when the caller did
		// not explicitly set a value. If the LLM passed network=..., that
		// choice wins.
		if ruleProf, ok := t.sandboxProfileFor(args, grant.Capabilities); ok {
			if ruleProf.Network && !reqProfile.Network &&
				hasCapability(grant.Capabilities, core.CapNetOutbound) {
				profile.Network = true
			}
		}
	}
	task, initial, err := registry.Start(ctx, StartRequest{
		Command:    command,
		Profile:    profile,
		Host:       host,
		Timeout:    shellTimeout(args),
		SampleWait: shellYield(args),
	})
	if err != nil {
		var unavailable sandbox.UnavailableError
		if errors.As(err, &unavailable) {
			return "", hostPermission(err.Error(), workspace, nil)
		}
		return "", fmt.Errorf("failed to start shell command: %w", err)
	}
	running := task.summary().Status == taskRunning.String()
	return formatShellSession(registry, task.id, string(initial), running), nil
}

// sandboxProfileFor consults the injected permission engine for a sandbox rule
// matching this command. The returned profile is the rule's recommendation; the
// caller is responsible for filtering it through its authorized capabilities.
func (t *ShellTool) sandboxProfileFor(args map[string]any, authorizedCaps []string) (permission.SandboxProfile, bool) {
	if t.permProfiler == nil {
		return permission.SandboxProfile{}, false
	}
	cmd, _ := args["command"].(string)
	callInfo := permission.BuildCallInfo("shell", map[string]any{"command": cmd}, "", "")
	return t.permProfiler.SandboxFor("shell", callInfo)
}

func shellAction(args map[string]any) string {
	action := strings.ToLower(strings.TrimSpace(optStringArg(args, "action")))
	if action == "" {
		return "run"
	}
	if action == "logs" {
		return "poll"
	}
	return action
}

func shellYield(args map[string]any) time.Duration {
	return durationArg(args, "yield_time_ms", defaultShellYield, maxShellTimeout)
}

func shellTimeout(args map[string]any) time.Duration {
	return durationArg(args, "timeout_ms", defaultShellTimeout, maxShellTimeout)
}

func durationArg(args map[string]any, key string, def, max time.Duration) time.Duration {
	raw, ok := args[key]
	if !ok || raw == nil {
		return def
	}
	var ms float64
	switch v := raw.(type) {
	case float64:
		ms = v
	case int:
		ms = float64(v)
	case int64:
		ms = float64(v)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return def
		}
		ms = parsed
	default:
		return def
	}
	if ms <= 0 || math.IsNaN(ms) || math.IsInf(ms, 0) {
		return def
	}
	d := time.Duration(ms) * time.Millisecond
	if d > max {
		return max
	}
	return d
}

func shellSessionID(args map[string]any) (int, error) {
	if id, err := numericID(args, "session_id"); err == nil {
		return id, nil
	}
	return numericID(args, "id")
}

func numericID(args map[string]any, key string) (int, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return 0, fmt.Errorf("%s is required", key)
	}
	switch v := raw.(type) {
	case float64:
		if v <= 0 || math.Trunc(v) != v {
			return 0, fmt.Errorf("%s must be a positive integer, got %v", key, v)
		}
		return int(v), nil
	case int:
		if v <= 0 {
			return 0, fmt.Errorf("%s must be a positive integer, got %v", key, v)
		}
		return v, nil
	case string:
		id, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || id <= 0 {
			return 0, fmt.Errorf("%s must be a positive integer, got %q", key, v)
		}
		return id, nil
	default:
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
}

func formatShellSession(registry *TaskRegistry, id int, output string, running bool) string {
	info := registry.summaryByID(id)
	if info == nil {
		return output
	}
	status := info.Status
	if running {
		status = taskRunning.String()
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "session_id: %d\nstatus: %s\nexit_code: %d\n", id, status, info.ExitCode)
	if running {
		sb.WriteString("Command is still running. Call shell(action=\"wait\", session_id=...) to wait, shell(action=\"poll\", session_id=...) to read output, or shell(action=\"stop\", session_id=...) to stop it.\n")
	}
	if strings.TrimSpace(output) != "" {
		sb.WriteString("output:\n")
		sb.WriteString(output)
		if !strings.HasSuffix(output, "\n") {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func formatShellList(tasks []TaskInfo) string {
	if len(tasks) == 0 {
		return "(no shell sessions)"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%-10s %-8s %-7s %-10s %s\n", "SESSION", "PID", "STATUS", "RUNTIME", "COMMAND")
	for _, task := range tasks {
		cmd := task.Command
		if len(cmd) > 40 {
			cmd = cmd[:37] + "..."
		}
		fmt.Fprintf(&sb, "%-10d %-8d %-7s %-10s %s\n",
			task.ID, task.Pid, task.Status, task.Duration, cmd)
	}
	return sb.String()
}
