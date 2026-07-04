// Package core defines the protocol shared by tool implementations,
// executors, and agent runtimes.
package core

import (
	"context"
)

// ExecutionMode controls whether a tool can run concurrently.
type ExecutionMode int

const (
	ModeParallel ExecutionMode = iota
	ModeSequential
)

type ToolCallItem struct {
	ID   string
	Name string
	Args map[string]any
}

type ToolCallResult struct {
	ID     string
	Name   string
	Output string
	Error  string
}

// EffectiveOutput returns Error if non-empty, otherwise Output.
func (r ToolCallResult) EffectiveOutput() string {
	if r.Error != "" {
		return r.Error
	}
	return r.Output
}

// Tool is the interface all tools implement.
type Tool interface {
	Name() string
	Description() string
	Parameters() []Parameter
	ExecutionMode(args map[string]any) ExecutionMode
	Execute(ctx context.Context, args map[string]any) (string, error)
}

type Parameter struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

type Descriptor struct {
	Name        string
	Description string
	Parameters  []Parameter
}

// PermissionRequest describes a capability a tool needs beyond its default
// sandbox. The executor may satisfy it from a persisted grant or ask the user.
type PermissionRequest struct {
	Reason       string
	Capabilities []string
	Scope        string
	Details      map[string]any
}

const (
	// Sandbox-granularity capabilities — when authorized, the command still
	// runs inside the sandbox; only the corresponding restriction is relaxed.
	CapNetOutbound  = "net.outbound"  // share host network namespace
	CapFsWriteCache = "fs.write.cache" // bind package manager cache dirs (~/.npm, ~/go/pkg/mod, ~/.cargo, ...)
	CapFsWritePath  = "fs.write.path"  // bind write_paths argument dirs as read-write

	// CapProcessHost: run entirely on the host without any sandbox isolation.
	// Special: this capability is NEVER persisted — every invocation prompts
	// the user regardless of prior grants. It is the escape hatch of last
	// resort (needs TTY, Docker socket, writes to /etc, ...), not a routine
	// escalation target.
	CapProcessHost = "process.host"
)

type RequiredPermissionError struct {
	Request PermissionRequest
}

func (e RequiredPermissionError) Error() string {
	if e.Request.Reason == "" {
		return "permission required"
	}
	return "permission required: " + e.Request.Reason
}

func (e RequiredPermissionError) PermissionRequest() PermissionRequest {
	return e.Request
}

// PermissionError is returned by tools when sandbox execution cannot proceed
// without an explicit grant.
type PermissionError interface {
	error
	PermissionRequest() PermissionRequest
}

// PrivilegedTool can retry a failed tool call with a concrete grant supplied by
// the executor. Implementations should still apply the narrowest restriction
// possible and must not treat this as blanket trust.
type PrivilegedTool interface {
	ExecuteWithPermission(ctx context.Context, args map[string]any, grant PermissionRequest) (string, error)
}
