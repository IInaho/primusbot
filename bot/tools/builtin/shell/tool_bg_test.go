package shell

import (
	"strings"
	"testing"
	"time"

	"nekocode/bot/tools/runtime/core"
)

func TestBg_StartEchoAndLogs(t *testing.T) {
	tool := &BgTool{}
	out, err := tool.ExecuteWithPermission(t.Context(), map[string]any{
		"action":  "start",
		"command": "echo hello-from-bg",
	}, permReq())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !strings.Contains(out, "hello-from-bg") {
		t.Fatalf("expected hello-from-bg in start output, got:\n%s", out)
	}

	// The task should now be in the registry (may already be exited since
	// echo exits instantly, but the record is there).
	registry := tool.registryOnce()
	summary := registry.List()
	if len(summary) != 1 {
		t.Fatalf("expected 1 task, got %d", len(summary))
	}
}

func TestBg_LogsReturnsOutput(t *testing.T) {
	tool := &BgTool{}
	_, err := tool.ExecuteWithPermission(t.Context(), map[string]any{
		"action":  "start",
		"command": "echo logs-line-1 && echo logs-line-2",
	}, permReq())
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Wait for output to arrive in the ring buffer.
	time.Sleep(500 * time.Millisecond)

	logsOut, err := tool.Execute(t.Context(), map[string]any{
		"action": "logs",
		"id":     1,
	})
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if !strings.Contains(logsOut, "logs-line-1") {
		t.Fatalf("expected logs-line-1 in logs output, got:\n%s", logsOut)
	}
	if !strings.Contains(logsOut, "logs-line-2") {
		t.Fatalf("expected logs-line-2 in logs output, got:\n%s", logsOut)
	}
}

