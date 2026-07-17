package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"nekocode/interaction/connect/telegram/internal/taskview"
	controlruntime "nekocode/runtime"
	"nekocode/runtime/view"
)

type Connector struct {
	rt controlruntime.Runtime

	mu          sync.Mutex
	cancel      context.CancelFunc
	running     bool
	active      string
	taskTracker *taskview.Tracker
}

func New(rt controlruntime.Runtime) *Connector {
	return &Connector{rt: rt, taskTracker: taskview.NewTracker()}
}

func (c *Connector) Name() string { return "telegram" }

func (c *Connector) ConnectorStatusView() controlruntime.ConnectorView {
	cfg, err := loadConfig()
	c.mu.Lock()
	running := c.running
	c.mu.Unlock()

	view := controlruntime.ConnectorView{
		Name:        "telegram",
		Registered:  true,
		Initialized: true,
		Running:     running,
		Status:      "stopped",
		Metadata:    make(map[string]any),
	}
	if running {
		view.Status = "running"
	}
	if err != nil {
		view.Status = "error"
		view.Message = err.Error()
		return view
	}
	profile, ok := cfg.activeProfile()
	view.Configured = ok && strings.TrimSpace(profile.BotToken) != ""
	if cfg.ActiveProfile != "" {
		view.Metadata["active_profile"] = cfg.ActiveProfile
	}
	if profile.BotUsername != "" {
		view.Metadata["bot_username"] = profile.BotUsername
	}
	if profile.PairingNonce != "" && time.Now().Unix() <= profile.PairingExpires {
		view.Metadata["pairing_expires"] = profile.PairingExpires
	}
	if profile.Owner != nil {
		device := *profile.Owner
		view.Devices = append(view.Devices, controlruntime.ConnectorDeviceView{
			ID:       strconv.FormatInt(device.UserID, 10),
			Username: device.Username,
			LastSeen: device.LastSeen,
			PairedAt: device.PairedAt,
		})
	}
	if !view.Configured {
		view.Status = "unconfigured"
		view.Message = "Run /connect telegram add <bot-token> first."
	}
	return view
}

func (c *Connector) Start(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	profile, ok := cfg.activeProfile()
	if !ok || strings.TrimSpace(profile.BotToken) == "" {
		return fmt.Errorf("telegram token is not configured")
	}
	c.mu.Lock()
	if c.running && c.active == profile.Name {
		c.mu.Unlock()
		return nil
	}
	if c.running && c.cancel != nil {
		c.cancel()
	}
	client := newAPIClient(profile.BotToken)
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.running = true
	c.active = profile.Name
	c.mu.Unlock()

	c.publishStatus("running", "")
	go c.pollLoop(runCtx, client)
	go c.eventLoop(runCtx, client)
	return nil
}

func (c *Connector) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
	c.cancel = nil
	c.running = false
	c.active = ""
	c.publishStatus("stopped", "Telegram connector stopped.")
	return nil
}

func (c *Connector) HandleCommand(ctx context.Context, args []string) (string, error) {
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "add":
			return c.addProfile(ctx, args[1:])
		case "token":
			return c.configureToken(ctx, args[1:])
		case "profiles", "list":
			return c.profiles()
		case "use":
			return c.useProfile(ctx, args[1:])
		case "pair":
			return c.pairProfile(ctx, args[1:])
		case "unpair":
			return c.unpairProfile(args[1:])
		case "remove", "delete":
			return c.removeProfile(args[1:])
		case "reset":
			return c.resetConfig()
		case "status":
			return c.status()
		case "disconnect", "stop":
			if err := c.Stop(); err != nil {
				return "", err
			}
			return "Telegram connector stopped.", nil
		}
	}
	return c.connectActive(ctx)
}

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
		getCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		me, err := client.getMe(getCtx)
		cancel()
		if err != nil {
			return "", err
		}
		profile.BotUsername = me.Username
	}
	nonce, err := newPairingNonce()
	if err != nil {
		return "", err
	}
	profile.PairingNonce = nonce
	profile.PairingExpires = time.Now().Add(pairingTTL).Unix()
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
	out.WriteString("\n\nPairing expires in 5 minutes.\n")
	out.WriteString("After pairing, Telegram messages will be routed into this NekoCode session.")
	return out.String(), nil
}

func (c *Connector) configureToken(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "Usage: /connect telegram add <bot-token>", nil
	}
	return c.addProfile(ctx, args)
}

