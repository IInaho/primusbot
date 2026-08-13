package wecom

import (
	"fmt"
	"strings"
	"time"

	"nekocode/interaction/connect"
)

const (
	section    = "wecom"
	pairingTTL = connect.DefaultPairingTTL
)

// Config is the persisted configuration for one WeCom intelligent robot.
type Config struct {
	BotID  string                 `json:"bot_id,omitempty"`
	Secret string                 `json:"secret,omitempty"`
	Owner  *connect.Owner[string] `json:"owner,omitempty"`
	connect.Pairing
}

func (c Config) configured() bool {
	return strings.TrimSpace(c.BotID) != "" && strings.TrimSpace(c.Secret) != ""
}

func (c Config) isAllowed(userID string) bool { return c.Owner.Matches(userID) }

func (c *Config) finishPairing(userID, chatID string) {
	connect.SetOwner(&c.Owner, userID, "", chatID, time.Now())
	c.Clear()
}

func (c *Config) unpair() {
	c.Owner = nil
	c.Clear()
}

func (c *Config) touchOwner(userID, chatID string) {
	c.Owner.Touch(userID, "", chatID, time.Now())
}

func loadConfig() (Config, error) {
	var cfg Config
	if err := connect.DefaultFileStore().Load(section, &cfg); err != nil {
		return Config{}, err
	}
	cfg.BotID = strings.TrimSpace(cfg.BotID)
	cfg.Secret = strings.TrimSpace(cfg.Secret)
	return cfg, nil
}

func saveConfig(cfg Config) error {
	return connect.DefaultFileStore().Save(section, cfg)
}

func setupInstructions() string {
	return fmt.Sprintf(`WeCom is not configured.

1. 在企业微信管理后台创建“智能机器人”，选择 API 模式和长连接。
2. 获取 Bot ID 和 Secret。
3. 在这里配置：

/connect wecom add <bot-id> <secret>

Config path: %s`, connect.DefaultFileStore().Path())
}
