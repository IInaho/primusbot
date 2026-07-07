package shell

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/sandbox"
)

// BgTool manages long-running processes that outlive a single bash call.
// It provides start/logs/list/stop actions via the "bg" tool.
//
// LLM interaction pattern:
//
//	bg(action="start", command="npm run dev")
//	  → user grants permission (or denies)
//	  → returns task id + first ~1 s of output
//	bg(action="logs", id=1)
//	  → returns most recent ~8 KiB of output
//	bg(action="list")
//	  → returns all tasks and their status
//	bg(action="stop", id=1)
//	  → sends SIGTERM → SIGKILL to the process group
type BgTool struct {
	// mu protects the one-time init of the global registry.
	mu       sync.Once
	registry *TaskRegistry
}

func (t *BgTool) registryOnce() *TaskRegistry {
	t.mu.Do(func() {
		t.registry = NewTaskRegistry()
	})
	return t.registry
}

// Shutdown terminates all running background tasks owned by this tool.
func (t *BgTool) Shutdown() []error {
	return t.registryOnce().StopAll()
}

func (t *BgTool) Name() string { return "bg" }

func (t *BgTool) Description() string {
	return `Manage long-running background processes (dev servers, file watchers, REPLs).

Use "bash" for short commands (< 2 minutes). Use "bg" for processes that run
indefinitely and must stay alive between agent turns.

ACTION START:
  bg(action="start", command="npm run dev")
  → User will be prompted to approve. The tool blocks until approval.
  → On success, returns the task id and the first ~1 s of output.
  → The task continues running after the tool returns.

ACTION LOGS:
  bg(action="logs", id=1)
  → Returns the most recent ~8 KiB of the task's combined stdout/stderr.
  → If the task has exited, includes an "[exited code 0]" summary at the end.

ACTION LIST:
  bg(action="list")
  → Returns a table of all running and recently completed tasks.

ACTION STOP:
  bg(action="stop", id=1)
  → Sends SIGTERM, then SIGKILL after 2 s if the process does not exit.
  → Returns a summary of the action taken.

IMPORTANT:
- Start uses the normal command approval flow first, then runs in the sandbox.
- Dev-server commands that need local/network access must set network=true explicitly.
- sandbox_mode="host" requests unsandboxed host execution and prompts every time.
- The runtime never infers network or host access from command text or output.
- Always use "bg" for: npm run dev, vite, webpack-dev-server, webpack --watch,
  file watchers, REPLs, tail -f, long-running servers.
- Never use "bash" for commands that block forever — bash has a 120 s timeout and
  will kill the process.`
}

func (t *BgTool) Parameters() []core.Parameter {
	return []core.Parameter{
		{
			Name:        "action",
			Type:        "string",
			Required:    true,
			Description: `Action to perform: "start", "logs", "list", or "stop".`,
		},
		{
			Name:        "command",
			Type:        "string",
			Required:    false,
			Description: `Shell command to run in the background. Required when action is "start".`,
		},
		{
			Name:        "id",
			Type:        "integer",
			Required:    false,
			Description: "Task id. Required for logs and stop actions.",
		},
		{
			Name:        "sandbox_mode",
			Type:        "string",
			Required:    false,
			Description: "Sandbox mode for start: read-only, workspace-write (default), or host.",
		},
		{
			Name:        "network",
			Type:        "boolean",
			Required:    false,
			Description: "Request outbound/local network access for start. Requires permission.",
		},
		{
			Name:        "writable_roots",
			Type:        "array",
			Required:    false,
			Description: "Extra writable directories outside the workspace for start. Requires permission.",
		},
	}
}

func (t *BgTool) ExecutionMode(args map[string]any) core.ExecutionMode {
	action, _ := args["action"].(string)
	if action == "start" {
		return core.ModeSequential
	}
	return core.ModeParallel
}

func (t *BgTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	return t.run(ctx, args, t.registryOnce(), core.PermissionRequest{})
}

func (t *BgTool) ExecuteWithPermission(ctx context.Context, args map[string]any, req core.PermissionRequest) (string, error) {
	return t.run(ctx, args, t.registryOnce(), req)
}

