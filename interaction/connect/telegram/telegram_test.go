package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nekocode/interaction/connect"
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
			Pairing:     connect.Pairing{Nonce: "nonce", Expires: 10},
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

// The action set itself is covered by connect's Translate tests; here we
// only assert the action → inline-button mapping (labels, callback format).
func TestApprovalKeyboard(t *testing.T) {
	in := connect.Intent{
		ID: "apr_1",
		Actions: []connect.Action{
			{ID: connect.ActionOnce, Label: "批准一次"},
			{ID: connect.ActionAlways, Label: "始终允许"},
			{ID: connect.ActionReject, Label: "拒绝"},
		},
	}
	row := approvalKeyboard(in).InlineKeyboard[0]
	wantData := []string{"once:apr_1", "always:apr_1", "reject:apr_1"}
	if len(row) != len(wantData) {
		t.Fatalf("buttons = %d, want %d", len(row), len(wantData))
	}
	for i, btn := range row {
		if btn.Text != in.Actions[i].Label || btn.CallbackData != wantData[i] {
			t.Fatalf("button %d = %q %q, want %q %q", i, btn.Text, btn.CallbackData, in.Actions[i].Label, wantData[i])
		}
	}
}

func questionIntent(view controlruntime.QuestionView) connect.Intent {
	return connect.Intent{ID: view.ID, Question: &view, Actions: connect.QuestionActions(view)}
}

func TestQuestionKeyboard(t *testing.T) {
	keyboard, ok := New(nil).questionKeyboard(questionIntent(controlruntime.QuestionView{
		ID: "q_1",
		Questions: []controlruntime.QuestionItem{{
			Question: "Proceed?",
			Options:  []controlruntime.QuestionOption{{Label: "Yes"}},
		}},
	}))
	if !ok || keyboard.InlineKeyboard[0][0].CallbackData != "answer:q_1:0" {
		t.Fatalf("question keyboard = %#v ok=%v", keyboard, ok)
	}
	dismissRow := keyboard.InlineKeyboard[len(keyboard.InlineKeyboard)-1]
	if dismissRow[0].Text != "忽略" || dismissRow[0].CallbackData != "dismiss:q_1" {
		t.Fatalf("dismiss button = %q %q", dismissRow[0].Text, dismissRow[0].CallbackData)
	}

	// Free-form questions get no keyboard.
	if _, ok := New(nil).questionKeyboard(questionIntent(controlruntime.QuestionView{
		ID:        "q_2",
		Questions: []controlruntime.QuestionItem{{Question: "Why?"}},
	})); ok {
		t.Fatal("free-form question should not have a keyboard")
	}
}

