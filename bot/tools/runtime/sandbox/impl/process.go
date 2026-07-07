package impl

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// Process is a started sandbox or host command.
type Process struct {
	cmd      *exec.Cmd
	cancel   context.CancelFunc
	stopKill func() bool

	stdout io.ReadCloser
	stderr io.ReadCloser
	done   chan struct{}

	doneOnce    sync.Once
	cleanupOnce sync.Once
	cleanup     func()
}

func (p *Process) PID() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

func (p *Process) Stdout() io.ReadCloser {
	if p == nil {
		return nil
	}
	return p.stdout
}

func (p *Process) Stderr() io.ReadCloser {
	if p == nil {
		return nil
	}
	return p.stderr
}

func (p *Process) Wait() error {
	if p == nil || p.cmd == nil {
		return fmt.Errorf("process not started")
	}
	err := p.cmd.Wait()
	p.doneOnce.Do(func() { close(p.done) })
	p.finish()
	return err
}

func (p *Process) Terminate(grace time.Duration) error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return fmt.Errorf("process not started")
	}
	pid := p.cmd.Process.Pid
	if runtime.GOOS == "windows" {
		if err := p.cmd.Process.Kill(); err != nil {
			return err
		}
		p.finish()
		return nil
	}
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	deadline := time.NewTimer(grace)
	defer deadline.Stop()
	for {
		select {
		case <-p.done:
			p.finish()
			return nil
		case <-deadline.C:
			err := syscall.Kill(-pid, syscall.SIGKILL)
			p.finish()
			return err
		}
	}
}

func (p *Process) finish() {
	if p == nil {
		return
	}
	if p.stopKill != nil {
		p.stopKill()
	}
	if p.cancel != nil {
		p.cancel()
	}
	p.cleanupOnce.Do(func() {
		if p.cleanup != nil {
			p.cleanup()
		}
	})
}

func startCommand(ctx context.Context, name string, args []string, env []string, dir string, sysproc *syscall.SysProcAttr, cleanup func()) (*Process, error) {
	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, name, args...)
	cmd.Env = env
	cmd.Dir = dir
	cmd.SysProcAttr = sysproc

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		if cleanup != nil {
			cleanup()
		}
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		if cleanup != nil {
			cleanup()
		}
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		if cleanup != nil {
			cleanup()
		}
		return nil, err
	}
	p := &Process{cmd: cmd, cancel: cancel, stdout: stdout, stderr: stderr, done: make(chan struct{}), cleanup: cleanup}
	p.stopKill = context.AfterFunc(cmdCtx, func() {
		if cmd.Process != nil && runtime.GOOS != "windows" {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	})
	return p, nil
}

// StartHost launches command on the host with NO sandbox isolation.
func StartHost(ctx context.Context, command string) (*Process, error) {
	dir, _ := os.Getwd()
	sysproc := &syscall.SysProcAttr{}
	if runtime.GOOS != "windows" {
		sysproc.Setpgid = true
	}
	return startCommand(ctx, "bash", []string{"-c", command}, nil, dir, sysproc, nil)
}
