package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nekocode/interaction/connect/core"
)

func (c *Connector) connectActive(ctx context.Context) (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	if len(cfg.Profiles) == 0 {
		return setupInstructions(), nil
	}
	profile, ok := cfg.activeProfile()
	if !ok || strings.TrimSpace(profile.BotToken) == "" {
		return setupInstructions(), nil
	}
	if profile.Owner == nil {
		return c.pairProfile(ctx, []string{profile.Name})
	}
	if err := c.Start(ctx); err != nil {
		return "", err
	}
	return fmt.Sprintf("Telegram connected.\n\nBot: %s\nOwner: %s", profileLabel(profile), ownerLabel(profile.Owner)), nil
}

func (c *Connector) pairProfile(ctx context.Context, args []string) (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	if len(cfg.Profiles) == 0 {
		return setupInstructions(), nil
	}
	name := cfg.ActiveProfile
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		name = args[0]
	}
	idx := cfg.profileIndex(name)
	if idx < 0 {
		return "", fmt.Errorf("telegram profile %q not found", name)
	}
	cfg.ActiveProfile = cfg.Profiles[idx].Name
	profile := &cfg.Profiles[idx]
	if strings.TrimSpace(profile.BotToken) == "" {
		return setupInstructions(), nil
	}
	if profile.Owner != nil {
		return fmt.Sprintf("%s is already paired with %s.\nRun /connect telegram unpair %s before pairing another account.", profileLabel(*profile), ownerLabel(profile.Owner), profile.Name), nil
	}
	client := newAPIClient(profile.BotToken)
	if profile.BotUsername == "" {
		me, err := botMe(ctx, client)
		if err != nil {
			return "", err
		}
		profile.BotUsername = me.Username
	}
	nonce, err := core.NewNonce(18)
	if err != nil {
		return "", err
	}
	profile.Nonce = nonce
	profile.Expires = time.Now().Add(pairingTTL).Unix()
	if err := saveConfig(cfg); err != nil {
		return "", err
	}
	if err := c.Start(ctx); err != nil {
		return "", err
	}
	link := fmt.Sprintf("https://t.me/%s?start=%s", profile.BotUsername, nonce)
	qr, err := terminalQR(link)
	if err != nil {
		qr = ""
	}
	var out strings.Builder
	out.WriteString("Telegram pairing started.\n")
	out.WriteString("Profile: ")
	out.WriteString(profileLabel(*profile))
	out.WriteString("\n\n")
	out.WriteString("Open this link on your phone, or scan the QR code:\n")
	out.WriteString(link)
	if qr != "" {
		out.WriteString("\n\n")
		out.WriteString(qr)
	}
	fmt.Fprintf(&out, "\n\nPairing expires in %d minutes.\n", int(pairingTTL/time.Minute))
	out.WriteString("After pairing, Telegram messages will be routed into this NekoCode session.")
	return out.String(), nil
}

func (c *Connector) addProfile(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return usageAddProfile, nil
	}
	name := ""
	token := strings.TrimSpace(args[0])
	if len(args) >= 2 {
		name = normalizeProfileName(args[0])
		token = strings.TrimSpace(args[1])
	}
	if token == "" {
		return usageAddProfile, nil
	}
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	client := newAPIClient(token)
	me, err := botMe(ctx, client)
	if err != nil {
		return "", err
	}
	if name == "" {
		name = profileNameFromBotUsername(me.Username)
	}
	idx := cfg.profileIndex(name)
	if idx < 0 {
		cfg.Profiles = append(cfg.Profiles, BotProfile{Name: name})
		idx = len(cfg.Profiles) - 1
	}
	cfg.Profiles[idx].Name = name
	cfg.Profiles[idx].BotToken = token
	cfg.Profiles[idx].BotUsername = me.Username
	if cfg.ActiveProfile == "" || len(cfg.Profiles) == 1 {
		cfg.ActiveProfile = name
	}
	if err := saveConfig(cfg); err != nil {
		return "", err
	}
	return fmt.Sprintf("Telegram bot saved: %s\nRun /connect telegram to connect.", profileLabel(cfg.Profiles[idx])), nil
}

// botMe calls getMe with a fixed timeout so a hung Telegram API cannot block
// command handling.
func botMe(ctx context.Context, client *apiClient) (User, error) {
	getCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return client.getMe(getCtx)
}

func (c *Connector) status() (string, error) {
	return c.profileReport(true)
}

func (c *Connector) profiles() (string, error) {
	return c.profileReport(false)
}

func (c *Connector) profileReport(withStatus bool) (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	running := c.base.IsRunning()
	if len(cfg.Profiles) == 0 {
		if withStatus {
			return "Telegram is not configured.\n\n" + setupInstructions(), nil
		}
		return setupInstructions(), nil
	}
	if withStatus {
		return fmt.Sprintf("Telegram: running=%v active=%s\n\n%s", running, cfg.ActiveProfile, profilesList(cfg, running)), nil
	}
	return profilesList(cfg, running), nil
}

