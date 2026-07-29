package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nekocode/interaction/connect/core"
	controlruntime "nekocode/runtime"
)

func TestParseStartPayload(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		payload string
		ok      bool
	}{
		{name: "plain start", text: "/start abc123", payload: "abc123", ok: true},
		{name: "addressed start", text: "/start@nekocode_bot abc123", payload: "abc123", ok: true},
		{name: "missing payload", text: "/start", payload: "", ok: true},
		{name: "other command", text: "/help abc123", payload: "", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, ok := parseStartPayload(tt.text)
			if payload != tt.payload || ok != tt.ok {
				t.Fatalf("parseStartPayload(%q) = %q,%v want %q,%v", tt.text, payload, ok, tt.payload, tt.ok)
			}
		})
	}
}

func TestProfilesListShowsActiveOwner(t *testing.T) {
	cfg := Config{
		ActiveProfile: "personal",
		Profiles: []BotProfile{
			{
				Name:        "personal",
				BotUsername: "my_bot",
				Owner:       &Device{UserID: 1, Username: "alice"},
			},
			{Name: "work", BotUsername: "work_bot"},
		},
	}
	got := profilesList(cfg, true)
	for _, want := range []string{
		"* personal @my_bot  running  owner @alice",
		"  work @work_bot  stopped  unpaired",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("profilesList missing %q in:\n%s", want, got)
		}
	}
}

