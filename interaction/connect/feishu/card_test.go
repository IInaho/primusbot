package feishu

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	larkcallback "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"

	controlruntime "nekocode/runtime"
)

func cardButtons(card map[string]any) []any {
	elements := card["elements"].([]any)
	return elements[len(elements)-1].(map[string]any)["actions"].([]any)
}

func buttonDecision(t *testing.T, btn any) (id, decision string) {
	t.Helper()
	id, decision, err := decodeCardActionValue(btn.(map[string]any)["value"].(map[string]interface{}))
	if err != nil {
		t.Fatalf("button value decode: %v", err)
	}
	return id, decision
}

func TestApprovalCardThreeButtons(t *testing.T) {
	card := approvalCard(controlruntime.ApprovalView{
		ID:       "ap_1",
		ToolName: "shell",
		Args:     map[string]any{"command": "go test ./..."},
	})

	buttons := cardButtons(card)
	if len(buttons) != 3 {
		t.Fatalf("buttons = %d, want 3 without escalation", len(buttons))
	}
	wantTexts := []string{"批准一次", "永久允许", "拒绝"}
	wantDecisions := []string{decisionApprove, decisionRemember, decisionReject}
	wantStyles := []string{"default", "primary", "danger"}
	for i, btn := range buttons {
		b := btn.(map[string]any)
		if got := b["text"].(map[string]any)["content"]; got != wantTexts[i] {
			t.Fatalf("button %d text = %v, want %q", i, got, wantTexts[i])
		}
		if b["type"] != wantStyles[i] {
			t.Fatalf("button %d style = %v, want %q", i, b["type"], wantStyles[i])
		}
		id, dec := buttonDecision(t, btn)
		if id != "ap_1" || dec != wantDecisions[i] {
			t.Fatalf("button %d value = %v/%v", i, id, dec)
		}
	}
}

func TestApprovalCardEscalationAddsFourthButton(t *testing.T) {
	card := approvalCard(controlruntime.ApprovalView{
		ID:                    "ap_2",
		ToolName:              "shell",
		CanEscalatePermission: true,
	})
	buttons := cardButtons(card)
	if len(buttons) != 4 {
		t.Fatalf("buttons = %d, want 4 with escalation", len(buttons))
	}
	last := buttons[3].(map[string]any)
	if got := last["text"].(map[string]any)["content"]; got != "允许并授权" {
		t.Fatalf("escalation button text = %v", got)
	}
	id, dec := buttonDecision(t, buttons[3])
	if id != "ap_2" || dec != decisionEscalate {
		t.Fatalf("escalation value = %v/%v", id, dec)
	}
}

func TestApprovalDecisionFor(t *testing.T) {
	cases := []struct {
		decision string
		want     controlruntime.ApprovalDecision
	}{
		{decisionApprove, controlruntime.ApprovalDecision{Allowed: true}},
		{decisionRemember, controlruntime.ApprovalDecision{Allowed: true, Remember: true}},
		{decisionReject, controlruntime.ApprovalDecision{Allowed: false}},
		{decisionEscalate, controlruntime.ApprovalDecision{Allowed: true, AllowWithPermission: true}},
	}
	for _, tc := range cases {
		got, err := approvalDecisionFor(tc.decision)
		if err != nil || got != tc.want {
			t.Fatalf("approvalDecisionFor(%q) = %+v, %v; want %+v", tc.decision, got, err, tc.want)
		}
	}
	if _, err := approvalDecisionFor("maybe"); err == nil {
		t.Fatal("unknown decision should error")
	}
}

