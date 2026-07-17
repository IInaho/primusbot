package telegram

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nekocode/interaction/connect/telegram/internal/taskview"
)

func TestProfileAllowsOnlyOwner(t *testing.T) {
	profile := BotProfile{Owner: &Device{UserID: 2, ChatID: 20}}
	if !profile.isAllowed(2) {
		t.Fatal("owner should be allowed")
	}
	if profile.isAllowed(3) {
		t.Fatal("unknown user should not be allowed")
	}
}

func TestSetOwnerUpdatesExisting(t *testing.T) {
	profile := BotProfile{}
	profile.setOwner(1, "alice", 10)
	profile.setOwner(1, "alice2", 11)
	if profile.Owner == nil {
		t.Fatal("owner not set")
	}
	if profile.Owner.Username != "alice2" || profile.Owner.ChatID != 11 {
		t.Fatalf("owner not updated: %#v", profile.Owner)
	}
}

func TestLoadConfigMigratesLegacySingleBot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".nekocode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{
  "telegram": {
    "bot_token": "123:secret",
    "bot_username": "old_bot",
    "devices": [{"user_id": 9, "username": "alice", "chat_id": 90}],
    "update_offset": 42
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "connect.json"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveProfile != "default" || len(cfg.Profiles) != 1 {
		t.Fatalf("profiles not migrated: %#v", cfg)
	}
	p := cfg.Profiles[0]
	if p.BotToken != "123:secret" || p.BotUsername != "old_bot" || p.UpdateOffset != 42 {
		t.Fatalf("profile fields not migrated: %#v", p)
	}
	if p.Owner == nil || p.Owner.UserID != 9 || p.Owner.ChatID != 90 {
		t.Fatalf("owner not migrated: %#v", p.Owner)
	}
}

func TestTruncateRunes(t *testing.T) {
	got := taskview.TruncateRunes("你好世界", 2)
	if got != "你好..." {
		t.Fatalf("truncateRunes = %q", got)
	}
}

func TestTerminalQR(t *testing.T) {
	got, err := terminalQR("https://t.me/example_bot?start=test")
	if err != nil {
		t.Fatalf("terminalQR: %v", err)
	}
	if got == "" {
		t.Fatal("terminalQR returned empty output")
	}
	if strings.HasSuffix(got, "\n") {
		t.Fatal("terminalQR should not add a trailing blank line")
	}
	if strings.Contains(got, "\n\n") {
		t.Fatal("terminalQR should not contain empty rows")
	}
	if !strings.Contains(got, "\x1b[47m") || !strings.Contains(got, "\x1b[40m") {
		t.Fatal("terminalQR should render explicit white and black backgrounds")
	}
}
