package shell

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"nekocode/bot/tools/runtime/sandbox"
	"nekocode/bot/tools/runtime/toolutil"
)

// LogRingSize is the per-task in-memory ring buffer size. 256 KiB holds
// roughly the last few minutes of a typical dev server's output.
const LogRingSize = 256 * 1024

// LogsReturnBytes is the default cap on bytes returned by session reads.
const LogsReturnBytes = 8 * 1024

// MaxTasks caps the total number of task records kept in memory. Once the
// limit is hit, the oldest ended task is dropped before a new one is added.
const MaxTasks = 64

// taskRetentionTTL is how long an ended task stays in the registry before
// being auto-removed. Running tasks are never evicted by this timer.
const taskRetentionTTL = 10 * time.Second

type taskStatus int

const (
	taskRunning taskStatus = iota
	taskExited
	taskKilled
	taskTimeout
)

func (s taskStatus) String() string {
	switch s {
	case taskRunning:
		return "running"
	case taskExited:
		return "exited"
	case taskKilled:
		return "killed"
	case taskTimeout:
		return "timeout"
	}
	return "unknown"
}

// displayStatus returns the human-facing status string. A clean exit
// (exit code 0) is reported as "done", a non-zero exit as "failed", so
// users don't mistake success for failure.
func (t *shellTask) displayStatus() string {
	if t.status == taskRunning {
		return "running"
	}
	if t.status == taskExited && t.exitCode == 0 {
		return "done"
	}
	if t.status == taskExited {
		return "failed"
	}
	return t.status.String()
}

// shellTask is a single tracked shell session.
type shellTask struct {
	id        int
	cmd       string
	pid       int
	startedAt time.Time

	// Protected by mu.
	mu       sync.Mutex
	status   taskStatus
	timedOut bool
	endedAt  time.Time
	exitCode int
	logs     []byte // ring buffer
	proc     sandbox.ProcessLike
	removeAt time.Time // zero means no pending auto-removal
}

func (t *shellTask) appendLogs(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.logs)+len(chunk) > LogRingSize {
		trim := len(t.logs) + len(chunk) - LogRingSize
		if trim < len(t.logs) {
			t.logs = t.logs[trim:]
		} else {
			t.logs = t.logs[:0]
		}
	}
	t.logs = append(t.logs, chunk...)
}

func (t *shellTask) markEnded(status taskStatus, code int) {
	t.mu.Lock()
	if t.timedOut {
		status = taskTimeout
		code = -1
	}
	t.status = status
	t.endedAt = time.Now()
	t.exitCode = code
	t.mu.Unlock()
}

// scheduleRemoval sets a deadline after which the task auto-removes from the
// registry. Calling this again (e.g. on access) pushes the deadline back.
func (t *shellTask) scheduleRemoval(now time.Time) {
	t.mu.Lock()
	t.removeAt = now.Add(taskRetentionTTL)
	t.mu.Unlock()
}

// shouldRemove reports whether the task's retention TTL has elapsed.
func (t *shellTask) shouldRemove(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.removeAt.IsZero() && now.After(t.removeAt)
}

func (t *shellTask) markTimedOut() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.status != taskRunning {
		return false
	}
	t.timedOut = true
	t.status = taskTimeout
	t.endedAt = time.Now()
	t.exitCode = -1
	return true
}

// snapshot returns the most recent `max` bytes from the log ring.
func (t *shellTask) snapshot(max int) []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := len(t.logs)
	if n == 0 {
		return nil
	}
	if max <= 0 || max > n {
		max = n
	}
	out := make([]byte, max)
	copy(out, t.logs[n-max:])
	return out
}

func (t *shellTask) summary() TaskInfo {
	t.mu.Lock()
	defer t.mu.Unlock()
	runtime := time.Since(t.startedAt)
	status := t.status
	exitCode := t.exitCode
	if status != taskRunning {
		runtime = t.endedAt.Sub(t.startedAt)
	}
	return TaskInfo{
		ID:       t.id,
		Command:  t.cmd,
		Pid:      t.pid,
		Status:   status.String(),
		Duration: formatDuration(runtime),
		ExitCode: exitCode,
	}
}

