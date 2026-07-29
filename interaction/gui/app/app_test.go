package app

import (
	"context"
	"strings"
	"testing"
	"time"

	controlruntime "nekocode/runtime"
)

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
	confirm       func(controlruntime.ConfirmRequest) controlruntime.ConfirmReply
	allowEscalate bool
	replies       chan controlruntime.ConfirmReply
}

func (b *approvalBot) Run(string, controlruntime.RunCallbacks) (string, error) {
	req := controlruntime.ConfirmRequest{
		ToolName:              "shell",
		Args:                  map[string]any{"command": "go get example.com/pkg"},
		Kind:                  controlruntime.ConfirmKindPermission,
		CanEscalatePermission: b.allowEscalate,
	}
	reply := b.confirm(req)
	b.replies <- reply
	return "", nil
}

func (b *approvalBot) ExecuteCommand(string) (string, controlruntime.CmdResult) {
	return "", controlruntime.CmdNone
}
func (b *approvalBot) SkillHint() (string, bool)      { return "", false }
func (b *approvalBot) Stats() controlruntime.BotStats { return controlruntime.BotStats{} }
func (b *approvalBot) CommandNames() []string         { return nil }
func (b *approvalBot) ConfigureRuntime(callbacks controlruntime.ControlCallbacks) {
	b.confirm = callbacks.Confirm
}
func (b *approvalBot) Steer(string)                                     {}
func (b *approvalBot) Abort()                                           {}
func (b *approvalBot) Close()                                           {}
func (b *approvalBot) ProviderModel() (provider, model string)          { return "", "" }
func (b *approvalBot) SessionMessages() []controlruntime.DisplayMessage { return nil }
func (b *approvalBot) MemoryView(controlruntime.MemoryScope) controlruntime.MemoryView {
	return controlruntime.MemoryView{}
}

func startApprovalApp(t *testing.T, allowEscalate bool) (*App, string, <-chan controlruntime.ConfirmReply) {
	t.Helper()
	bot := &approvalBot{
		allowEscalate: allowEscalate,
		replies:       make(chan controlruntime.ConfirmReply, 1),
	}
	rt := controlruntime.New(bot)
	t.Cleanup(rt.Close)
	app := &App{ctx: context.Background(), rt: rt}

	_, err := rt.Submit(context.Background(), controlruntime.Input{
		Kind:   controlruntime.InputMessage,
		Source: controlruntime.SourceRef{Kind: "test"},
		Text:   "run approval",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pending := rt.PendingApprovals()
		if len(pending) == 1 {
			return app, pending[0].ID, bot.replies
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