func TestResolvedCardVerdicts(t *testing.T) {
	p := controlruntime.ApprovalView{ID: "ap_1", ToolName: "edit", Args: map[string]any{"path": "a.go"}}
	cases := []struct {
		decision string
		title    string
		template string
	}{
		{decisionApprove, "已批准: edit", "green"},
		{decisionRemember, "已永久允许: edit", "green"},
		{decisionReject, "已拒绝: edit", "red"},
		{decisionEscalate, "已批准并授权: edit", "green"},
	}
	for _, tc := range cases {
		card := resolvedCard(p, tc.decision)
		header := card["header"].(map[string]any)
		if got := header["title"].(map[string]any)["content"]; got != tc.title {
			t.Fatalf("%s title = %v, want %q", tc.decision, got, tc.title)
		}
		if header["template"] != tc.template {
			t.Fatalf("%s template = %v", tc.decision, header["template"])
		}
		if elements := card["elements"].([]any); len(elements) != 1 {
			t.Fatalf("%s: resolved card should have no action block", tc.decision)
		}
	}
}

func TestApprovalSummaryVariants(t *testing.T) {
	withCmd := approvalSummary(controlruntime.ApprovalView{ToolName: "shell", Args: map[string]any{"command": "ls"}})
	if !containsAll(withCmd, "shell", "ls") {
		t.Fatalf("command summary = %q", withCmd)
	}
	withPath := approvalSummary(controlruntime.ApprovalView{ToolName: "edit", Args: map[string]any{"path": "a.go", "_preview": "diff..."}})
	if !containsAll(withPath, "edit", "a.go", "diff...") {
		t.Fatalf("path summary = %q", withPath)
	}
	bare := approvalSummary(controlruntime.ApprovalView{ToolName: "read"})
	if bare == "" {
		t.Fatal("bare summary should not be empty")
	}
}

func TestDecodeCardActionValue(t *testing.T) {
	if _, _, err := decodeCardActionValue(map[string]interface{}{"decision": "approve"}); err == nil {
		t.Fatal("missing approval id should error")
	}
	if _, _, err := decodeCardActionValue(map[string]interface{}{"approval_id": "ap_1", "decision": "maybe"}); err == nil {
		t.Fatal("unknown decision should error")
	}
	if _, _, err := decodeCardActionValue(map[string]interface{}{"approval_id": 42, "decision": "approve"}); err == nil {
		t.Fatal("non-string approval id should error")
	}
	for _, dec := range []string{decisionApprove, decisionRemember, decisionReject, decisionEscalate} {
		id, got, err := decodeCardActionValue(map[string]interface{}{"approval_id": "ap_1", "decision": dec})
		if err != nil || id != "ap_1" || got != dec {
			t.Fatalf("valid decode %q = %v, %v, %v", dec, id, got, err)
		}
	}
}

func TestIsAlreadyResolvedErr(t *testing.T) {
	if !isAlreadyResolvedErr(fmt.Errorf("runtime: approval ap_1 already resolved")) {
		t.Fatal("already resolved should match")
	}
	if !isAlreadyResolvedErr(fmt.Errorf("runtime: approval ap_1 not pending")) {
		t.Fatal("not pending should match")
	}
	if isAlreadyResolvedErr(errors.New("connection refused")) || isAlreadyResolvedErr(nil) {
		t.Fatal("unrelated errors should not match")
	}
}

// --- handleCardAction dispatch ---

type stubRuntime struct {
	controlruntime.Runtime
	approveID  string
	decision   controlruntime.ApprovalDecision
	approveErr error
	published  []controlruntime.Event
}

func (s *stubRuntime) Approve(_ context.Context, approvalID string, decision controlruntime.ApprovalDecision) error {
	s.approveID = approvalID
	s.decision = decision
	return s.approveErr
}

func (s *stubRuntime) Publish(ev controlruntime.Event) { s.published = append(s.published, ev) }

func cardEvent(value map[string]interface{}, operatorOpenID string) *larkcallback.CardActionTriggerEvent {
	ev := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{Value: value},
		},
	}
	if operatorOpenID != "" {
		ev.Event.Operator = &larkcallback.Operator{OpenID: operatorOpenID}
	}
	return ev
}