func (t *BgTool) run(ctx context.Context, args map[string]any, registry *TaskRegistry, grant core.PermissionRequest) (string, error) {
	action, _ := args["action"].(string)

	switch action {
	case "start":
		return t.start(ctx, args, registry, grant)
	case "logs":
		return t.logs(args, registry)
	case "list":
		return t.list(registry)
	case "stop":
		return t.stop(args, registry)
	default:
		return "", fmt.Errorf("unknown action %q: must be start, logs, list, or stop", action)
	}
}

func (t *BgTool) start(ctx context.Context, args map[string]any, registry *TaskRegistry, grant core.PermissionRequest) (string, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return "", fmt.Errorf("command is required when action is \"start\"")
	}
	workspace, _ := os.Getwd()
	reqProfile := sandboxRequestFromArgs(args)
	requestedCaps := reqProfile.permissionCapabilities()
	if hasCapability(requestedCaps, core.CapProcessHost) && !hasCapability(grant.Capabilities, core.CapProcessHost) {
		return "", permissionRequired(
			"bg command requests unsandboxed host execution",
			[]string{core.CapProcessHost},
			"once",
			workspace,
			reqProfile.WritableRoots,
		)
	}
	if len(requestedCaps) > 0 && !containsAllCapabilities(grant.Capabilities, requestedCaps) {
		return "", permissionRequired(
			fmt.Sprintf("bg command requests sandbox profile: %s", strings.Join(requestedCaps, ", ")),
			requestedCaps,
			"project",
			workspace,
			reqProfile.WritableRoots,
		)
	}
	host := hasCapability(grant.Capabilities, core.CapProcessHost)
	var profile sandbox.Profile
	if !host {
		var err error
		profile, err = buildProfileFromRequest(workspace, reqProfile, grant.Capabilities)
		if err != nil {
			return "", err
		}
	}

	req := StartRequest{
		Command: command,
		Profile: profile,
		Host:    host,
	}
	task, initial, err := registry.Start(ctx, req)
	if err != nil {
		var unavailable sandbox.UnavailableError
		if errors.As(err, &unavailable) {
			return "", hostPermission(err.Error(), workspace, nil)
		}
		return "", fmt.Errorf("failed to start background process: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Task %d started (pid %d).\n", task.id, task.pid)
	if len(initial) > 0 {
		fmt.Fprintf(&sb, "First ~1 s of output:\n%s\n", initial)
	}
	fmt.Fprintf(&sb, "Use action=\"logs\" id=%d to see more output.", task.id)
	return sb.String(), nil
}

func (t *BgTool) logs(args map[string]any, registry *TaskRegistry) (string, error) {
	id, err := extractID(args)
	if err != nil {
		return "", err
	}
	output, running, err := registry.Logs(id)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if len(output) > 0 {
		sb.WriteString(output)
	} else {
		sb.WriteString("(no output yet)\n")
	}
	if !running {
		task := registry.summaryByID(id)
		if task != nil {
			fmt.Fprintf(&sb, "\n[Task %d: %s with exit code %d]\n", id, task.Status, task.ExitCode)
		}
	}
	return sb.String(), nil
}

func (t *BgTool) list(registry *TaskRegistry) (string, error) {
	tasks := registry.List()
	if len(tasks) == 0 {
		return "(no background tasks)", nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%-4s %-8s %-7s %-10s %s\n", "ID", "PID", "STATUS", "RUNTIME", "COMMAND")
	for _, task := range tasks {
		cmd := task.Command
		if len(cmd) > 40 {
			cmd = cmd[:37] + "..."
		}
		fmt.Fprintf(&sb, "%-4d %-8d %-7s %-10s %s\n",
			task.ID, task.Pid, task.Status, task.Duration, cmd)
	}
	return sb.String(), nil
}

func (t *BgTool) stop(args map[string]any, registry *TaskRegistry) (string, error) {
	id, err := extractID(args)
	if err != nil {
		return "", err
	}
	return registry.Stop(id)
}

func extractID(args map[string]any) (int, error) {
	idVal, ok := args["id"]
	if !ok {
		return 0, fmt.Errorf("id is required")
	}
	switch v := idVal.(type) {
	case float64:
		if v <= 0 || math.Trunc(v) != v {
			return 0, fmt.Errorf("id must be a positive integer, got %v", v)
		}
		return int(v), nil
	case int:
		if v <= 0 {
			return 0, fmt.Errorf("id must be a positive integer, got %d", v)
		}
		return v, nil
	default:
		return 0, fmt.Errorf("id must be an integer, got %T", idVal)
	}
}
