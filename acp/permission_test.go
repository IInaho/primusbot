package acp

import (
	"strings"
	"testing"

	controlruntime "nekocode/runtime"
)

func TestPermissionMapping(t *testing.T) {
	summary := approvalSummary(controlruntime.ApprovalView{Approval: &controlruntime.ApprovalContext{
		Risk: "high", Capabilities: []string{"network"}, WritePaths: []string{"/workspace"},
	}})
	if !strings.Contains(summary, "Risk: high") || !strings.Contains(summary, "Capabilities: network") {
		t.Fatalf("approval summary = %q", summary)
	}
	var forged permissionResponse
	forged.Outcome.Outcome = "selected"
	forged.Outcome.OptionID = "allow_always"
	if decision := permissionDecision(forged, true, false); decision.Allowed || decision.Remember {
		t.Fatalf("unadvertised persistent decision accepted: %#v", decision)
	}
}