func pairedConnector(t *testing.T, rt controlruntime.Runtime) *Connector {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	cfg := Config{AppID: "cli", AppSecret: "sec"}
	cfg.finishPairing("ou_owner", "oc_1")
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return New(rt)
}

func TestHandleCardActionDecisions(t *testing.T) {
	cases := []struct {
		decision string
		want     controlruntime.ApprovalDecision
		toast    string
	}{
		{decisionApprove, controlruntime.ApprovalDecision{Allowed: true}, "已批准"},
		{decisionRemember, controlruntime.ApprovalDecision{Allowed: true, Remember: true}, "已永久允许"},
		{decisionReject, controlruntime.ApprovalDecision{Allowed: false}, "已拒绝"},
		{decisionEscalate, controlruntime.ApprovalDecision{Allowed: true, AllowWithPermission: true}, "已批准并授权"},
	}
	for _, tc := range cases {
		t.Run(tc.decision, func(t *testing.T) {
			rt := &stubRuntime{}
			c := pairedConnector(t, rt)
			c.approvals.Store("ap_1", controlruntime.ApprovalView{ID: "ap_1", ToolName: "shell"})

			resp, err := c.handleCardAction(context.Background(), cardEvent(cardActionValue("ap_1", tc.decision), "ou_owner"))
			if err != nil {
				t.Fatal(err)
			}
			if rt.approveID != "ap_1" || rt.decision != tc.want {
				t.Fatalf("Approve(%q, %+v), want %+v", rt.approveID, rt.decision, tc.want)
			}
			if resp.Toast == nil || resp.Toast.Content != tc.toast || resp.Toast.Type != "success" {
				t.Fatalf("toast = %+v, want %q", resp.Toast, tc.toast)
			}
			if resp.Card == nil {
				t.Fatal("expected resolved card update")
			}
			header := resp.Card.Data.(map[string]any)["header"].(map[string]any)
			if title := header["title"].(map[string]any)["content"]; title != tc.toast+": shell" {
				t.Fatalf("resolved title = %v", title)
			}
			if _, ok := c.approvals.Load("ap_1"); ok {
				t.Fatal("approval should be removed from the in-flight map")
			}
		})
	}
}

func TestHandleCardActionRejectWithoutStoredView(t *testing.T) {
	rt := &stubRuntime{}
	c := pairedConnector(t, rt)
	resp, err := c.handleCardAction(context.Background(), cardEvent(cardActionValue("ap_2", decisionReject), "ou_owner"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Toast.Content != "已拒绝" {
		t.Fatalf("toast = %+v", resp.Toast)
	}
	if resp.Card != nil {
		t.Fatal("no stored view → no card update")
	}
}

func TestHandleCardActionAlreadyResolved(t *testing.T) {
	rt := &stubRuntime{approveErr: fmt.Errorf("runtime: approval ap_1 already resolved")}
	c := pairedConnector(t, rt)
	resp, err := c.handleCardAction(context.Background(), cardEvent(cardActionValue("ap_1", decisionApprove), "ou_owner"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Toast.Type != "info" || resp.Toast.Content != "该审批已处理" {
		t.Fatalf("toast = %+v", resp.Toast)
	}
}

func TestHandleCardActionRejectsBadValueAndNonOwner(t *testing.T) {
	rt := &stubRuntime{}
	c := pairedConnector(t, rt)

	// Bad value: no approval id.
	resp, err := c.handleCardAction(context.Background(), cardEvent(map[string]interface{}{"decision": "approve"}, "ou_owner"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Toast.Type != "error" || rt.approveID != "" {
		t.Fatalf("bad value: toast=%+v approveID=%q", resp.Toast, rt.approveID)
	}

	// Non-owner operator must not reach the runtime.
	resp, err = c.handleCardAction(context.Background(), cardEvent(cardActionValue("ap_1", decisionApprove), "ou_stranger"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Toast.Type != "error" || rt.approveID != "" {
		t.Fatalf("non-owner: toast=%+v approveID=%q", resp.Toast, rt.approveID)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
