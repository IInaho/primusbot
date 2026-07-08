package guiapp

import (
	"strings"
	"testing"

	"nekocode/common"
)

func TestCompactConfirmArgsEditUsesV2Fields(t *testing.T) {
	req := common.NewConfirmRequest("edit", map[string]any{
		"path":       "/tmp/file.go",
		"oldString":  strings.Repeat("a", 250),
		"newString":  "next",
		"replaceAll": true,
		"patch":      "legacy",
		"_preview":   "diff",
	}, common.ConfirmKindPermission)

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
	app := NewApp()
	req := common.NewConfirmRequest("shell", map[string]any{"command": "go get example.com/pkg"}, common.ConfirmKindPermission)
	req.CanEscalatePermission = true

	app.confirmMu.Lock()
	app.confs["confirm-1"] = req
	app.confirmMu.Unlock()

	app.ReplyConfirmDecision("confirm-1", true, true)
	reply := <-req.Response
	if !reply.Allowed || !reply.Remember {
		t.Fatalf("reply = %+v, want allowed+remember", reply)
	}
	if reply.AllowWithPermission {
		t.Fatalf("ReplyConfirmDecision must not auto-escalate permission; got %+v", reply)
	}
}

func TestReplyConfirmWithPermission(t *testing.T) {
	app := NewApp()
	req := common.NewConfirmRequest("shell", map[string]any{"command": "go get example.com/pkg"}, common.ConfirmKindPermission)
	req.CanEscalatePermission = true

	app.confirmMu.Lock()
	app.confs["confirm-2"] = req
	app.confirmMu.Unlock()

	app.ReplyConfirmWithPermission("confirm-2", true, true, true)
	reply := <-req.Response
	if !reply.Allowed || !reply.Remember || !reply.AllowWithPermission {
		t.Fatalf("reply = %+v, want allowed+remember+permission", reply)
	}
}

func TestReplyConfirmWithPermissionIgnoredWhenCannotEscalate(t *testing.T) {
	app := NewApp()
	// A non-privileged tool's confirm request must not silently gain
	// AllowWithPermission even if the frontend mistakenly passes true.
	req := common.NewConfirmRequest("read", map[string]any{"path": "/tmp/x"}, common.ConfirmKindPermission)
	req.CanEscalatePermission = false

	app.confirmMu.Lock()
	app.confs["confirm-3"] = req
	app.confirmMu.Unlock()

	app.ReplyConfirmWithPermission("confirm-3", true, false, true)
	reply := <-req.Response
	if !reply.Allowed {
		t.Fatalf("reply should be allowed, got %+v", reply)
	}
	if reply.AllowWithPermission {
		t.Fatalf("AllowWithPermission must be forced false for non-escalatable tools, got %+v", reply)
	}
}
