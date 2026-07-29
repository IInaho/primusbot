package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"nekocode/interaction/connect/core"
	"nekocode/interaction/connect/telegram/internal/taskview"
	controlruntime "nekocode/runtime"
)

// msgRef locates a sent interactive message so it can be terminalized
// (verdict text + keyboard stripped) once its approval/question resolves.
type msgRef struct {
	chatID    int64
	messageID int
	text      string
}

type Connector struct {
	rt   controlruntime.Runtime
	base *core.Base

	mu          sync.Mutex
	active      string
	taskTracker *taskview.Tracker
	// pendingMsgs maps approval/question IDs to their interactive messages;
	// pendingSelect holds multi-select toggle state per question ID.
	pendingMsgs   map[string]msgRef
	pendingSelect map[string]map[int]bool
}

func New(rt controlruntime.Runtime) *Connector {
	return &Connector{
		rt:            rt,
		base:          core.NewBase(rt, "telegram", "Telegram"),
		taskTracker:   taskview.NewTracker(),
		pendingMsgs:   make(map[string]msgRef),
		pendingSelect: make(map[string]map[int]bool),
	}
}

func (c *Connector) Name() string { return "telegram" }

func (c *Connector) ConnectorStatusView() controlruntime.ConnectorView {
	cfg, err := loadConfig()
	running := c.base.IsRunning()

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
	if profile.Nonce != "" && time.Now().Unix() <= profile.Expires {
		view.Metadata["pairing_expires"] = profile.Expires
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
	if c.base.IsRunning() && c.active == profile.Name {
		c.mu.Unlock()
		return nil
	}
	// The base detaches from the caller's context (which may be a single
	// HTTP request or a finished run) and cancels any previous run.
	runCtx, generation := c.base.Start(ctx)
	client := newAPIClient(profile.BotToken)
	c.active = profile.Name
	c.mu.Unlock()

	c.base.PublishStatus("running", "")
	go c.pollLoop(runCtx, client, generation)
	go c.eventLoop(runCtx, client)
	return nil
}

func (c *Connector) Stop() error {
	c.mu.Lock()
	c.active = ""
	c.mu.Unlock()
	return c.base.Stop()
}

func (c *Connector) pollLoop(ctx context.Context, client *apiClient, generation int) {
	defer func() {
		c.mu.Lock()
		c.active = ""
		c.mu.Unlock()
		c.base.MarkStopped(generation)
	}()
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
		if nonce != "" && nonce == profile.Nonce && time.Now().Unix() <= profile.Expires {
			if profile.Owner != nil && profile.Owner.UserID != msg.From.ID {
				_ = client.sendMessage(ctx, msg.Chat.ID, "Pairing failed: this Telegram profile is already paired. Run /connect telegram unpair in NekoCode first.")
				return
			}
			profile.setOwner(msg.From.ID, msg.From.Username, msg.Chat.ID)
			profile.Pairing.Clear()
			_ = client.sendMessage(ctx, msg.Chat.ID, "NekoCode connected. Send a message to control the current session.")
			c.base.PublishStatus("paired", fmt.Sprintf("Telegram connected: @%s", msg.From.Username))
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

	// Shared commands (/stop /help /approve /reject) come from the connector
	// core; telegram-specific commands follow below.
	cmds := core.CommandHandler{RT: c.rt, Help: taskview.Help()}
	if reply, handled := cmds.Handle(ctx, text); handled {
		_ = client.sendMessage(ctx, msg.Chat.ID, taskview.HTMLEscape(reply))
		return
	}

	switch {
	case text == "/answer" || strings.HasPrefix(text, "/answer "):
		id, answer := parseAnswerCommand(text)
		reply, resolvedID, err := c.tracker().BuildQuestionReply(id, answer)
		if err == nil {
			err = c.rt.Answer(ctx, resolvedID, reply)
		}
		c.replyErr(ctx, client, msg.Chat.ID, "答案已发送。", err)
	case text == "/dismiss" || strings.HasPrefix(text, "/dismiss "):
		id := strings.TrimSpace(strings.TrimPrefix(text, "/dismiss"))
		resolvedID, err := c.tracker().RejectQuestion(id)
		if err == nil {
			err = c.rt.Answer(ctx, resolvedID, controlruntime.QuestionReply{Rejected: true})
		}
		c.replyErr(ctx, client, msg.Chat.ID, "已忽略。", err)
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
	if profile.Nonce == "" || time.Now().Unix() > profile.Expires {
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
		_ = client.answerCallbackQuery(ctx, cb.ID, "未知操作。")
		return
	}
	action, id := parts[0], parts[1]

	verdicts := map[string]controlruntime.ApprovalDecision{
		"approve":  {Allowed: true},
		"remember": {Allowed: true, Remember: true},
		"escalate": {Allowed: true, AllowWithPermission: true},
		"reject":   {},
	}
	verdictText := map[string]string{
		"approve": "已批准", "remember": "已永久允许", "escalate": "已批准并授权", "reject": "已拒绝",
	}

	switch action {
	case "approve", "remember", "escalate", "reject":
		err := c.rt.Approve(ctx, id, verdicts[action])
		if err != nil {
			if isStaleRequest(err) {
				c.terminalize(ctx, client, id, verdictText[action])
				_ = client.answerCallbackQuery(ctx, cb.ID, "该请求已处理")
				return
			}
			_ = client.answerCallbackQuery(ctx, cb.ID, "错误: "+err.Error())
			return
		}
		c.terminalize(ctx, client, id, verdictText[action])
		_ = client.answerCallbackQuery(ctx, cb.ID, verdictText[action])
	case "dismiss":
		_, err := c.tracker().RejectQuestion(id)
		if err == nil {
			err = c.rt.Answer(ctx, id, controlruntime.QuestionReply{Rejected: true})
		}
		if err != nil {
			if isStaleRequest(err) {
				c.terminalize(ctx, client, id, "已忽略")
				_ = client.answerCallbackQuery(ctx, cb.ID, "该请求已处理")
				return
			}
			_ = client.answerCallbackQuery(ctx, cb.ID, "错误: "+err.Error())
			return
		}
		c.terminalize(ctx, client, id, "已忽略")
		_ = client.answerCallbackQuery(ctx, cb.ID, "已忽略")
	case "answer":
		c.handleAnswerCallback(ctx, client, cb, id, parts[2:])
	default:
		_ = client.answerCallbackQuery(ctx, cb.ID, "未知操作。")
	}
}

// handleAnswerCallback dispatches question-answer callbacks: single-select
// submits immediately; multi-select toggles options (redrawing the keyboard)
// until the confirm button submits the selection.
func (c *Connector) handleAnswerCallback(ctx context.Context, client *apiClient, cb *CallbackQuery, id string, rest []string) {
	if len(rest) == 0 {
		_ = client.answerCallbackQuery(ctx, cb.ID, "未知操作。")
		return
	}

	// Confirm submits the accumulated multi-selection.
	if rest[0] == "confirm" {
		c.mu.Lock()
		selected := c.pendingSelect[id]
		c.mu.Unlock()
		indices := make([]int, 0, len(selected))
		for idx, on := range selected {
			if on {
				indices = append(indices, idx)
			}
		}
		reply, resolvedID, err := c.tracker().BuildQuestionMultiOptionReply(id, indices)
		if err == nil {
			err = c.rt.Answer(ctx, resolvedID, reply)
		}
		if err != nil {
			if isStaleRequest(err) {
				c.terminalize(ctx, client, id, "已回答")
				_ = client.answerCallbackQuery(ctx, cb.ID, "该请求已处理")
				return
			}
			_ = client.answerCallbackQuery(ctx, cb.ID, "错误: "+err.Error())
			return
		}
		c.terminalize(ctx, client, id, "已回答")
		_ = client.answerCallbackQuery(ctx, cb.ID, "已回答")
		return
	}

	idx, parseErr := strconv.Atoi(rest[0])
	if parseErr != nil {
		_ = client.answerCallbackQuery(ctx, cb.ID, "未知操作。")
		return
	}

	// Multi-select: toggle the option and redraw the keyboard in place.
	if view, ok := c.tracker().PendingQuestion(id); ok && len(view.Questions) == 1 && view.Questions[0].Multiple {
		if idx < 0 || idx >= len(view.Questions[0].Options) {
			_ = client.answerCallbackQuery(ctx, cb.ID, "未知选项。")
			return
		}
		c.mu.Lock()
		sel := c.pendingSelect[id]
		if sel == nil {
			sel = make(map[int]bool)
			c.pendingSelect[id] = sel
		}
		sel[idx] = !sel[idx]
		ref, hasRef := c.pendingMsgs[id]
		c.mu.Unlock()
		if hasRef {
			keyboard, _ := c.eventKeyboard(controlruntime.Event{Type: controlruntime.EventQuestionRequested, Payload: view})
			_ = client.editMessage(ctx, ref.chatID, ref.messageID, ref.text, &keyboard)
		}
		_ = client.answerCallbackQuery(ctx, cb.ID, "")
		return
	}

	// Single-select: submit immediately.
	reply, resolvedID, err := c.tracker().BuildQuestionOptionReply(id, idx)
	if err == nil {
		err = c.rt.Answer(ctx, resolvedID, reply)
	}
	if err != nil {
		if isStaleRequest(err) {
			c.terminalize(ctx, client, id, "已回答")
			_ = client.answerCallbackQuery(ctx, cb.ID, "该请求已处理")
			return
		}
		_ = client.answerCallbackQuery(ctx, cb.ID, "错误: "+err.Error())
		return
	}
	c.terminalize(ctx, client, id, "已回答")
	_ = client.answerCallbackQuery(ctx, cb.ID, "已回答")
}

// isStaleRequest reports whether err means the approval/question was already
// resolved (here or on another surface) — i.e. terminalize rather than error.
func isStaleRequest(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not pending") ||
		strings.Contains(msg, "already resolved") ||
		strings.Contains(msg, "not found")
}

func (c *Connector) eventLoop(ctx context.Context, client *apiClient) {
	events, err := c.rt.Subscribe(ctx, controlruntime.EventFilter{})
	if err != nil {
		return
	}
	preview := newPreviewTracker(client, c.pairedChats)
	ticker := time.NewTicker(previewEditInterval / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			preview.flush(ctx)
		case ev, ok := <-events:
			if !ok {
				return
			}
			// Assistant text deltas feed the streaming preview instead of
			// becoming individual messages.
			if ev.Type == controlruntime.EventAssistantDelta {
				if p, ok := ev.Payload.(controlruntime.DeltaPayload); ok {
					preview.addDelta(ctx, ev.RunID, p.Delta)
				}
				continue
			}
			skipDoneReply := false
			switch ev.Type {
			case controlruntime.EventRunDone:
				// A settled preview already shows the final text; the tracker's
				// done-reply would duplicate it.
				skipDoneReply = preview.finalize(ctx, ev.RunID)
			case controlruntime.EventRunFailed, controlruntime.EventRunAborted:
				// Settle the preview but keep the done-reply (error context).
				preview.finalize(ctx, ev.RunID)
			case controlruntime.EventApprovalResolved:
				// Resolved on another surface (TUI, HTTP): terminalize the
				// pending message with the canonical verdict (first-answer-wins).
				if p, ok := ev.Payload.(controlruntime.ApprovalView); ok {
					status := "已处理"
					switch p.Status {
					case controlruntime.ApprovalApproved:
						status = "已批准"
					case controlruntime.ApprovalRejected:
						status = "已拒绝"
					}
					c.terminalize(ctx, client, p.ID, status)
				}
			case controlruntime.EventQuestionResolved:
				if p, ok := ev.Payload.(controlruntime.QuestionView); ok {
					c.terminalize(ctx, client, p.ID, "已回答")
				}
			}
			text := c.renderEvent(ev)
			if text == "" || skipDoneReply {
				continue
			}
			c.sendToChats(ctx, client, ev, text)
		}
	}
}

// pairedChats returns the chats of the active profile to broadcast to.
func (c *Connector) pairedChats() []int64 {
	cfg, err := loadConfig()
	if err != nil {
		return nil
	}
	profile, ok := cfg.activeProfile()
	if !ok {
		return nil
	}
	return profile.pairedChatIDs()
}

func (c *Connector) sendToChats(ctx context.Context, client *apiClient, ev controlruntime.Event, text string) {
	for _, chatID := range c.pairedChats() {
		if keyboard, ok := c.eventKeyboard(ev); ok {
			id, err := client.sendMessageWithKeyboard(ctx, chatID, text, keyboard)
			if err == nil {
				if pid := pendingID(ev); pid != "" {
					c.mu.Lock()
					c.pendingMsgs[pid] = msgRef{chatID: chatID, messageID: id, text: text}
					c.mu.Unlock()
				}
			}
		} else {
			_ = client.sendMessage(ctx, chatID, text)
		}
	}
}

// pendingID returns the approval/question ID carried by an interactive event.
func pendingID(ev controlruntime.Event) string {
	switch ev.Type {
	case controlruntime.EventApprovalRequested:
		if p, ok := ev.Payload.(controlruntime.ApprovalView); ok {
			return p.ID
		}
	case controlruntime.EventQuestionRequested:
		if p, ok := ev.Payload.(controlruntime.QuestionView); ok {
			return p.ID
		}
	}
	return ""
}

// terminalize rewrites an interactive message into its verdict form (status
// line appended, keyboard stripped). First-answer-wins: the loser surface is
// terminalized with the canonical verdict rather than showing an error.
// Idempotent — unknown or already-terminalized IDs are no-ops.
func (c *Connector) terminalize(ctx context.Context, client *apiClient, id, status string) {
	c.mu.Lock()
	ref, ok := c.pendingMsgs[id]
	delete(c.pendingMsgs, id)
	delete(c.pendingSelect, id)
	c.mu.Unlock()
	if !ok {
		return
	}
	body := ref.text + "\n\n<b>✅ " + taskview.HTMLEscape(status) + "</b>"
	_ = client.editMessage(ctx, ref.chatID, ref.messageID, body, &emptyKeyboard)
}

func (c *Connector) renderEvent(ev controlruntime.Event) string {
	return c.tracker().RenderEvent(ev)
}

// eventKeyboard builds the inline keyboard for interactive events. For
// multi-select questions it renders toggle buttons (✅ marks current
// selection) plus confirm/dismiss; single-select and approvals submit on tap.
func (c *Connector) eventKeyboard(ev controlruntime.Event) (inlineKeyboardMarkup, bool) {
	switch ev.Type {
	case controlruntime.EventApprovalRequested:
		p, ok := ev.Payload.(controlruntime.ApprovalView)
		if !ok || p.ID == "" {
			return inlineKeyboardMarkup{}, false
		}
		row := []inlineKeyboardButton{
			{Text: "批准一次", CallbackData: "approve:" + p.ID},
			{Text: "永久允许", CallbackData: "remember:" + p.ID},
			{Text: "拒绝", CallbackData: "reject:" + p.ID},
		}
		if p.CanEscalatePermission {
			row = append(row, inlineKeyboardButton{Text: "允许并授权", CallbackData: "escalate:" + p.ID})
		}
		return inlineKeyboardMarkup{InlineKeyboard: [][]inlineKeyboardButton{row}}, true
	case controlruntime.EventQuestionRequested:
		p, ok := ev.Payload.(controlruntime.QuestionView)
		if !ok || !taskview.UsesQuestionButtons(p) {
			return inlineKeyboardMarkup{}, false
		}
		q := p.Questions[0]
		rows := make([][]inlineKeyboardButton, 0, len(q.Options)+1)
		if q.Multiple {
			c.mu.Lock()
			selected := c.pendingSelect[p.ID]
			c.mu.Unlock()
			for i, opt := range q.Options {
				label := opt.Label
				if selected[i] {
					label = "✅ " + label
				}
				rows = append(rows, []inlineKeyboardButton{{
					Text:         label,
					CallbackData: fmt.Sprintf("answer:%s:%d", p.ID, i),
				}})
			}
			rows = append(rows, []inlineKeyboardButton{
				{Text: "确认", CallbackData: "answer:" + p.ID + ":confirm"},
				{Text: "忽略", CallbackData: "dismiss:" + p.ID},
			})
			return inlineKeyboardMarkup{InlineKeyboard: rows}, true
		}
		for i, opt := range q.Options {
			rows = append(rows, []inlineKeyboardButton{{
				Text:         opt.Label,
				CallbackData: fmt.Sprintf("answer:%s:%d", p.ID, i),
			}})
		}
		rows = append(rows, []inlineKeyboardButton{{Text: "忽略", CallbackData: "dismiss:" + p.ID}})
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
