package shell

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

type nonReapingProcess struct{}

func (nonReapingProcess) Stdout() io.ReadCloser         { return nil }
func (nonReapingProcess) Stderr() io.ReadCloser         { return nil }
func (nonReapingProcess) Wait() error                   { return nil }
func (nonReapingProcess) Terminate(time.Duration) error { return nil }

type retryableProcess struct {
	task     *shellTask
	attempts int
}

func (*retryableProcess) Stdout() io.ReadCloser { return nil }
func (*retryableProcess) Stderr() io.ReadCloser { return nil }
func (*retryableProcess) Wait() error           { return nil }
func (p *retryableProcess) Terminate(time.Duration) error {
	p.attempts++
	if p.attempts == 1 {
		return errors.New("temporary stop failure")
	}
	p.task.markEnded(taskKilled, -1)
	return nil
}

func TestTaskRegistryReservesNameBeforeLaunch(t *testing.T) {
	registry := newTaskRegistry()
	defer registry.cancel()
	name, err := registry.reserveIdentity("session-a", "download")
	if err != nil {
		t.Fatalf("first reservation: %v", err)
	}
	defer registry.releaseReservation("session-a", name)
	if _, err := registry.reserveIdentity("session-a", "download"); err == nil || !strings.Contains(err.Error(), "already starting") {
		t.Fatalf("duplicate in-flight name should fail, got %v", err)
	}
}

func TestTaskRegistryCapsRunningProcesses(t *testing.T) {
	registry := newTaskRegistry()
	defer registry.cancel()
	for id := 1; id <= maxTasks; id++ {
		name := fmt.Sprintf("task_%d", id)
		registry.tasks[reservationKey("session-a", name)] = &shellTask{name: name, owner: "session-a", status: taskRunning}
	}
	if _, err := registry.reserveIdentity("session-a", "one-too-many"); err == nil || !strings.Contains(err.Error(), "limit reached") {
		t.Fatalf("running process cap should reject another start, got %v", err)
	}
}

func TestCompactProcessTextKeepsUTF8Valid(t *testing.T) {
	got := compactProcessText("下载大型模型并启动服务", 8)
	if !strings.HasSuffix(got, "...") || !strings.Contains(got, "下载") {
		t.Fatalf("unexpected compact text: %q", got)
	}
	if strings.ToValidUTF8(got, "replacement") != got {
		t.Fatalf("compact text is not valid UTF-8: %q", got)
	}
}

func TestTaskAllowsOnlyOneActiveMonitor(t *testing.T) {
	task := &shellTask{}
	if !task.beginMonitor() {
		t.Fatal("first monitor should be accepted")
	}
	if task.beginMonitor() {
		t.Fatal("second concurrent monitor should be rejected")
	}
	task.endMonitor()
	if !task.beginMonitor() {
		t.Fatal("monitor slot was not released")
	}
	task.endMonitor()
}

func TestStopTaskRequiresConfirmedTerminalState(t *testing.T) {
	registry := newTaskRegistry()
	defer registry.cancel()
	task := &shellTask{
		name: "stuck", status: taskRunning, proc: nonReapingProcess{},
		done: make(chan struct{}), changed: make(chan struct{}, 1),
	}
	if _, err := registry.stopTask(task); err == nil || !strings.Contains(err.Error(), "did not reach a terminal state") {
		t.Fatalf("unconfirmed stop should fail, got %v", err)
	}
}

func TestTaskRegistryShutdownCanRetryAfterFailure(t *testing.T) {
	registry := newTaskRegistry()
	process := &retryableProcess{}
	task := &shellTask{
		name: "retry", status: taskRunning, proc: process,
		done: make(chan struct{}), changed: make(chan struct{}, 1),
	}
	process.task = task
	registry.tasks[reservationKey("", task.name)] = task

	if err := registry.shutdown(); err == nil {
		t.Fatal("first shutdown should report the stop failure")
	}
	select {
	case <-registry.lifecycle.Done():
		t.Fatal("failed shutdown cancelled the process owner")
	default:
	}
	if err := registry.shutdown(); err != nil {
		t.Fatalf("retry shutdown: %v", err)
	}
	if process.attempts != 2 {
		t.Fatalf("terminate attempts = %d, want 2", process.attempts)
	}
}
