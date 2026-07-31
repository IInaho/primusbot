package feishu

import (
	"fmt"
	"strings"
	"time"

	"nekocode/interaction/connect"
)

const (
	section    = "feishu"
	pairingTTL = connect.DefaultPairingTTL
)

// Config is the feishu connector configuration: a single app (app_id +
// app_secret) paired with one owner (DM-only MVP, no multi-profile).
// The pairing state machine and owner lifecycle come from the shared
// connector connect.
type Config struct {
	AppID     string                 `json:"app_id,omitempty"`
	AppSecret string                 `json:"app_secret,omitempty"`
	Owner     *connect.Owner[string] `json:"owner,omitempty"`
	connect.Pairing
}

func loadConfig() (Config, error) {
	var cfg Config
	err := connect.DefaultFileStore().Load(section, &cfg)
	return cfg, err
}

func saveConfig(cfg Config) error {
	return connect.DefaultFileStore().Save(section, cfg)
}

func (c Config) configured() bool {
	return strings.TrimSpace(c.AppID) != "" && strings.TrimSpace(c.AppSecret) != ""
}

func (c Config) isAllowed(openID string) bool {
	return c.Owner.Matches(openID)
}

// finishPairing binds the owner and clears the pairing state.
func (c *Config) finishPairing(openID, chatID string) {
	connect.SetOwner(&c.Owner, openID, "", chatID, time.Now())
	c.Clear()
}

func (c *Config) unpair() {
	c.Owner = nil
	c.Clear()
}

func (c *Config) touchOwner(openID, chatID string) {
	c.Owner.Touch(openID, "", chatID, time.Now())
}

func (c Config) pairedChatIDs() []string {
	if c.Owner == nil || c.Owner.ChatID == "" {
		return nil
	}
	return []string{c.Owner.ChatID}
}

func setupInstructions() string {
	return fmt.Sprintf(`Feishu is not configured.

1. Create an app at https://open.feishu.cn (enable the bot capability).
2. Subscribe to the im.message.receive_v1 event and enable long-connection mode.
3. Paste the credentials here:

/connect feishu add <app-id> <app-secret>

Config path: %s`, connect.DefaultFileStore().Path())
}
