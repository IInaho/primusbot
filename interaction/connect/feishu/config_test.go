package feishu

import (
	"testing"
	"time"
)

func TestConfigPairingLifecycle(t *testing.T) {
	now := time.Now()
	cfg := Config{}
	cfg.Pairing.Start("nonce-1", now, pairingTTL)

	if !cfg.Pairing.Valid("nonce-1", now.Unix()) {
		t.Fatal("fresh nonce should be valid")
	}
	if cfg.Pairing.Valid("wrong", now.Unix()) {
		t.Fatal("mismatched nonce should be invalid")
	}
	if cfg.Pairing.Valid("nonce-1", now.Add(pairingTTL+time.Second).Unix()) {
		t.Fatal("expired nonce should be invalid")
	}

	cfg.finishPairing("ou_1", "oc_1")
	if cfg.Pairing.Nonce != "" || cfg.Pairing.Expires != 0 {
		t.Fatal("pairing state should be cleared")
	}
	if !cfg.isAllowed("ou_1") {
		t.Fatal("owner open_id should be allowed")
	}
	if cfg.isAllowed("ou_2") {
		t.Fatal("other open_id should not be allowed")
	}
	if got := cfg.pairedChatIDs(); len(got) != 1 || got[0] != "oc_1" {
		t.Fatalf("pairedChatIDs = %v, want [oc_1]", got)
	}
}

func TestUnpairAndTouchOwner(t *testing.T) {
	cfg := Config{}
	cfg.finishPairing("ou_1", "oc_1")

	cfg.touchOwner("ou_2", "oc_2")
	if cfg.Owner.ChatID != "oc_1" {
		t.Fatal("touch from non-owner must be ignored")
	}
	cfg.touchOwner("ou_1", "oc_2")
	if cfg.Owner.ChatID != "oc_2" {
		t.Fatal("owner chat id should be updated")
	}

	cfg.unpair()
	if cfg.Owner != nil || cfg.isAllowed("ou_1") || len(cfg.pairedChatIDs()) != 0 {
		t.Fatal("unpair should clear owner and chats")
	}
}

func TestConfigured(t *testing.T) {
	if (Config{}).configured() {
		t.Fatal("empty config should not be configured")
	}
	if (Config{AppID: "  ", AppSecret: "sec"}).configured() {
		t.Fatal("whitespace-only app id should not be configured")
	}
	if !(Config{AppID: "cli_a", AppSecret: "sec"}).configured() {
		t.Fatal("app id + secret should be configured")
	}
}