func (c *Connector) addProfile(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return "Usage: /connect telegram add <bot-token>", nil
	}
	name := ""
	token := strings.TrimSpace(args[0])
	if len(args) >= 2 {
		name = normalizeProfileName(args[0])
		token = strings.TrimSpace(args[1])
	}
	if token == "" {
		return "Usage: /connect telegram add <bot-token>", nil
	}
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	client := newAPIClient(token)
	getCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	me, err := client.getMe(getCtx)
	cancel()
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

func (c *Connector) status() (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	running := c.running
	c.mu.Unlock()
	if len(cfg.Profiles) == 0 {
		return "Telegram is not configured.\n\n" + setupInstructions(), nil
	}
	return fmt.Sprintf("Telegram: running=%v active=%s\n\n%s", running, cfg.ActiveProfile, profilesList(cfg, running)), nil
}

func (c *Connector) profiles() (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	running := c.running
	c.mu.Unlock()
	if len(cfg.Profiles) == 0 {
		return setupInstructions(), nil
	}
	return profilesList(cfg, running), nil
}

func (c *Connector) useProfile(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return "Usage: /connect telegram use <name>", nil
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
	wasRunning := c.running
	active := c.active
	c.mu.Unlock()
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
	cfg.Profiles[idx].PairingNonce = ""
	cfg.Profiles[idx].PairingExpires = 0
	if err := saveConfig(cfg); err != nil {
		return "", err
	}
	return fmt.Sprintf("Telegram profile unpaired: %s", profileLabel(cfg.Profiles[idx])), nil
}

func (c *Connector) removeProfile(args []string) (string, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return "Usage: /connect telegram remove <name>", nil
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
		c.mu.Lock()
		wasRunning := c.running
		c.mu.Unlock()
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
	c.mu.Lock()
	wasRunning := c.running
	c.mu.Unlock()
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

func (c *Connector) pollLoop(ctx context.Context, client *apiClient) {
	for {
		cfg, err := loadConfig()
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}
		idx := cfg.activeIndex()
		if idx < 0 {
			time.Sleep(3 * time.Second)
			continue
		}
		profile := &cfg.Profiles[idx]
		updates, err := client.getUpdates(ctx, profile.UpdateOffset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			time.Sleep(3 * time.Second)
			continue
		}
		for _, update := range updates {
			if update.UpdateID >= profile.UpdateOffset {
				profile.UpdateOffset = update.UpdateID + 1
			}
			c.handleUpdate(ctx, client, profile, update)
		}
		_ = saveConfig(cfg)
	}
}

