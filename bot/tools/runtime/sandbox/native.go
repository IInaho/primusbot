//go:build linux
// +build linux

package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// childFlag is the sentinel argument that tells a re-exec'd process to
// behave as the sandbox child (set up namespaces and exec the command).
const childFlag = "--nekocode-sandbox-child"

// defaultStagingRoot is used when the parent did not supply one. It is a
// shared path, so the parent must NOT clean it up (see RunNativeBash).
const defaultStagingRoot = "/tmp/.nekocode-sandbox-root"

// IsNativeAvailable reports whether the current system can run the native
// Linux-namespaced sandbox. It requires Linux and working user namespaces.
func IsNativeAvailable() bool {
	if os.Getenv("NEKOCODE_DISABLE_NATIVE_SANDBOX") != "" {
		return false
	}
	cmd := exec.Command("unshare", "--user", "--map-root-user", "true")
	return cmd.Run() == nil
}

// RunNativeBash executes command inside a fresh set of Linux namespaces
// (user, mount, net, pid, ipc, uts) created without any external binary.
// It re-execs the current binary with childFlag; the child process builds
// an isolated filesystem view (via pivot_root) and execs /bin/bash -c command.
func RunNativeBash(ctx context.Context, command string, profile BashProfile, timeout time.Duration) (string, error) {
	if profile.Workspace == "" {
		return "", fmt.Errorf("sandbox workspace is required")
	}

	ws, err := filepath.Abs(profile.Workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}
	profile.Workspace = ws

	cachePaths, err := allowedCachePaths(profile.CachePaths)
	if err != nil {
		return "", err
	}
	profile.CachePaths = cachePaths

	// The parent creates the staging mountpoint on the host /tmp filesystem
	// and cleans it up after the child exits. Using a unique dir avoids
	// collisions between concurrent sandbox runs. When MkdirTemp fails we
	// fall back to a shared default path, which must NOT be removed (it
	// would break concurrent runs).
	createdStaging := false
	if profile.StagingRoot == "" {
		if staging, err := os.MkdirTemp("/tmp", "nekocode-sb-"); err == nil {
			profile.StagingRoot = staging
			createdStaging = true
		} else {
			profile.StagingRoot = defaultStagingRoot
		}
	}
	if err := os.MkdirAll(profile.StagingRoot, 0o700); err != nil {
		return "", fmt.Errorf("create staging root: %w", err)
	}
	if createdStaging {
		defer os.RemoveAll(profile.StagingRoot)
	}

	profileJSON, err := json.Marshal(profile)
	if err != nil {
		return "", fmt.Errorf("marshal profile: %w", err)
	}

	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get executable path: %w", err)
	}

	args := []string{childFlag, string(profileJSON), command}

	// Default to an isolated network namespace. Only when the caller has been
	// granted public network access do we keep the host network namespace.
	var cloneFlags uintptr = syscall.CLONE_NEWUSER |
		syscall.CLONE_NEWNS |
		syscall.CLONE_NEWIPC |
		syscall.CLONE_NEWUTS |
		syscall.CLONE_NEWPID |
		syscall.CLONE_NEWNET
	if profile.Network {
		// Network was granted: share the host network namespace instead of
		// creating an isolated (loopback-only) one.
		cloneFlags &^= syscall.CLONE_NEWNET
	}
	sysproc := &syscall.SysProcAttr{
		Cloneflags: cloneFlags,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
		GidMappingsEnableSetgroups: false,
		Setpgid:                    true,
	}

	// Namespace creation (user/mount/pid/...) fails with EPERM on kernels or
	// container runtimes that disable user namespaces; classify that so the
	// router can fall back to the Landlock backend.
	unavail := func(stderr string, runErr error) bool {
		return strings.Contains(stderr, "operation not permitted") ||
			strings.Contains(runErr.Error(), "operation not permitted")
	}

	return runChild(ctx, self, args, nil, profile.Workspace, timeout, sysproc, unavail, "namespace creation failed")
}

// handleSandboxChild is invoked from init() when the process has been
// re-exec'd by RunNativeBash. It sets up the filesystem view and execs
// the target command, never returning.
func handleSandboxChild() {
	if len(os.Args) < 4 || os.Args[1] != childFlag {
		return
	}

	var profile BashProfile
	if err := json.Unmarshal([]byte(os.Args[2]), &profile); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox child: invalid profile: %v\n", err)
		os.Exit(1)
	}
	command := os.Args[3]

	if err := sandboxChildSetupAndExec(profile, command); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox child: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func init() {
	handleSandboxChild()
}