func (c *Connector) useProfile(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return usageUseProfile, nil
	}
	name := normalizeProfileName(args[0])
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	idx := cfg.profileIndex(name)
	if idx < 0 {
		return "", fmt.Errorf("telegram profile %q not found", name)
	}
	c.mu.Lock()
	active := c.active
	c.mu.Unlock()
	wasRunning := c.base.IsRunning()
	if wasRunning && active != "" && active != cfg.Profiles[idx].Name {
		if err := c.Stop(); err != nil {
			return "", err
		}
	}
	cfg.ActiveProfile = cfg.Profiles[idx].Name
	if err := saveConfig(cfg); err != nil {
		return "", err
	}
	profile := cfg.Profiles[idx]
	if profile.Owner == nil {
		return fmt.Sprintf("Active Telegram profile set to %s.\nRun /connect telegram pair to bind your Telegram account.", profileLabel(profile)), nil
	}
	if err := c.Start(ctx); err != nil {
		return "", err
	}
	return fmt.Sprintf("Telegram connected.\n\nActive profile: %s\nOwner: %s", profileLabel(profile), ownerLabel(profile.Owner)), nil
}

func (c *Connector) unpairProfile(args []string) (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	name := cfg.ActiveProfile
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		name = args[0]
	}
	idx := cfg.profileIndex(name)
	if idx < 0 {
		return "", fmt.Errorf("telegram profile %q not found", name)
	}
	cfg.Profiles[idx].Owner = nil
	cfg.Profiles[idx].Nonce = ""
	cfg.Profiles[idx].Expires = 0
	if err := saveConfig(cfg); err != nil {
		return "", err
	}
	return fmt.Sprintf("Telegram profile unpaired: %s", profileLabel(cfg.Profiles[idx])), nil
}

func (c *Connector) removeProfile(args []string) (string, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return usageRemoveProfile, nil
	}
	name := normalizeProfileName(args[0])
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	idx := cfg.profileIndex(name)
	if idx < 0 {
		return "", fmt.Errorf("telegram profile %q not found", name)
	}
	removed := cfg.Profiles[idx]
	if cfg.ActiveProfile == removed.Name {
		wasRunning := c.base.IsRunning()
		if wasRunning {
			if err := c.Stop(); err != nil {
				return "", err
			}
		}
	}
	cfg.Profiles = append(cfg.Profiles[:idx], cfg.Profiles[idx+1:]...)
	if cfg.ActiveProfile == removed.Name {
		cfg.ActiveProfile = ""
		if len(cfg.Profiles) > 0 {
			cfg.ActiveProfile = cfg.Profiles[0].Name
		}
	}
	if err := saveConfig(cfg); err != nil {
		return "", err
	}
	return fmt.Sprintf("Telegram profile removed: %s", profileLabel(removed)), nil
}

func (c *Connector) resetConfig() (string, error) {
	wasRunning := c.base.IsRunning()
	if wasRunning {
		if err := c.Stop(); err != nil {
			return "", err
		}
	}
	if err := saveConfig(Config{}); err != nil {
		return "", err
	}
	return "Telegram configuration reset.", nil
}

func profilesList(cfg Config, running bool) string {
	if len(cfg.Profiles) == 0 {
		return "No Telegram profiles configured."
	}
	lines := []string{"Telegram profiles:"}
	for _, p := range cfg.Profiles {
		marker := " "
		status := "stopped"
		if p.Name == cfg.ActiveProfile {
			marker = "*"
			if running {
				status = "running"
			} else {
				status = "active"
			}
		}
		owner := "unpaired"
		if p.Owner != nil {
			owner = "owner " + ownerLabel(p.Owner)
		}
		lines = append(lines, fmt.Sprintf("%s %s  %s  %s", marker, profileLabel(p), status, owner))
	}
	return strings.Join(lines, "\n")
}

func profileLabel(p BotProfile) string {
	if p.BotUsername != "" {
		return p.Name + " @" + p.BotUsername
	}
	return p.Name
}

func ownerLabel(owner *Device) string {
	if owner == nil {
		return "unpaired"
	}
	if owner.Username != "" {
		return "@" + owner.Username
	}
	if owner.UserID != 0 {
		return strconv.FormatInt(owner.UserID, 10)
	}
	return "unknown"
}

func profileNameFromBotUsername(username string) string {
	name := normalizeProfileName(username)
	name = strings.TrimSuffix(name, "_bot")
	name = strings.TrimSuffix(name, "bot")
	if name == "" {
		return "telegram"
	}
	return name
}
