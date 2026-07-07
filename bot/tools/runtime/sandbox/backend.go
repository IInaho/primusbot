// Package sandbox provides a pluggable command-execution sandbox. Callers depend
// on the Backend interface and Profile configuration defined here; the actual
// sandbox implementations (Linux namespaces, Landlock, tbsb, host execution)
// live in the impl subpackage and are not part of the public surface.
package sandbox

import (
	"context"
	"io"
	"time"

	"nekocode/bot/tools/runtime/sandbox/impl"
)

// Backend is the contract every sandbox executor satisfies. Callers depend
// on this interface (not any concrete type) so the sandbox can be substituted
// in tests.
//
// The default implementation is DefaultBackend, which selects the best
// available OS backend (Linux namespaces → Landlock → tbsb).
type Backend interface {
	// Run executes command inside the sandbox described by profile.
	//
	// Error contract:
	//   nil              — command succeeded
	//   UnavailableError — no sandbox backend could be used; caller should
	//                      request process.host authorization then retry
	//                      via RunHost.
	//   other error      — command failed inside the sandbox (non-zero exit,
	//                      timeout, sandbox setup failure). Return the error
	//                      verbatim to the LLM — it carries the stderr that
	//                      lets the LLM decide which capability to request.
	Run(ctx context.Context, command string, profile Profile, timeout time.Duration) (string, error)

	// Start launches command inside the sandbox described by profile and
	// returns immediately with process handles and stdout/stderr pipes.
	// Callers are responsible for draining pipes and eventually calling Wait.
	Start(ctx context.Context, command string, profile Profile) (*Process, error)

	// RunHost executes command on the host with NO sandbox isolation.
	// Use only when the caller has an explicit process.host grant.
	RunHost(ctx context.Context, command string, timeout time.Duration) (string, error)

	// StartHost launches command on the host with NO sandbox isolation.
	// Use only when the caller has an explicit process.host grant.
	StartHost(ctx context.Context, command string) (*Process, error)

	// IsAvailable reports whether at least one sandbox backend is usable.
	// When false, Run will return UnavailableError without executing.
	IsAvailable() bool
}

// Profile describes the sandbox environment for a command.
//
// A zero-value Profile (with only Workspace set) applies the strictest
// isolation: no outbound network, only the workspace directory is writable,
// system directories are read-only, and /tmp is an isolated tmpfs.
//
// Fields opened by capability grants incrementally relax isolation without
// leaving the sandbox:
//
//   - Network: share the host network namespace instead of an isolated one.
//   - WritePaths: extra host directories bind-mounted as read-write.
//
// The sandbox package does NOT enforce a whitelist on WritePaths — the
// calling layer is responsible for authorization. Paths may use ~/ prefixes
// (resolved to the user's home) and are cleaned + deduplicated internally.
//
// This is a type alias to impl.Profile (defined in the impl subpackage) so
// the public sandbox package exposes the same struct callers use, while the
// canonical definition lives with the implementations.
type Profile = impl.Profile

type SandboxMode = impl.SandboxMode

const (
	ModeWorkspaceWrite = impl.ModeWorkspaceWrite
	ModeReadOnly       = impl.ModeReadOnly
)

// Process is a started sandbox or host process.
type Process = impl.Process

// ProcessLike is the subset of Process used by callers that need to inject
// fakes in tests.
type ProcessLike interface {
	PID() int
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser
	Wait() error
	Terminate(grace time.Duration) error
}

// UnavailableError is returned when no sandbox backend could be used.
// Callers should treat it as a signal to request host-execution permission.
//
// This is a type alias to impl.UnavailableError so that the public sandbox
// package exposes the same error type callers match on, while the canonical
// definition lives with the implementations.
type UnavailableError = impl.UnavailableError
