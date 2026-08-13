package wecom

import "testing"

func TestConfigPairingLifecycle(t *testing.T) {
	cfg := Config{BotID: "bot", Secret: "secret"}
	if !cfg.configured() {
		t.Fatal("bot id and secret should configure connector")
	}
	cfg.finishPairing("zhangsan", "group-1")
	if !cfg.isAllowed("zhangsan") || cfg.isAllowed("lisi") {
		t.Fatal("only the paired owner should be allowed")
	}
	if cfg.Owner.ChatID != "group-1" || cfg.Pairing.Nonce != "" {
		t.Fatalf("paired config = %#v", cfg)
	}
	cfg.touchOwner("zhangsan", "zhangsan")
	if cfg.Owner.ChatID != "zhangsan" {
		t.Fatal("the latest owner conversation should receive outbound events")
	}
	cfg.unpair()
	if cfg.Owner != nil || cfg.isAllowed("zhangsan") {
		t.Fatal("unpair should clear owner access")
	}
}

func TestConfigRequiresCompleteCredentials(t *testing.T) {
	for _, cfg := range []Config{{}, {BotID: "bot"}, {Secret: "secret"}, {BotID: " ", Secret: "secret"}} {
		if cfg.configured() {
			t.Fatalf("incomplete config reported configured: %#v", cfg)
		}
	}
}