// TaskInfo is the per-task row returned by the list action.
type TaskInfo struct {
	ID       int
	Command  string
	Pid      int
	Status   string
	Duration string
	ExitCode int
}

// TaskRegistry manages shell sessions in a single NekoCode process.
type TaskRegistry struct {
	mu     sync.Mutex
	tasks  map[int]*shellTask
	nextID int
}

func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{tasks: make(map[int]*shellTask)}
}

// cleanupExpired removes ended tasks whose retention TTL has elapsed.
func (r *TaskRegistry) cleanupExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for id, t := range r.tasks {
		if t.shouldRemove(now) {
			delete(r.tasks, id)
		}
	}
}

type StartRequest struct {
	Command    string
	Profile    sandbox.Profile
	Host       bool
	Timeout    time.Duration
	SampleWait time.Duration
}

// Start launches command as a tracked shell session. It returns the session
// and the initial output sample for immediate LLM feedback.
func (r *TaskRegistry) Start(ctx context.Context, req StartRequest) (*shellTask, []byte, error) {
	r.mu.Lock()
	r.nextID++
	id := r.nextID
	r.mu.Unlock()

	var (
		proc sandbox.ProcessLike
		err  error
	)
	if req.Host {
		proc, err = backend.StartHost(ctx, req.Command)
	} else {
		proc, err = backend.Start(ctx, req.Command, req.Profile)
	}
	if err != nil {
		return nil, nil, err
	}

	t := &shellTask{
		id:        id,
		cmd:       req.Command,
		pid:       proc.PID(),
		startedAt: time.Now(),
		status:    taskRunning,
		proc:      proc,
	}

	go drainToRing(t, proc.Stdout())
	go drainToRing(t, proc.Stderr())

	// Reap on exit and record status.
	go func() {
		err := proc.Wait()
		status, code := taskStatusFromWait(err)
		t.markEnded(status, code)
		t.scheduleRemoval(time.Now())
	}()

	if req.Timeout > 0 {
		go func() {
			timer := time.NewTimer(req.Timeout)
			defer timer.Stop()
			<-timer.C
			if !t.markTimedOut() {
				return
			}
			t.appendLogs([]byte(fmt.Sprintf("\n[command timed out after %s]\n", req.Timeout.Truncate(time.Millisecond))))
			_ = proc.Terminate(2 * time.Second)
		}()
	}

	// Capture startup output so the LLM sees immediate feedback without
	// needing to call logs separately.
	sampleWait := req.SampleWait
	if sampleWait <= 0 {
		sampleWait = time.Second
	}
	waitStartupSample(t, sampleWait)
	initial := toolutil.StripAnsi(string(t.snapshot(LogsReturnBytes)))

	r.mu.Lock()
	r.tasks[id] = t
	r.evictOld()
	r.mu.Unlock()

	return t, []byte(initial), nil
}

func waitStartupSample(t *shellTask, max time.Duration) {
	deadline := time.NewTimer(max)
	defer deadline.Stop()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline.C:
			return
		case <-tick.C:
			t.mu.Lock()
			ended := t.status != taskRunning
			t.mu.Unlock()
			if ended {
				return
			}
		}
	}
}

func taskStatusFromWait(err error) (taskStatus, int) {
	if err == nil {
		return taskExited, 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if ws.Signaled() {
				return taskKilled, -1
			}
			return taskExited, ws.ExitStatus()
		}
		return taskExited, exitErr.ExitCode()
	}
	return taskExited, 1
}

// Logs returns the tail of task id's log ring buffer (ANSI-stripped).
// running indicates whether the task is still alive. Accessing a task
// refreshes its retention timer.
func (r *TaskRegistry) Logs(id int) (string, bool, error) {
	r.mu.Lock()
	t, ok := r.tasks[id]
	r.mu.Unlock()
	if !ok {
		return "", false, fmt.Errorf("task %d not found", id)
	}
	t.scheduleRemoval(time.Now())
	snap := t.snapshot(LogsReturnBytes)
	t.mu.Lock()
	status := t.status
	t.mu.Unlock()
	return toolutil.StripAnsi(string(snap)), status == taskRunning, nil
}