func (c *Connector) handleUpdate(ctx context.Context, client *apiClient, profile *BotProfile, update Update) {
	if update.CallbackQuery != nil {
		c.handleCallbackQuery(ctx, client, profile, update.CallbackQuery)
		return
	}
	if update.Message == nil || update.Message.From == nil {
		return
	}
	msg := update.Message
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}
	if nonce, ok := parseStartPayload(text); ok {
		if nonce != "" && nonce == profile.PairingNonce && time.Now().Unix() <= profile.PairingExpires {
			if profile.Owner != nil && profile.Owner.UserID != msg.From.ID {
				_ = client.sendMessage(ctx, msg.Chat.ID, "Pairing failed: this Telegram profile is already paired. Run /connect telegram unpair in NekoCode first.")
				return
			}
			profile.setOwner(msg.From.ID, msg.From.Username, msg.Chat.ID)
			profile.PairingNonce = ""
			profile.PairingExpires = 0
			_ = client.sendMessage(ctx, msg.Chat.ID, "NekoCode connected. Send a message to control the current session.")
			c.publishStatus("paired", fmt.Sprintf("Telegram connected: @%s", msg.From.Username))
			return
		}
		if !profile.isAllowed(msg.From.ID) {
			_ = client.sendMessage(ctx, msg.Chat.ID, pairingFailureMessage(nonce, profile))
			return
		}
	}
	if !profile.isAllowed(msg.From.ID) {
		_ = client.sendMessage(ctx, msg.Chat.ID, "This Telegram account is not connected to NekoCode. Run /connect telegram pair in NekoCode to bind this account.")
		return
	}
	profile.touchOwner(msg.From.ID, msg.From.Username, msg.Chat.ID)

	switch {
	case text == "/stop":
		_ = c.rt.Abort(ctx, "")
		_ = client.sendMessage(ctx, msg.Chat.ID, "Stop requested.")
	case text == "/help":
		_ = client.sendMessage(ctx, msg.Chat.ID, taskview.Help())
	case strings.HasPrefix(text, "/approve "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/approve "))
		err := c.rt.Approve(ctx, id, controlruntime.ApprovalDecision{Allowed: true})
		c.replyErr(ctx, client, msg.Chat.ID, "Approved.", err)
	case strings.HasPrefix(text, "/reject "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/reject "))
		err := c.rt.Approve(ctx, id, controlruntime.ApprovalDecision{})
		c.replyErr(ctx, client, msg.Chat.ID, "Rejected.", err)
	case text == "/answer" || strings.HasPrefix(text, "/answer "):
		id, answer := parseAnswerCommand(text)
		reply, resolvedID, err := c.tracker().BuildQuestionReply(id, answer)
		if err == nil {
			err = c.rt.Answer(ctx, resolvedID, reply)
		}
		c.replyErr(ctx, client, msg.Chat.ID, "Answer sent.", err)
	case text == "/dismiss" || strings.HasPrefix(text, "/dismiss "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/dismiss"))
		resolvedID, err := c.tracker().RejectQuestion(id)
		if err == nil {
			err = c.rt.Answer(ctx, resolvedID, view.QuestionReply{Rejected: true})
		}
		c.replyErr(ctx, client, msg.Chat.ID, "Dismissed.", err)
	case text == "/last":
		_ = client.sendMessage(ctx, msg.Chat.ID, c.tracker().LastSummary())
	case text == "/diff" || strings.HasPrefix(text, "/diff "):
		runID := strings.TrimSpace(strings.TrimPrefix(text, "/diff"))
		_ = client.sendMessage(ctx, msg.Chat.ID, c.tracker().DiffSummary(controlruntime.RunID(runID)))
	case text == "/status":
		_ = client.sendMessage(ctx, msg.Chat.ID, c.tracker().Status())
	default:
		_, err := c.rt.Submit(ctx, controlruntime.Input{
			Kind:   controlruntime.InputMessage,
			Source: controlruntime.SourceRef{Kind: "telegram", ID: strconv.FormatInt(msg.Chat.ID, 10)},
			Sender: controlruntime.SenderRef{
				ID:       strconv.FormatInt(msg.From.ID, 10),
				Username: msg.From.Username,
				Display:  msg.From.FirstName,
			},
			Text: text,
		})
		if err != nil {
			c.replyErr(ctx, client, msg.Chat.ID, "", err)
		}
	}
}

func parseStartPayload(text string) (payload string, ok bool) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return "", false
	}
	cmd := strings.ToLower(fields[0])
	if cmd != "/start" && !strings.HasPrefix(cmd, "/start@") {
		return "", false
	}
	if len(fields) == 1 {
		return "", true
	}
	return strings.TrimSpace(fields[1]), true
}

func pairingFailureMessage(nonce string, profile *BotProfile) string {
	if strings.TrimSpace(nonce) == "" {
		return "Pairing failed: the Telegram start message did not include a pairing code. Open the full link from /connect telegram, or manually send /start <code>."
	}
	if profile.PairingNonce == "" || time.Now().Unix() > profile.PairingExpires {
		return "Pairing failed: the pairing link is expired. Run /connect telegram pair in NekoCode again and open the new link."
	}
	return "Pairing failed: the pairing code does not match the current NekoCode session. Run /connect telegram pair again and use the newest link."
}

func (c *Connector) handleCallbackQuery(ctx context.Context, client *apiClient, profile *BotProfile, cb *CallbackQuery) {
	if cb == nil {
		return
	}
	chatID := int64(0)
	if cb.Message != nil {
		chatID = cb.Message.Chat.ID
	}
	if !profile.isAllowed(cb.From.ID) {
		_ = client.answerCallbackQuery(ctx, cb.ID, "This Telegram account is not connected.")
		if chatID != 0 {
			_ = client.sendMessage(ctx, chatID, "This Telegram account is not connected to NekoCode.")
		}
		return
	}
	if chatID != 0 {
		profile.touchOwner(cb.From.ID, cb.From.Username, chatID)
	}

	parts := strings.Split(cb.Data, ":")
	if len(parts) < 2 {
		_ = client.answerCallbackQuery(ctx, cb.ID, "Unknown action.")
		return
	}
	action, id := parts[0], parts[1]
	var err error
	var okText string
	switch action {
	case "approve":
		err = c.rt.Approve(ctx, id, controlruntime.ApprovalDecision{Allowed: true})
		okText = "Approved"
	case "reject":
		err = c.rt.Approve(ctx, id, controlruntime.ApprovalDecision{})
		okText = "Rejected"
	case "dismiss":
		_, err = c.tracker().RejectQuestion(id)
		if err == nil {
			err = c.rt.Answer(ctx, id, view.QuestionReply{Rejected: true})
		}
		okText = "Dismissed"
	case "answer":
		if len(parts) < 3 {
			err = fmt.Errorf("missing option")
			break
		}
		idx, parseErr := strconv.Atoi(parts[2])
		if parseErr != nil {
			err = parseErr
			break
		}
		var reply view.QuestionReply
		var resolvedID string
		reply, resolvedID, err = c.tracker().BuildQuestionOptionReply(id, idx)
		if err == nil {
			err = c.rt.Answer(ctx, resolvedID, reply)
		}
		okText = "Answered"
	default:
		err = fmt.Errorf("unknown action %q", action)
	}
	if err != nil {
		_ = client.answerCallbackQuery(ctx, cb.ID, "Error: "+err.Error())
		if chatID != 0 {
			_ = client.sendMessage(ctx, chatID, taskview.HTMLEscape("Error: "+err.Error()))
		}
		return
	}
	_ = client.answerCallbackQuery(ctx, cb.ID, okText)
}