// sandboxChildSetupAndExec builds an isolated filesystem view and execs the
// command. It uses pivot_root so the workspace keeps its host-absolute path
// (absolute paths in commands still work) while /tmp, /home and the system
// directories are isolated or read-only.
func sandboxChildSetupAndExec(profile BashProfile, command string) error {
	root := profile.StagingRoot
	if root == "" {
		root = defaultStagingRoot
	}

	// 1. Detach all mounts from the host namespace so nothing leaks back.
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("make root private: %w", err)
	}

	// 2. The new root is a fresh tmpfs mounted on the staging mountpoint.
	//    Mounting on a subdirectory (not on /tmp itself) means the host /tmp
	//    — and any workspace that lives under it — stays reachable as a bind
	//    source until we pivot away from it.
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("create staging root: %w", err)
	}
	if err := unix.Mount("tmpfs", root, "tmpfs", 0, "size=384m"); err != nil {
		return fmt.Errorf("mount staging root: %w", err)
	}

	// put_old must exist under the new root before pivot_root.
	if err := os.MkdirAll(filepath.Join(root, ".old"), 0o755); err != nil {
		return fmt.Errorf("create pivot old: %w", err)
	}

	// 3. Bind host paths into the new root. All sources are still reachable
	//    here: the only thing we shadowed is the staging dir itself.
	//    Read-only system dirs: bind, then remount read-only (a single
	//    MS_BIND|MS_RDONLY call is not honoured by older kernels).
	//    Note: in a user namespace, binding the *root* of a separately-mounted
	//    filesystem (e.g. /dev, /run) fails with EINVAL, so those are handled
	//    separately below via fresh tmpfs + entry-level binds.
	for _, src := range []string{"/usr", "/bin", "/lib", "/lib64", "/lib32", "/etc", "/nix/store"} {
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := bindHostPath(root, src, true); err != nil {
			return err
		}
	}

	// /dev: a fresh tmpfs with the essential device nodes bind-mounted from
	// the host (binding the device files works even though binding the /dev
	// mountpoint itself does not).
	if err := setupDev(root); err != nil {
		return err
	}

	// /proc: fresh mount for the new pid namespace (best-effort).
	if err := os.MkdirAll(filepath.Join(root, "proc"), 0o755); err == nil {
		_ = unix.Mount("proc", filepath.Join(root, "proc"), "proc", 0, "")
	}

	// /run: isolated tmpfs. On NixOS /run/current-system/sw holds the system
	// binaries; bind it in if present (best-effort — it may be a mountpoint
	// and fail under some kernels).
	if err := os.MkdirAll(filepath.Join(root, "run"), 0o755); err != nil {
		return fmt.Errorf("mkdir /run: %w", err)
	}
	_ = unix.Mount("tmpfs", filepath.Join(root, "run"), "tmpfs", 0, "size=128m")
	if _, err := os.Stat("/run/current-system/sw"); err == nil {
		_ = bindHostPath(root, "/run/current-system/sw", true)
	}

	// /tmp: isolated tmpfs so writes never reach the host.
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o1777); err != nil {
		return fmt.Errorf("mkdir /tmp: %w", err)
	}
	if err := unix.Mount("tmpfs", filepath.Join(root, "tmp"), "tmpfs", 0, "size=256m"); err != nil {
		return fmt.Errorf("mount /tmp: %w", err)
	}

	// Workspace: bind at its host-absolute path so absolute paths in commands
	// resolve to the real project directory.
	if err := bindHostPath(root, profile.Workspace, false); err != nil {
		return err
	}

	// Cache paths: bind at their host-absolute paths. They all live under the
	// user's home, so with HOME set to the real home, access like ~/.npm
	// resolves to the bound cache while unrelated home entries (~/.ssh) stay
	// invisible and isolated.
	for _, cp := range profile.CachePaths {
		if err := bindHostPath(root, cp, false); err != nil {
			return err
		}
	}

	// 4. Swap the staging tree in as the root.
	if err := unix.PivotRoot(root, filepath.Join(root, ".old")); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}
	// The process cwd still points at the old-root workspace inode (now under
	// /.old). Move to the new-root workspace before detaching the old root.
	if err := os.Chdir(profile.Workspace); err != nil {
		return fmt.Errorf("chdir workspace: %w", err)
	}
	if err := unix.Unmount("/.old", unix.MNT_DETACH); err != nil {
		// Non-fatal: a lazy detach keeps the old root around briefly but it is
		// invisible inside the new tree.
	}

	// 5. Environment + exec the command.
	env := []string{
		"HOME=" + hostHome(),
		"PATH=" + sandboxPath(),
		"LC_ALL=C.UTF-8",
		"TERM=xterm-256color",
	}

	shells := []string{"/bin/bash", "/bin/sh", "/usr/bin/bash", "/usr/bin/sh"}
	var lastErr error
	for _, shell := range shells {
		if _, err := os.Stat(shell); err != nil {
			continue
		}
		name := "bash"
		if strings.HasSuffix(shell, "/sh") {
			name = "sh"
		}
		lastErr = unix.Exec(shell, []string{name, "-c", command}, env)
		if lastErr == nil {
			return nil
		}
	}
	if lastErr != nil {
		return fmt.Errorf("exec shell: %w", lastErr)
	}
	return fmt.Errorf("no usable shell found (tried %v)", shells)
}