func TestBg_ListShowsTasks(t *testing.T) {
	tool := &BgTool{}
	_, err := tool.ExecuteWithPermission(t.Context(), map[string]any{
		"action":  "start",
		"command": "echo listed-task && sleep 30",
	}, permReq())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	listOut, err := tool.Execute(t.Context(), map[string]any{
		"action": "list",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(listOut, "listed-task") {
		t.Fatalf("expected listed-task in list output, got:\n%s", listOut)
	}
	if !strings.Contains(listOut, "running") {
		t.Fatalf("expected 'running' status in list output, got:\n%s", listOut)
	}
}

func TestBg_StopKillsProcess(t *testing.T) {
	tool := &BgTool{}
	_, err := tool.ExecuteWithPermission(t.Context(), map[string]any{
		"action":  "start",
		"command": "sleep 300",
	}, permReq())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	stopOut, err := tool.Execute(t.Context(), map[string]any{
		"action": "stop",
		"id":     1,
	})
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !strings.Contains(stopOut, "task 1 stopped") && !strings.Contains(stopOut, "task 1 killed") {
		t.Fatalf("expected stopped/killed in stop output, got:\n%s", stopOut)
	}

	// Verify the task shows as ended.
	time.Sleep(300 * time.Millisecond)
	listOut, err := tool.Execute(t.Context(), map[string]any{
		"action": "list",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(listOut, "exited") && !strings.Contains(listOut, "killed") {
		t.Fatalf("expected ended status after stop, got:\n%s", listOut)
	}
}

func TestBg_ShutdownStopsRunningTasks(t *testing.T) {
	tool := &BgTool{}
	_, err := tool.ExecuteWithPermission(t.Context(), map[string]any{
		"action":  "start",
		"command": "sleep 300",
	}, permReq())
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if errs := tool.Shutdown(); len(errs) != 0 {
		t.Fatalf("shutdown errors: %v", errs)
	}
	time.Sleep(300 * time.Millisecond)

	listOut, err := tool.Execute(t.Context(), map[string]any{"action": "list"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if strings.Contains(listOut, "running") {
		t.Fatalf("shutdown left task running: %s", listOut)
	}
}

func TestBg_ExitedTaskLogsShowsSummary(t *testing.T) {
	tool := &BgTool{}
	_, err := tool.ExecuteWithPermission(t.Context(), map[string]any{
		"action":  "start",
		"command": "echo done-then-exit",
	}, permReq())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	// Wait for exit + drain.
	time.Sleep(2500 * time.Millisecond)

	logsOut, err := tool.Execute(t.Context(), map[string]any{
		"action": "logs",
		"id":     1,
	})
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if !strings.Contains(logsOut, "done-then-exit") {
		t.Fatalf("expected done-then-exit in logs, got:\n%s", logsOut)
	}
	if !strings.Contains(logsOut, "exited") {
		t.Fatalf("expected 'exited' summary in logs, got:\n%s", logsOut)
	}
}

func TestBg_ListEmpty(t *testing.T) {
	tool := &BgTool{}
	out, err := tool.Execute(t.Context(), map[string]any{
		"action": "list",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "no background tasks") {
		t.Fatalf("expected empty message, got:\n%s", out)
	}
}

func TestBg_StartMissingCommand(t *testing.T) {
	tool := &BgTool{}
	_, err := tool.ExecuteWithPermission(t.Context(), map[string]any{
		"action": "start",
	}, permReq())
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestBg_InvalidAction(t *testing.T) {
	tool := &BgTool{}
	_, err := tool.ExecuteWithPermission(t.Context(), map[string]any{
		"action": "bogus",
	}, permReq())
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestBg_ExplicitNetworkRequiresPermission(t *testing.T) {
	tool := &BgTool{}
	_, err := tool.Execute(t.Context(), map[string]any{
		"action":  "start",
		"command": "npm run dev",
		"network": true,
	})
	if err == nil {
		t.Fatal("expected permission error")
	}
	permErr, ok := err.(core.PermissionError)
	if !ok {
		t.Fatalf("expected core.PermissionError, got %T: %v", err, err)
	}
	req := permErr.PermissionRequest()
	if len(req.Capabilities) != 1 || req.Capabilities[0] != core.CapNetOutbound {
		t.Fatalf("expected [net.outbound], got %+v", req)
	}
	if req.Scope != "project" {
		t.Fatalf("background local network permission must be project scope, got %q", req.Scope)
	}
}

func TestBg_ExplicitNetworkRejectsUnrelatedPermissionGrant(t *testing.T) {
	tool := &BgTool{}
	_, err := tool.ExecuteWithPermission(t.Context(), map[string]any{
		"action":  "start",
		"command": "npm run dev",
		"network": true,
	}, core.PermissionRequest{Capabilities: []string{core.CapFsWritePath}})
	if err == nil {
		t.Fatal("expected wrong permission grant to be rejected")
	}
	permErr, ok := err.(core.PermissionError)
	if !ok {
		t.Fatalf("expected core.PermissionError, got %T: %v", err, err)
	}
	req := permErr.PermissionRequest()
	if len(req.Capabilities) != 1 || req.Capabilities[0] != core.CapNetOutbound {
		t.Fatalf("expected [net.outbound], got %+v", req)
	}
}

func TestBg_StopNonexistent(t *testing.T) {
	tool := &BgTool{}
	_, err := tool.ExecuteWithPermission(t.Context(), map[string]any{
		"action": "stop",
		"id":     999,
	}, permReq())
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestBg_LogsNonexistent(t *testing.T) {
	tool := &BgTool{}
	_, err := tool.ExecuteWithPermission(t.Context(), map[string]any{
		"action": "logs",
		"id":     999,
	}, permReq())
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestBg_StopAlreadyExited(t *testing.T) {
	tool := &BgTool{}
	_, err := tool.ExecuteWithPermission(t.Context(), map[string]any{
		"action":  "start",
		"command": "exit 0",
	}, permReq())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(2500 * time.Millisecond) // wait for exit

	_, err = tool.ExecuteWithPermission(t.Context(), map[string]any{
		"action": "stop",
		"id":     1,
	}, permReq())
	if err == nil {
		t.Fatal("expected error for already-exited task")
	}
}

func TestBg_IDMustBePositiveInteger(t *testing.T) {
	for _, args := range []map[string]any{
		{"action": "logs", "id": 1.5},
		{"action": "logs", "id": 0},
		{"action": "logs", "id": -1},
	} {
		tool := &BgTool{}
		_, err := tool.Execute(t.Context(), args)
		if err == nil {
			t.Fatalf("expected invalid id error for args %+v", args)
		}
		if !strings.Contains(err.Error(), "positive integer") {
			t.Fatalf("error = %v, want positive integer", err)
		}
	}
}

func TestBg_NameAndParameters(t *testing.T) {
	tool := &BgTool{}
	if tool.Name() != "bg" {
		t.Fatalf("expected name 'bg', got %q", tool.Name())
	}
	params := tool.Parameters()
	if len(params) != 6 {
		t.Fatalf("expected 6 parameters, got %d", len(params))
	}
}

func TestBg_DescriptionCoversAllActions(t *testing.T) {
	desc := (&BgTool{}).Description()
	for _, want := range []string{"start", "logs", "list", "stop", "network=true"} {
		if !strings.Contains(desc, want) {
			t.Fatalf("description missing %q", want)
		}
	}
}

func TestBg_MultipleTasks(t *testing.T) {
	tool := &BgTool{}
	for i := 0; i < 3; i++ {
		_, err := tool.ExecuteWithPermission(t.Context(), map[string]any{
			"action":  "start",
			"command": "echo task" + string(rune('0'+i)),
		}, permReq())
		if err != nil {
			t.Fatalf("start %d: %v", i, err)
		}
	}
	time.Sleep(500 * time.Millisecond)
	registry := tool.registryOnce()
	summary := registry.List()
	if len(summary) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(summary))
	}
}

func permReq() core.PermissionRequest {
	return core.PermissionRequest{Capabilities: []string{"process.host"}}
}
