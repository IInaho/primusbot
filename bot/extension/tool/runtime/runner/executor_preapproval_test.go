package runner

import (
	"context"
	"strings"
	"testing"

	"nekocode/bot/extension/tool/runtime/core"
	"nekocode/bot/extension/tool/runtime/permission"
	"nekocode/protocol"
)

// Regression: a unified pre-approval only covers the capabilities
// shown in the confirm dialog. A tool whose actual escalation request is
// broader than the prediction must NOT be silently privileged — it falls
// back to a fresh dialog showing the real request.
func TestPreApprovalDoesNotCoverBroaderActualRequest(t *testing.T) {
	// Tool always escalates to process.host; the sandbox args predict only
	// net.outbound, so the unified dialog includes network access.
	tool := &permissionTool{fakeTool: fakeTool{name: "shell", mode: core.ModeSequential}}
	tool.req = core.PermissionRequest{Reason: "needs host", Capabilities: []string{core.CapProcessHost}}
	e := newTestExecutor(fakeRegistry{"shell": tool})
	e.SetPermissionPolicy(permission.PermissionsDecl{}, "/repo", "/home/user")

	var confirms int
	e.SetConfirmFn(func(protocol.ConfirmRequest) protocol.ConfirmReply {
		confirms++
		if confirms == 1 {
			// First dialog: approve the command and displayed network access.
			return protocol.ConfirmReply{Allowed: true}
		}
		// Second dialog (actual process.host escalation): deny.
		return protocol.Deny()
	})

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{
		ID:   "1",
		Name: "shell",
		Args: map[string]any{"command": "curl https://example.com", "network": true},
	}})[0]

	if got.Error == "" || !strings.Contains(got.Error, "denied") {
		t.Fatalf("out-of-scope escalation must not succeed silently: %+v", got)
	}
	if tool.privilegedCalls != 0 {
		t.Fatalf("privilegedCalls = %d, want 0 (process.host was never approved)", tool.privilegedCalls)
	}
	if confirms != 2 {
		t.Fatalf("confirms = %d, want 2 (broader actual request must re-prompt)", confirms)
	}
}

// The inverse: an actual request within the predicted scope still rides the
// pre-approval without a second dialog.
func TestPreApprovalCoversPredictedRequest(t *testing.T) {
	tool := &permissionTool{fakeTool: fakeTool{name: "shell", mode: core.ModeSequential}}
	tool.req = core.PermissionRequest{Reason: "needs network", Capabilities: []string{core.CapNetOutbound}}
	e := newTestExecutor(fakeRegistry{"shell": tool})
	e.SetPermissionPolicy(permission.PermissionsDecl{}, "/repo", "/home/user")

	var confirms int
	e.SetConfirmFn(func(request protocol.ConfirmRequest) protocol.ConfirmReply {
		confirms++
		if request.Approval == nil || !request.Approval.Combined {
			t.Fatalf("unified approval context missing: %+v", request)
		}
		return protocol.ConfirmReply{Allowed: true}
	})

	got := e.ExecuteBatch(context.Background(), []core.ToolCallItem{{
		ID:   "1",
		Name: "shell",
		Args: map[string]any{"command": "curl https://example.com", "network": true},
	}})[0]

	if got.Error != "" || !strings.Contains(got.Output, "privileged ok") {
		t.Fatalf("in-scope pre-approval should run privileged: %+v", got)
	}
	if confirms != 1 {
		t.Fatalf("confirms = %d, want 1 (no second dialog for in-scope request)", confirms)
	}
}

func TestEscalationApprovalCoverage(t *testing.T) {
	predicted := &core.PermissionRequest{Capabilities: []string{core.CapNetOutbound}}
	a := escalationApproval{predicted: predicted}
	if !a.requestCoveredBy(core.PermissionRequest{Capabilities: []string{core.CapNetOutbound}}) {
		t.Fatal("same capabilities must be covered")
	}
	if a.requestCoveredBy(core.PermissionRequest{Capabilities: []string{core.CapProcessHost}}) {
		t.Fatal("process.host must not be covered by a net.outbound prediction")
	}
	if (escalationApproval{}).requestCoveredBy(core.PermissionRequest{}) {
		t.Fatal("nil prediction covers nothing")
	}
}