func TestPairProfileRefusesAlreadyPairedOwner(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := Config{
		ActiveProfile: "personal",
		Profiles: []BotProfile{{
			Name:        "personal",
			BotToken:    "123:secret",
			BotUsername: "my_bot",
			Owner:       &Device{UserID: 1, Username: "alice"},
		}},
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	got, err := New(nil).pairProfile(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "already paired with @alice") {
		t.Fatalf("unexpected response: %s", got)
	}
}

func TestUnpairProfileClearsOwner(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := Config{
		ActiveProfile: "personal",
		Profiles: []BotProfile{{
			Name:        "personal",
			BotUsername: "my_bot",
			Owner:       &Device{UserID: 1, Username: "alice"},
			Pairing:     core.Pairing{Nonce: "nonce", Expires: 10},
		}},
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	got, err := New(nil).unpairProfile(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "unpaired") {
		t.Fatalf("unexpected response: %s", got)
	}
	cfg, err = loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Profiles[0]
	if p.Owner != nil || p.Pairing.Nonce != "" || p.Pairing.Expires != 0 {
		t.Fatalf("profile not unpaired: %#v", p)
	}
}

func TestRemoveActiveProfileSelectsNext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := Config{
		ActiveProfile: "personal",
		Profiles: []BotProfile{
			{Name: "personal", BotUsername: "my_bot"},
			{Name: "work", BotUsername: "work_bot"},
		},
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	got, err := New(nil).removeProfile([]string{"personal"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "removed") {
		t.Fatalf("unexpected response: %s", got)
	}
	cfg, err = loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveProfile != "work" || len(cfg.Profiles) != 1 {
		t.Fatalf("unexpected config after remove: %#v", cfg)
	}
}

func TestProfileNameFromBotUsername(t *testing.T) {
	tests := map[string]string{
		"my_nekocode_bot": "my_nekocode",
		"helperbot":       "helper",
		"@WorkBot":        "work",
		"":                "telegram",
	}
	for in, want := range tests {
		if got := profileNameFromBotUsername(in); got != want {
			t.Fatalf("profileNameFromBotUsername(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResetConfigClearsProfiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := Config{
		ActiveProfile: "personal",
		Profiles: []BotProfile{{
			Name:        "personal",
			BotUsername: "my_bot",
			Owner:       &Device{UserID: 1, Username: "alice"},
		}},
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	got, err := New(nil).resetConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "reset") {
		t.Fatalf("unexpected response: %s", got)
	}
	cfg, err = loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Profiles) != 0 || cfg.ActiveProfile != "" {
		t.Fatalf("config not reset: %#v", cfg)
	}
}

func TestEventKeyboard(t *testing.T) {
	approvalKeyboard, ok := New(nil).eventKeyboard(controlruntime.Event{
		Type: controlruntime.EventApprovalRequested,
		Payload: controlruntime.ApprovalView{
			ID: "apr_1",
		},
	})
	if !ok {
		t.Fatal("approval keyboard missing")
	}
	row := approvalKeyboard.InlineKeyboard[0]
	if len(row) != 3 {
		t.Fatalf("approval buttons = %d, want 3 without escalation", len(row))
	}
	wantTexts := []string{"批准一次", "永久允许", "拒绝"}
	wantData := []string{"approve:apr_1", "remember:apr_1", "reject:apr_1"}
	for i, btn := range row {
		if btn.Text != wantTexts[i] || btn.CallbackData != wantData[i] {
			t.Fatalf("button %d = %q %q, want %q %q", i, btn.Text, btn.CallbackData, wantTexts[i], wantData[i])
		}
	}

	escalateKeyboard, ok := New(nil).eventKeyboard(controlruntime.Event{
		Type: controlruntime.EventApprovalRequested,
		Payload: controlruntime.ApprovalView{
			ID:                    "apr_2",
			CanEscalatePermission: true,
		},
	})
	if !ok {
		t.Fatal("escalation keyboard missing")
	}
	escRow := escalateKeyboard.InlineKeyboard[0]
	if len(escRow) != 4 {
		t.Fatalf("escalation buttons = %d, want 4", len(escRow))
	}
	if escRow[3].Text != "允许并授权" || escRow[3].CallbackData != "escalate:apr_2" {
		t.Fatalf("escalation button = %q %q", escRow[3].Text, escRow[3].CallbackData)
	}

	questionKeyboard, ok := New(nil).eventKeyboard(controlruntime.Event{
		Type: controlruntime.EventQuestionRequested,
		Payload: controlruntime.QuestionView{
			ID: "q_1",
			Questions: []controlruntime.QuestionItem{{
				Question: "Proceed?",
				Options:  []controlruntime.QuestionOption{{Label: "Yes"}},
			}},
		},
	})
	if !ok || questionKeyboard.InlineKeyboard[0][0].CallbackData != "answer:q_1:0" {
		t.Fatalf("question keyboard = %#v ok=%v", questionKeyboard, ok)
	}
	dismissRow := questionKeyboard.InlineKeyboard[len(questionKeyboard.InlineKeyboard)-1]
	if dismissRow[0].Text != "忽略" || dismissRow[0].CallbackData != "dismiss:q_1" {
		t.Fatalf("dismiss button = %q %q", dismissRow[0].Text, dismissRow[0].CallbackData)
	}
}

func TestParseAnswerCommand(t *testing.T) {
	id, answer := parseAnswerCommand("/answer q_12 yes please")
	if id != "q_12" || answer != "yes please" {
		t.Fatalf("parse explicit = %q %q", id, answer)
	}
	id, answer = parseAnswerCommand("/answer yes please")
	if id != "" || answer != "yes please" {
		t.Fatalf("parse implicit = %q %q", id, answer)
	}
}

func TestMarkStoppedClearsState(t *testing.T) {
	c := New(nil)
	_, gen1 := c.base.Start(context.Background())
	_, gen2 := c.base.Start(context.Background())

	// A stale generation (from an older Start) must not clear the state.
	c.base.MarkStopped(gen1)
	if !c.base.IsRunning() {
		t.Fatal("stale generation should not clear running state")
	}

	c.base.MarkStopped(gen2)
	if c.base.IsRunning() {
		t.Fatal("current generation should clear running state")
	}
}

func TestEventKeyboardMultiSelect(t *testing.T) {
	view := controlruntime.QuestionView{
		ID: "q_1",
		Questions: []controlruntime.QuestionItem{{
			Multiple: true,
			Options: []controlruntime.QuestionOption{
				{Label: "选项A"}, {Label: "选项B"},
			},
		}},
	}
	ev := controlruntime.Event{Type: controlruntime.EventQuestionRequested, Payload: view}

	conn := New(nil)
	keyboard, ok := conn.eventKeyboard(ev)
	if !ok {
		t.Fatal("multi-select keyboard missing")
	}
	if len(keyboard.InlineKeyboard) != 3 {
		t.Fatalf("rows = %d, want 2 options + confirm/dismiss row", len(keyboard.InlineKeyboard))
	}
	last := keyboard.InlineKeyboard[2]
	if last[0].Text != "确认" || last[0].CallbackData != "answer:q_1:confirm" {
		t.Fatalf("confirm button = %#v", last[0])
	}
	if last[1].Text != "忽略" {
		t.Fatalf("dismiss button = %#v", last[1])
	}

	// Toggled option gets the ✅ mark.
	conn.pendingSelect["q_1"] = map[int]bool{1: true}
	keyboard, _ = conn.eventKeyboard(ev)
	if keyboard.InlineKeyboard[1][0].Text != "✅ 选项B" {
		t.Fatalf("selected option = %q, want ✅ mark", keyboard.InlineKeyboard[1][0].Text)
	}
	if keyboard.InlineKeyboard[0][0].Text != "选项A" {
		t.Fatalf("unselected option = %q, want no mark", keyboard.InlineKeyboard[0][0].Text)
	}
}

func TestTerminalizeEditsMessageOnce(t *testing.T) {
	var edits []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		edits = append(edits, body)
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	client := newAPIClient("token")
	client.base = server.URL

	conn := New(nil)
	conn.pendingMsgs["apr_1"] = msgRef{chatID: 42, messageID: 7, text: "<b>需要审批</b>"}
	conn.pendingSelect["apr_1"] = map[int]bool{0: true}

	conn.terminalize(context.Background(), client, "apr_1", "已批准")
	if len(edits) != 1 {
		t.Fatalf("edits = %d, want 1", len(edits))
	}
	text, _ := edits[0]["text"].(string)
	if !strings.Contains(text, "需要审批") || !strings.Contains(text, "已批准") {
		t.Fatalf("terminal text = %q", text)
	}
	markup, _ := edits[0]["reply_markup"].(map[string]any)
	if rows, _ := markup["inline_keyboard"].([]any); len(rows) != 0 {
		t.Fatalf("keyboard not stripped: %#v", markup)
	}
	if _, ok := conn.pendingSelect["apr_1"]; ok {
		t.Fatal("selection state should be cleared")
	}

	// Idempotent: second call is a no-op (no further edits).
	conn.terminalize(context.Background(), client, "apr_1", "已批准")
	if len(edits) != 1 {
		t.Fatalf("edits after repeat = %d, want 1", len(edits))
	}
}

func TestIsStaleRequest(t *testing.T) {
	stale := []string{
		"question q_1 is not pending",
		"approval already resolved",
		"approval not found",
	}
	for _, msg := range stale {
		if !isStaleRequest(fmt.Errorf("%s", msg)) {
			t.Fatalf("%q should be stale", msg)
		}
	}
	if isStaleRequest(fmt.Errorf("network timeout")) {
		t.Fatal("network error is not stale")
	}
	if isStaleRequest(nil) {
		t.Fatal("nil is not stale")
	}
}