func TestQuestionKeyboardMultiSelect(t *testing.T) {
	view := controlruntime.QuestionView{
		ID: "q_1",
		Questions: []controlruntime.QuestionItem{{
			Multiple: true,
			Options: []controlruntime.QuestionOption{
				{Label: "选项A"}, {Label: "选项B"},
			},
		}},
	}

	conn := New(nil)
	keyboard, ok := conn.questionKeyboard(questionIntent(view))
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
	keyboard, _ = conn.questionKeyboard(questionIntent(view))
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

// TestEventSinkApprovalAndQuestionFlow drives the sink with translated
// intents: interactive messages are sent with keyboards and recorded, the
// question tracker is fed, and resolved intents terminalize in place.
func TestEventSinkApprovalAndQuestionFlow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := saveConfig(Config{
		ActiveProfile: "personal",
		Profiles: []BotProfile{{
			Name:  "personal",
			Owner: &Device{UserID: 1, Username: "alice", ChatID: 42},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	var calls []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		body["_endpoint"] = r.URL.Path
		calls = append(calls, body)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":7}}`))
	}))
	defer server.Close()

	client := newAPIClient("token")
	client.base = server.URL

	conn := New(nil)
	sink := newEventSink(conn, client)
	if caps := sink.Caps(); !caps.EditMessages || !caps.Buttons {
		t.Fatalf("caps = %#v, want edit+buttons", caps)
	}
	if sink.FlushInterval() != previewEditInterval/2 {
		t.Fatalf("flush interval = %v", sink.FlushInterval())
	}
	ctx := context.Background()

	// Approval intent: interactive message sent and recorded.
	view := controlruntime.ApprovalView{ID: "apr_1", ToolName: "shell"}
	for _, in := range connect.Translate(controlruntime.Event{Type: controlruntime.EventApprovalRequested, Payload: view}) {
		if err := sink.Post(ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := conn.pendingMsgs["apr_1"]; !ok {
		t.Fatal("approval message not recorded in pendingMsgs")
	}

	// Resolved on another surface: the message is terminalized in place.
	resolved := view
	resolved.Status = controlruntime.ApprovalApproved
	for _, in := range connect.Translate(controlruntime.Event{Type: controlruntime.EventApprovalResolved, Payload: resolved}) {
		if err := sink.Post(ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := conn.pendingMsgs["apr_1"]; ok {
		t.Fatal("resolved approval should drop pendingMsgs entry")
	}

	// Question intents feed the shared question tracker; resolved removes.
	qview := controlruntime.QuestionView{
		ID: "q_1",
		Questions: []controlruntime.QuestionItem{{
			Question: "Proceed?",
			Options:  []controlruntime.QuestionOption{{Label: "Yes"}, {Label: "No"}},
		}},
	}
	for _, in := range connect.Translate(controlruntime.Event{Type: controlruntime.EventQuestionRequested, Payload: qview}) {
		if err := sink.Post(ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	if conn.questions.LastID() != "q_1" {
		t.Fatalf("last question = %q, want q_1", conn.questions.LastID())
	}
	if _, ok := conn.pendingMsgs["q_1"]; !ok {
		t.Fatal("question message not recorded in pendingMsgs")
	}
	for _, in := range connect.Translate(controlruntime.Event{Type: controlruntime.EventQuestionResolved, Payload: qview}) {
		if err := sink.Post(ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	if conn.questions.LastID() != "" {
		t.Fatal("resolved question should leave the tracker")
	}

	// Traffic: approval send + approval edit + question send + question edit.
	if len(calls) != 4 {
		t.Fatalf("api calls = %d, want 4", len(calls))
	}
	for i, endpoint := range []string{"sendMessage", "editMessageText", "sendMessage", "editMessageText"} {
		if !strings.HasSuffix(calls[i]["_endpoint"].(string), endpoint) {
			t.Fatalf("call %d endpoint = %v, want %s", i, calls[i]["_endpoint"], endpoint)
		}
	}
	// The approval send carries the canonical action keyboard.
	markup, _ := calls[0]["reply_markup"].(map[string]any)
	rows, _ := markup["inline_keyboard"].([]any)
	first, _ := rows[0].([]any)
	btn, _ := first[0].(map[string]any)
	if btn["callback_data"] != "once:apr_1" {
		t.Fatalf("first approval button = %#v", btn)
	}
	// The terminalized edit appends the verdict and strips the keyboard.
	text, _ := calls[1]["text"].(string)
	if !strings.Contains(text, "已批准") {
		t.Fatalf("terminalized text = %q", text)
	}
}

// TestEventSinkResultFallsBackWhenRunUnknown is a regression test: a result
// intent for a run the tracker never saw (race between parallel
// subscriptions, or a run started before the connector connected) must
// still deliver the raw result — never drop the message silently.
func TestEventSinkResultFallsBackWhenRunUnknown(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := saveConfig(Config{
		ActiveProfile: "personal",
		Profiles: []BotProfile{{
			Name:  "personal",
			Owner: &Device{UserID: 1, Username: "alice", ChatID: 42},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	var calls []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		calls = append(calls, body)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":7}}`))
	}))
	defer server.Close()

	client := newAPIClient("token")
	client.base = server.URL
	sink := newEventSink(New(nil), client)

	// The run was never tracked (no Track calls) — DoneReply would be "".
	for _, in := range connect.Translate(controlruntime.Event{
		Type:    controlruntime.EventRunDone,
		RunID:   "run_unknown",
		Payload: controlruntime.RunResult{Output: "final answer"},
	}) {
		if err := sink.Post(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	}
	if len(calls) != 1 {
		t.Fatalf("api calls = %d, want 1 (result must be delivered)", len(calls))
	}
	text, _ := calls[0]["text"].(string)
	if !strings.Contains(text, "final answer") {
		t.Fatalf("result text = %q", text)
	}
}
