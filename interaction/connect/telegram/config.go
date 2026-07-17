package telegram

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nekocode/util/fs"
)

const pairingTTL = 5 * time.Minute

type configFile struct {
	Telegram Config `json:"telegram"`
}

type Config struct {
	ActiveProfile string       `json:"active_profile,omitempty"`
	Profiles      []BotProfile `json:"profiles,omitempty"`

	// Legacy single-bot fields. loadConfig migrates these into Profiles and
	// saveConfig writes only the profile format.
	BotToken       string   `json:"bot_token,omitempty"`
	BotUsername    string   `json:"bot_username,omitempty"`
	AllowedUsers   []int64  `json:"allowed_users,omitempty"`
	Devices        []Device `json:"devices,omitempty"`
	UpdateOffset   int      `json:"update_offset,omitempty"`
	PairingNonce   string   `json:"pairing_nonce,omitempty"`
	PairingExpires int64    `json:"pairing_expires,omitempty"`
}

type BotProfile struct {
	Name           string  `json:"name"`
	BotToken       string  `json:"bot_token,omitempty"`
	BotUsername    string  `json:"bot_username,omitempty"`
	Owner          *Device `json:"owner,omitempty"`
	UpdateOffset   int     `json:"update_offset,omitempty"`
	PairingNonce   string  `json:"pairing_nonce,omitempty"`
	PairingExpires int64   `json:"pairing_expires,omitempty"`
}

type Device struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username,omitempty"`
	ChatID   int64  `json:"chat_id"`
	PairedAt int64  `json:"paired_at"`
	LastSeen int64  `json:"last_seen"`
}

func configPath() string {
	return filepath.Join(fs.NekocodeHome(), "connect.json")
}

func loadConfig() (Config, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var file configFile
	if err := json.Unmarshal(data, &file); err != nil {
		return Config{}, err
	}
	cfg := file.Telegram
	cfg.normalize()
	return cfg, nil
}

func saveConfig(cfg Config) error {
	cfg.normalize()
	cfg.clearLegacy()
	file := configFile{Telegram: cfg}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return fs.WriteFileWithDir(configPath(), data, 0o600)
}

func (c *Config) normalize() {
	if len(c.Profiles) == 0 && strings.TrimSpace(c.BotToken) != "" {
		p := BotProfile{
			Name:           "default",
			BotToken:       strings.TrimSpace(c.BotToken),
			BotUsername:    strings.TrimSpace(c.BotUsername),
			UpdateOffset:   c.UpdateOffset,
			PairingNonce:   c.PairingNonce,
			PairingExpires: c.PairingExpires,
		}
		if len(c.Devices) > 0 {
			owner := c.Devices[0]
			p.Owner = &owner
		} else if len(c.AllowedUsers) > 0 {
			p.Owner = &Device{UserID: c.AllowedUsers[0]}
		}
		c.Profiles = append(c.Profiles, p)
	}
	for i := range c.Profiles {
		c.Profiles[i].Name = normalizeProfileName(c.Profiles[i].Name)
		if c.Profiles[i].Name == "" {
			c.Profiles[i].Name = fmt.Sprintf("profile-%d", i+1)
		}
		c.Profiles[i].BotToken = strings.TrimSpace(c.Profiles[i].BotToken)
		c.Profiles[i].BotUsername = strings.TrimPrefix(strings.TrimSpace(c.Profiles[i].BotUsername), "@")
	}
	c.ActiveProfile = normalizeProfileName(c.ActiveProfile)
	if c.ActiveProfile == "" && len(c.Profiles) > 0 {
		c.ActiveProfile = c.Profiles[0].Name
	}
	if c.ActiveProfile != "" && c.profileIndex(c.ActiveProfile) < 0 && len(c.Profiles) > 0 {
		c.ActiveProfile = c.Profiles[0].Name
	}
	c.clearLegacy()
}

func (c *Config) clearLegacy() {
	c.BotToken = ""
	c.BotUsername = ""
	c.AllowedUsers = nil
	c.Devices = nil
	c.UpdateOffset = 0
	c.PairingNonce = ""
	c.PairingExpires = 0
}

func normalizeProfileName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimPrefix(name, "@")
	name = strings.TrimSpace(name)
	return name
}

func (c Config) profileIndex(name string) int {
	name = normalizeProfileName(name)
	for i := range c.Profiles {
		if normalizeProfileName(c.Profiles[i].Name) == name {
			return i
		}
	}
	return -1
}

func (c Config) activeIndex() int {
	return c.profileIndex(c.ActiveProfile)
}

func (c Config) activeProfile() (BotProfile, bool) {
	idx := c.activeIndex()
	if idx < 0 {
		return BotProfile{}, false
	}
	return c.Profiles[idx], true
}

func (c *Config) activeProfileRef() (*BotProfile, bool) {
	idx := c.activeIndex()
	if idx < 0 {
		return nil, false
	}
	return &c.Profiles[idx], true
}

func newPairingNonce() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (p BotProfile) pairedChatIDs() []int64 {
	if p.Owner == nil || p.Owner.ChatID == 0 {
		return nil
	}
	return []int64{p.Owner.ChatID}
}

func (p BotProfile) isAllowed(userID int64) bool {
	return p.Owner != nil && p.Owner.UserID != 0 && p.Owner.UserID == userID
}

func (p *BotProfile) setOwner(userID int64, username string, chatID int64) {
	now := time.Now().Unix()
	if p.Owner != nil && p.Owner.UserID == userID {
		p.Owner.Username = username
		p.Owner.ChatID = chatID
		p.Owner.LastSeen = now
		return
	}
	p.Owner = &Device{
		UserID:   userID,
		Username: username,
		ChatID:   chatID,
		PairedAt: now,
		LastSeen: now,
	}
}

func (p *BotProfile) touchOwner(userID int64, username string, chatID int64) {
	if p.Owner == nil || p.Owner.UserID != userID {
		return
	}
	p.Owner.Username = username
	p.Owner.ChatID = chatID
	p.Owner.LastSeen = time.Now().Unix()
}

func setupInstructions() string {
	return fmt.Sprintf(`Telegram is not configured.

1. Open @BotFather in Telegram.
2. Create a bot with /newbot.
3. Paste the token here:

/connect telegram add <bot-token>

Config path: %s`, configPath())
}
