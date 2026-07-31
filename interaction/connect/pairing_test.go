package connect

import (
	"testing"
	"time"
)

func TestPairingLifecycle(t *testing.T) {
	now := time.Now()
	var p Pairing
	p.Start("nonce-1", now, DefaultPairingTTL)

	if !p.Valid("nonce-1", now.Unix()) {
		t.Fatal("fresh nonce should be valid")
	}
	if p.Valid("wrong", now.Unix()) {
		t.Fatal("mismatched nonce should be invalid")
	}
	if p.Valid("nonce-1", now.Add(DefaultPairingTTL+time.Second).Unix()) {
		t.Fatal("expired nonce should be invalid")
	}
	if (Pairing{}).Valid("", now.Unix()) {
		t.Fatal("empty pairing state should never validate")
	}

	p.Clear()
	if p.Valid("nonce-1", now.Unix()) {
		t.Fatal("cleared pairing should not validate")
	}
}

func TestNewNonce(t *testing.T) {
	a, err := NewNonce(9)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewNonce(18)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("nonces should be random")
	}
	if len(b) <= len(a) {
		t.Fatalf("18-byte nonce should be longer than 9-byte: %q vs %q", b, a)
	}
}

func TestOwnerLifecycleInt64(t *testing.T) {
	var owner *Owner[int64]
	now := time.Now()
	SetOwner(&owner, 42, "alice", 7, now)
	if !owner.Matches(42) || owner.Matches(43) {
		t.Fatal("matches broken")
	}
	if owner.ChatID != 7 || owner.PairedAt != now.Unix() {
		t.Fatalf("owner = %+v", owner)
	}

	// Re-bind same ID updates in place, keeping PairedAt.
	SetOwner(&owner, 42, "alice2", 8, now.Add(time.Minute))
	if owner.Username != "alice2" || owner.ChatID != 8 || owner.PairedAt != now.Unix() {
		t.Fatalf("rebind = %+v", owner)
	}

	// Touch from a non-owner is a no-op; from the owner it refreshes.
	owner.Touch(43, "mallory", 9, now)
	if owner.ChatID != 8 {
		t.Fatal("touch from non-owner must be ignored")
	}
	owner.Touch(42, "alice3", 10, now)
	if owner.ChatID != 10 || owner.Username != "alice3" {
		t.Fatalf("touch = %+v", owner)
	}
}

func TestOwnerStringIDs(t *testing.T) {
	var owner *Owner[string]
	SetOwner(&owner, "ou_1", "", "oc_1", time.Now())
	if !owner.Matches("ou_1") || owner.Matches("") || owner.IsZero() {
		t.Fatalf("string owner = %+v", owner)
	}
	var nilOwner *Owner[string]
	if !nilOwner.IsZero() || nilOwner.Matches("ou_1") {
		t.Fatal("nil owner should be zero and match nothing")
	}
}
