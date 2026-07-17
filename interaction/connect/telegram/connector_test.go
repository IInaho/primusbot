package telegram

import (
	"context"
	"strings"
	"testing"

	controlruntime "nekocode/runtime"
	"nekocode/runtime/view"
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
			Name:           "personal",
			BotUsername:    "my_bot",
			Owner:          &Device{UserID: 1, Username: "alice"},
			PairingNonce:   "nonce",
			PairingExpires: 10,
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
	if p.Owner != nil || p.PairingNonce != "" || p.PairingExpires != 0 {
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
	approvalKeyboard, ok := eventKeyboard(controlruntime.Event{
		Type: controlruntime.EventApprovalRequested,
		Payload: controlruntime.ApprovalView{
			ID: "apr_1",
		},
	})
	if !ok || approvalKeyboard.InlineKeyboard[0][0].CallbackData != "approve:apr_1" {
		t.Fatalf("approval keyboard = %#v ok=%v", approvalKeyboard, ok)
	}

	questionKeyboard, ok := eventKeyboard(controlruntime.Event{
		Type: controlruntime.EventQuestionRequested,
		Payload: controlruntime.QuestionView{
			ID: "q_1",
			Questions: []view.QuestionItem{{
				Question: "Proceed?",
				Options:  []view.QuestionOption{{Label: "Yes"}},
			}},
		},
	})
	if !ok || questionKeyboard.InlineKeyboard[0][0].CallbackData != "answer:q_1:0" {
		t.Fatalf("question keyboard = %#v ok=%v", questionKeyboard, ok)
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