func (c *Connector) publishStatus(status, message string) {
	if publisher, ok := c.rt.(interface {
		Publish(controlruntime.Event)
	}); ok {
		publisher.Publish(controlruntime.Event{
			Type:   controlruntime.EventConnectorStatus,
			Source: controlruntime.SourceRef{Kind: "telegram"},
			Payload: controlruntime.ConnectorStatusPayload{
				Name:    "telegram",
				Status:  status,
				Message: message,
			},
		})
	}
}

func (c *Connector) eventLoop(ctx context.Context, client *apiClient) {
	events, err := c.rt.Subscribe(ctx, controlruntime.EventFilter{})
	if err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			text := c.renderEvent(ev)
			if text == "" {
				continue
			}
			cfg, err := loadConfig()
			if err != nil {
				continue
			}
			profile, ok := cfg.activeProfile()
			if !ok {
				continue
			}
			for _, chatID := range profile.pairedChatIDs() {
				if keyboard, ok := eventKeyboard(ev); ok {
					_ = client.sendMessageWithKeyboard(ctx, chatID, text, keyboard)
				} else {
					_ = client.sendMessage(ctx, chatID, text)
				}
			}
		}
	}
}

func (c *Connector) renderEvent(ev controlruntime.Event) string {
	return c.tracker().RenderEvent(ev)
}

func eventKeyboard(ev controlruntime.Event) (inlineKeyboardMarkup, bool) {
	switch ev.Type {
	case controlruntime.EventApprovalRequested:
		p, ok := ev.Payload.(controlruntime.ApprovalView)
		if !ok || p.ID == "" {
			return inlineKeyboardMarkup{}, false
		}
		return inlineKeyboardMarkup{InlineKeyboard: [][]inlineKeyboardButton{
			{
				{Text: "Approve", CallbackData: "approve:" + p.ID},
				{Text: "Reject", CallbackData: "reject:" + p.ID},
			},
		}}, true
	case controlruntime.EventQuestionRequested:
		p, ok := ev.Payload.(controlruntime.QuestionView)
		if !ok || !taskview.UsesQuestionButtons(p) {
			return inlineKeyboardMarkup{}, false
		}
		q := p.Questions[0]
		rows := make([][]inlineKeyboardButton, 0, len(q.Options)+1)
		for i, opt := range q.Options {
			rows = append(rows, []inlineKeyboardButton{{
				Text:         opt.Label,
				CallbackData: fmt.Sprintf("answer:%s:%d", p.ID, i),
			}})
		}
		rows = append(rows, []inlineKeyboardButton{{Text: "Dismiss", CallbackData: "dismiss:" + p.ID}})
		return inlineKeyboardMarkup{InlineKeyboard: rows}, true
	default:
		return inlineKeyboardMarkup{}, false
	}
}

func (c *Connector) tracker() *taskview.Tracker {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.taskTracker == nil {
		c.taskTracker = taskview.NewTracker()
	}
	return c.taskTracker
}

func parseAnswerCommand(text string) (questionID, answer string) {
	rest := strings.TrimSpace(strings.TrimPrefix(text, "/answer"))
	if rest == "" {
		return "", ""
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", ""
	}
	if strings.HasPrefix(fields[0], "q_") {
		return fields[0], strings.TrimSpace(strings.TrimPrefix(rest, fields[0]))
	}
	return "", rest
}

func (c *Connector) replyErr(ctx context.Context, client *apiClient, chatID int64, ok string, err error) {
	if err != nil {
		_ = client.sendMessage(ctx, chatID, taskview.HTMLEscape("Error: "+err.Error()))
		return
	}
	_ = client.sendMessage(ctx, chatID, taskview.HTMLEscape(ok))
}
