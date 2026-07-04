package impl

// Profile describes the sandbox environment for a command. The public
// sandbox.Profile type is a type alias pointing at this struct.
//
// A zero-value Profile (with only Workspace set) applies the strictest
// isolation: no outbound network, only the workspace directory is writable,
// system directories are read-only, and /tmp is an isolated tmpfs.
type Profile struct {
	// Workspace is the primary working directory, bind-mounted read-write
	// at its host-absolute path. Required.
	Workspace string

	// Network controls outbound network access.
	//   false (default) = isolated network namespace (loopback only)
	//   true            = share host network namespace
	Network bool

	// WritePaths are extra host directories bind-mounted as read-write
	// inside the sandbox, in addition to Workspace. Use cases: package
	// manager caches, authorized external project directories.
	// Paths may use ~/ prefixes; they are resolved to absolute paths.
	WritePaths []string

	// StagingRoot is an internal field used by the native backend to pass
	// the host-side mountpoint to the re-exec'd child process (via JSON).
	// Callers should not set it — it is assigned automatically.
	StagingRoot string
}