// setupDev creates a minimal /dev tree on a fresh tmpfs inside the staging
// root and bind-mounts the essential host device nodes into it. Binding the
// individual device files works in a user namespace; binding the /dev
// mountpoint itself does not.
func setupDev(root string) error {
	devDir := filepath.Join(root, "dev")
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		return fmt.Errorf("mkdir /dev: %w", err)
	}
	if err := unix.Mount("tmpfs", devDir, "tmpfs", 0, "size=64m,mode=755"); err != nil {
		return fmt.Errorf("mount /dev: %w", err)
	}
	for _, dev := range []string{"/dev/null", "/dev/zero", "/dev/urandom", "/dev/random", "/dev/tty", "/dev/console"} {
		if _, err := os.Stat(dev); err != nil {
			continue
		}
		dst := filepath.Join(root, dev)
		if f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0o666); err == nil {
			f.Close()
		}
		if err := unix.Mount(dev, dst, "", unix.MS_BIND, ""); err != nil {
			return fmt.Errorf("bind %s: %w", dev, err)
		}
	}
	// Convenience symlinks that shells and tools commonly expect.
	for _, link := range []struct{ target, name string }{
		{"/proc/self/fd", filepath.Join(root, "dev", "fd")},
		{"/proc/self/fd/0", filepath.Join(root, "dev", "stdin")},
		{"/proc/self/fd/1", filepath.Join(root, "dev", "stdout")},
		{"/proc/self/fd/2", filepath.Join(root, "dev", "stderr")},
	} {
		_ = os.Symlink(link.target, link.name)
	}
	return nil
}

// bindHostPath bind-mounts the host path src into the staging root at the
// same absolute path. When readOnly is true the bind is remounted read-only.
// The remount is best-effort: some source mounts (notably /nix/store on
// NixOS) are owned by the host's user namespace and refuse flag changes with
// EPERM, but they are already read-only on the host so the bind inherits ro.
func bindHostPath(root, src string, readOnly bool) error {
	if src == "" {
		return nil
	}
	dst := filepath.Join(root, src)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", src, err)
	}
	if err := unix.Mount(src, dst, "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind %s: %w", src, err)
	}
	if readOnly {
		if err := unix.Mount("", dst, "", unix.MS_REMOUNT|unix.MS_BIND|unix.MS_RDONLY, ""); err != nil {
			if err != unix.EPERM {
				return fmt.Errorf("remount read-only %s: %w", src, err)
			}
		}
	}
	return nil
}

// hostHome returns the user's home directory as seen on the host. The child
// inherits the parent environment, so $HOME is available before we replace
// the environment for the exec'd command.
func hostHome() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "/root"
}

// sandboxPath builds PATH for the sandboxed shell. On NixOS the real
// binaries live under /run/current-system/sw/bin; elsewhere the standard
// locations suffice.
func sandboxPath() string {
	if _, err := os.Stat("/run/current-system/sw/bin"); err == nil {
		return "/run/current-system/sw/bin:/usr/local/bin:/usr/bin:/bin"
	}
	return "/usr/local/bin:/usr/bin:/bin"
}