func (r *TaskRegistry) Wait(id int, max time.Duration) (string, bool, error) {
	r.mu.Lock()
	t, ok := r.tasks[id]
	r.mu.Unlock()
	if !ok {
		return "", false, fmt.Errorf("task %d not found", id)
	}
	if max > 0 {
		waitTaskEnded(t, max)
	}
	t.scheduleRemoval(time.Now())
	return r.Logs(id)
}

// List returns a summary of all tracked tasks (after evicting any whose
// retention TTL has elapsed).
func (r *TaskRegistry) List() []TaskInfo {
	r.cleanupExpired()
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]TaskInfo, 0, len(r.tasks))
	for _, t := range r.tasks {
		info := t.summary()
		info.Status = t.displayStatus()
		out = append(out, info)
	}
	return out
}

// summaryByID returns the summary for a single task, or nil if not found.
// Accessing a task refreshes its retention timer so it stays available for
// follow-up reads.
func (r *TaskRegistry) summaryByID(id int) *TaskInfo {
	r.mu.Lock()
	t, ok := r.tasks[id]
	r.mu.Unlock()
	if !ok {
		return nil
	}
	t.scheduleRemoval(time.Now())
	info := t.summary()
	info.Status = t.displayStatus()
	return &info
}

// Stop sends SIGTERM to task id's process group. If the process does not
// exit within 2 s, escalates to SIGKILL. Returns a human-readable summary.
func (r *TaskRegistry) Stop(id int) (string, error) {
	r.mu.Lock()
	t, ok := r.tasks[id]
	r.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("task %d not found", id)
	}

	t.mu.Lock()
	if t.status != taskRunning {
		was := t.status.String()
		t.mu.Unlock()
		return "", fmt.Errorf("task %d is already %s", id, was)
	}
	proc := t.proc
	t.mu.Unlock()

	if proc == nil {
		return "", fmt.Errorf("task %d has no process handle", id)
	}
	if err := proc.Terminate(2 * time.Second); err != nil {
		return "", fmt.Errorf("failed to signal task %d: %w", id, err)
	}
	status := waitTaskEnded(t, 250*time.Millisecond)
	if status == taskKilled {
		return fmt.Sprintf("task %d killed", id), nil
	}
	return fmt.Sprintf("task %d stopped", id), nil
}

// StopAll terminates every running task currently tracked by the registry.
func (r *TaskRegistry) StopAll() []error {
	r.mu.Lock()
	running := make([]int, 0, len(r.tasks))
	for id, t := range r.tasks {
		t.mu.Lock()
		if t.status == taskRunning {
			running = append(running, id)
		}
		t.mu.Unlock()
	}
	r.mu.Unlock()

	var errs []error
	for _, id := range running {
		if _, err := r.Stop(id); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func waitTaskEnded(t *shellTask, max time.Duration) taskStatus {
	deadline := time.NewTimer(max)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		t.mu.Lock()
		status := t.status
		t.mu.Unlock()
		if status != taskRunning {
			return status
		}
		select {
		case <-deadline.C:
			return taskRunning
		case <-tick.C:
		}
	}
}

// evictOld drops the oldest ended task when over MaxTasks. Caller must hold mu.
func (r *TaskRegistry) evictOld() {
	if len(r.tasks) <= MaxTasks {
		return
	}
	var oldestID int
	var oldestEnd time.Time
	for tid, t := range r.tasks {
		t.mu.Lock()
		ended := t.status != taskRunning
		end := t.endedAt
		t.mu.Unlock()
		if !ended {
			continue
		}
		if oldestID == 0 || end.Before(oldestEnd) {
			oldestID = tid
			oldestEnd = end
		}
	}
	if oldestID > 0 {
		delete(r.tasks, oldestID)
	}
}

func drainToRing(t *shellTask, r io.Reader) {
	if r == nil {
		return
	}
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			t.appendLogs([]byte(line))
		}
		if err != nil {
			if err != io.EOF {
				t.appendLogs([]byte(fmt.Sprintf("[read error: %v]\n", err)))
			}
			return
		}
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return d.Truncate(time.Millisecond).String()
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}
