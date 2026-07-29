package core

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

// DefaultPairingTTL is how long a pairing nonce stays valid.
const DefaultPairingTTL = 5 * time.Minute

// NewNonce generates a URL-safe random pairing code of the given byte size
// (telegram uses 18, feishu uses 9).
func NewNonce(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Pairing is the nonce state machine of an in-flight pairing request.
// Channels embed it in their config/profile structs; its JSON field names
// match the historical telegram format (anonymous embedding flattens them).
type Pairing struct {
	Nonce   string `json:"pairing_nonce,omitempty"`
	Expires int64  `json:"pairing_expires,omitempty"`
}

// Valid reports whether code matches an in-flight, unexpired pairing.
func (p Pairing) Valid(code string, now int64) bool {
	return p.Nonce != "" && code == p.Nonce && now <= p.Expires
}

// Start records a fresh nonce with its expiry.
func (p *Pairing) Start(nonce string, now time.Time, ttl time.Duration) {
	p.Nonce = nonce
	p.Expires = now.Add(ttl).Unix()
}

// Clear drops the pairing state (after success or explicit unpair).
func (p *Pairing) Clear() {
	p.Nonce = ""
	p.Expires = 0
}

// Owner is the paired user of a connector. The ID type varies by channel:
// telegram uses int64 user/chat IDs, feishu (and QQ) use strings.
//
// The JSON tags match the historical telegram device format; channels with
// pre-existing divergent formats migrate to this layout.
type Owner[ID comparable] struct {
	UserID   ID     `json:"user_id"`
	Username string `json:"username,omitempty"`
	ChatID   ID     `json:"chat_id"`
	PairedAt int64  `json:"paired_at"`
	LastSeen int64  `json:"last_seen"`
}

// IsZero reports whether the owner carries a usable identity.
func (o *Owner[ID]) IsZero() bool {
	var zero ID
	return o == nil || o.UserID == zero
}

// Matches reports whether the owner is the given user.
func (o *Owner[ID]) Matches(id ID) bool {
	return !o.IsZero() && o.UserID == id
}

// Touch refreshes the owner's username/chat/last-seen if id matches;
// otherwise it is a no-op.
func (o *Owner[ID]) Touch(id ID, username string, chatID ID, now time.Time) {
	if !o.Matches(id) {
		return
	}
	o.Username = username
	o.ChatID = chatID
	o.LastSeen = now.Unix()
}

// SetOwner binds (or re-binds) the owner: an existing owner with the same
// ID is updated in place, otherwise a fresh owner is created.
func SetOwner[ID comparable](owner **Owner[ID], id ID, username string, chatID ID, now time.Time) {
	if *owner != nil && (*owner).UserID == id {
		(*owner).Username = username
		(*owner).ChatID = chatID
		(*owner).LastSeen = now.Unix()
		return
	}
	*owner = &Owner[ID]{
		UserID:   id,
		Username: username,
		ChatID:   chatID,
		PairedAt: now.Unix(),
		LastSeen: now.Unix(),
	}
}
