package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	controlruntime "nekocode/runtime"
)

type imageSessionRunner struct {
	cwd string
}

func (*imageSessionRunner) Run(context.Context, string, controlruntime.RunHost) (string, error) {
	return "", nil
}

func (*imageSessionRunner) CurrentSessionID() string { return "session_1" }

func (r *imageSessionRunner) ListSessions() []controlruntime.SessionMeta {
	return []controlruntime.SessionMeta{{ID: "session_1", CWD: r.cwd}}
}

func (*imageSessionRunner) SessionMessages() []controlruntime.DisplayMessage { return nil }

func TestReadImageBase64RejectsSymlinkOutsideSession(t *testing.T) {
	cwd := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.png")
	if err := os.WriteFile(outside, []byte("not really an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(cwd, "linked.png")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	runner := &imageSessionRunner{cwd: cwd}
	rt := controlruntime.New(runner, controlruntime.Services{
		CurrentSessionID: runner.CurrentSessionID,
		ListSessions:     runner.ListSessions,
		SessionMessages:  runner.SessionMessages,
	})
	app := &App{ctx: context.Background(), rt: rt}
	if _, err := app.ReadImageBase64(link); err == nil || !strings.Contains(err.Error(), "outside allowed") {
		t.Fatalf("ReadImageBase64 error = %v, want path rejection", err)
	}
}

func TestCompactConfirmArgsEditUsesV2Fields(t *testing.T) {
	req := controlruntime.ConfirmRequest{
		ToolName: "edit",
		Args: map[string]any{
			"path":       "/tmp/file.go",
			"oldString":  strings.Repeat("a", 250),
			"newString":  "next",
			"replaceAll": true,
			"patch":      "legacy",
			"_preview":   "diff",
		},
		Kind: controlruntime.ConfirmKindPermission,
	}

	got := compactConfirmArgs(req)
	if got["path"] != "/tmp/file.go" {
		t.Fatalf("path = %v", got["path"])
	}
	if _, ok := got["patch"]; ok {
		t.Fatalf("legacy patch should not be exposed: %#v", got)
	}
	if got["replaceAll"] != true {
		t.Fatalf("replaceAll = %v", got["replaceAll"])
	}
	old, _ := got["oldString"].(string)
	if len(old) != 203 || !strings.HasSuffix(old, "...") {
		t.Fatalf("oldString was not truncated: len=%d value=%q", len(old), old)
	}
}

func TestReplyConfirmDecisionNoLongerAutoEscalates(t *testing.T) {
	// Regression guard for the GUI bug where ReplyConfirmDecision silently
	// granted AllowWithPermission whenever req.CanEscalatePermission was
	// true — making the GUI's "允许" button indistinguishable from a "允许
	// 并授权" button. Allow-with-permission now requires an explicit call
	// to ReplyConfirmWithPermission.
	app, id, replies := startApprovalApp(t, true)

	app.ReplyConfirmDecision(id, true, true)
	reply := waitConfirmReply(t, replies)
	if !reply.Allowed || !reply.Remember {
		t.Fatalf("reply = %+v, want allowed+remember", reply)
	}
	if reply.AllowWithPermission {
		t.Fatalf("ReplyConfirmDecision must not auto-escalate permission; got %+v", reply)
	}
}

func TestReplyConfirmWithPermission(t *testing.T) {
	app, id, replies := startApprovalApp(t, true)

	app.ReplyConfirmWithPermission(id, true, true, true)
	reply := waitConfirmReply(t, replies)
	if !reply.Allowed || !reply.Remember || !reply.AllowWithPermission {
		t.Fatalf("reply = %+v, want allowed+remember+permission", reply)
	}
}

func TestReplyConfirmWithPermissionIgnoredWhenCannotEscalate(t *testing.T) {
	app, id, replies := startApprovalApp(t, false)

	app.ReplyConfirmWithPermission(id, true, false, true)
	reply := waitConfirmReply(t, replies)
	if !reply.Allowed {
		t.Fatalf("reply should be allowed, got %+v", reply)
	}
	if reply.AllowWithPermission {
		t.Fatalf("AllowWithPermission must be forced false for non-escalatable tools, got %+v", reply)
	}
}

type approvalBot struct {
	allowEscalate bool
	replies       chan controlruntime.ConfirmReply
}

func (b *approvalBot) Run(_ context.Context, _ string, host controlruntime.RunHost) (string, error) {
	req := controlruntime.ConfirmRequest{
		ToolName:              "shell",
		Args:                  map[string]any{"command": "go get example.com/pkg"},
		Kind:                  controlruntime.ConfirmKindPermission,
		CanEscalatePermission: b.allowEscalate,
	}
	reply := host.Confirm(req)
	b.replies <- reply
	return "", nil
}

func startApprovalApp(t *testing.T, allowEscalate bool) (*App, string, <-chan controlruntime.ConfirmReply) {
	t.Helper()
	bot := &approvalBot{
		allowEscalate: allowEscalate,
		replies:       make(chan controlruntime.ConfirmReply, 1),
	}
	rt := controlruntime.New(bot, controlruntime.Services{})
	t.Cleanup(func() {
		if err := rt.Close(); err != nil {
			t.Error(err)
		}
	})
	app := &App{ctx: context.Background(), rt: rt}

	_, err := rt.StartRun(context.Background(), controlruntime.Input{
		Source: controlruntime.SourceRef{Kind: "test"},
		Text:   "run approval",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		run, ok := rt.CurrentRun()
		if ok && len(run.Approvals) == 1 {
			return app, run.Approvals[0].ID, bot.replies
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for pending approval")
	return nil, "", nil
}

func waitConfirmReply(t *testing.T, replies <-chan controlruntime.ConfirmReply) controlruntime.ConfirmReply {
	t.Helper()
	select {
	case reply := <-replies:
		return reply
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for confirm reply")
		return controlruntime.ConfirmReply{}
	}
}
