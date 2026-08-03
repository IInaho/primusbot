package telegram

import (
	"os"
	"path/filepath"
	"testing"
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
