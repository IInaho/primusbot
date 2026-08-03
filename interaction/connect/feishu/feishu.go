package feishu

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	larkcallback "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"nekocode/interaction/connect"
	controlruntime "nekocode/runtime"
)

// Connector bridges feishu (Lark) and the control runtime: inbound DM
// messages become runtime inputs, outbound runtime events go through the
// connect protocol layer (Translate → Intent → Sink) to the paired chat.
// Feishu cannot edit messages, so the capability set excludes EditMessages:
// no streaming preview (multi-message "streaming" is spam), and no
// event-driven card replacement — approval cards are replaced in the button
// callback instead. The run-state machine, pairing primitives, and shared
// commands come from the shared connect package.
type Connector struct {
	rt   controlruntime.ConnectorRuntime
	base *connect.Base

	questions *connect.QuestionTracker

	// approvals tracks cards in flight (approval ID → view) so button
	// callbacks can render the resolved replacement card.
	approvals sync.Map
}

func New(rt controlruntime.ConnectorRuntime) *Connector {
	return &Connector{
		rt:        rt,
		base:      connect.NewBase(rt, "feishu", "Feishu"),
		questions: connect.NewQuestionTracker(),
	}
}

func (c *Connector) Name() string { return "feishu" }

func (c *Connector) Start(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if !cfg.configured() {
		return fmt.Errorf("feishu app credentials are not configured")
	}
	if c.base.IsRunning() {
		return nil
	}
	runCtx, generation := c.base.Start(ctx)
	client := newFeishuClient(cfg.AppID, cfg.AppSecret)

	c.base.PublishStatus("running", "")
	go c.wsLoop(runCtx, client, generation)
	go c.eventLoop(runCtx, client)
	return nil
}

func (c *Connector) Stop() error {
	return c.base.Stop()
}

func (c *Connector) ConnectorStatusView() controlruntime.ConnectorView {
	cfg, err := loadConfig()
	view := connect.StatusView("feishu", c.base.IsRunning())
	if err != nil {
		view.Status = "error"
		view.Message = err.Error()
		return view
	}
	view.Configured = cfg.configured()
	if cfg.AppID != "" {
		view.Metadata["app_id"] = cfg.AppID
	}
	if !view.Configured {
		view.Status = "unconfigured"
		view.Message = "Run /connect feishu add <app-id> <app-secret> first."
	}
	return view
}

// wsLoop runs the SDK websocket with a reconnect backoff: the SDK
// auto-reconnects transient drops, so a returned error with a live context
// is treated as fatal-but-retryable (e.g. credential or network issues).
func (c *Connector) wsLoop(ctx context.Context, client *feishuClient, generation int) {
	defer c.base.MarkStopped(generation)
	for {
		err := client.serveWS(ctx, func(evtCtx context.Context, ev *larkim.P2MessageReceiveV1) error {
			c.handleMessage(evtCtx, client, ev)
			return nil
		}, c.handleCardAction)
		if ctx.Err() != nil {
			return
		}
		c.base.PublishStatus("error", fmt.Sprintf("Feishu websocket exited: %v", err))
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *Connector) handleMessage(ctx context.Context, client *feishuClient, ev *larkim.P2MessageReceiveV1) {
	chatID, openID, msgType, text, ok := messageText(ev)
	if !ok {
		return
	}
	if msgType != "text" {
		_ = client.sendText(ctx, chatID, fmt.Sprintf("暂不支持的消息类型：%s，请发送文本消息。", msgType))
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		return
	}

	// Pairing: an unpaired sender who presents the in-flight nonce becomes
	// the owner. Everyone else gets turned away.
	if !cfg.isAllowed(openID) {
		if cfg.Valid(text, time.Now().Unix()) {
			cfg.finishPairing(openID, chatID)
			if err := saveConfig(cfg); err == nil {
				_ = client.sendText(ctx, chatID, "NekoCode connected. Send a message to control the current session.")
				c.base.PublishStatus("paired", "Feishu connected: "+openID)
			}
			return
		}
		_ = client.sendText(ctx, chatID, "This Feishu account is not connected to NekoCode. Run /connect feishu pair in NekoCode to bind this account.")
		return
	}
	cfg.touchOwner(openID, chatID)
	_ = saveConfig(cfg)

	// Shared commands (/stop /help /approve /reject /answer /dismiss), then chat.
	cmds := connect.CommandHandler{RT: c.rt, Help: helpText(), Questions: c.questions}
	if reply, handled := cmds.Handle(ctx, text); handled {
		_ = client.sendText(ctx, chatID, reply)
		return
	}
	_, err = c.rt.StartRun(context.WithoutCancel(ctx), controlruntime.Input{
		Source: controlruntime.SourceRef{Kind: "feishu", ID: chatID},
		Sender: controlruntime.SenderRef{ID: openID},
		Text:   text,
	})
	if err != nil {
		_ = client.sendText(ctx, chatID, "Error: "+err.Error())
	}
}

// eventLoop delivers runtime events to the paired owner's chat through the
// connect protocol layer.
func (c *Connector) eventLoop(ctx context.Context, client *feishuClient) {
	_ = connect.Dispatch(ctx, c.rt, eventSink{c: c, client: client})
}

// messageSender is the outbound subset of feishuClient the event sink uses;
// feishuClient satisfies it, and tests substitute a fake.
type messageSender interface {
	sendText(ctx context.Context, chatID, text string) error
	sendCard(ctx context.Context, chatID string, card map[string]any) error
}

// eventSink is feishu's connect.Sink implementation: cards and buttons are
// available, but messages cannot be edited (EditMessages=false), so the
// dispatcher drops preview intents and feishu never streams. Question
// intents also feed the pending-question tracker so /answer can omit the
// question ID.
type eventSink struct {
	c      *Connector
	client messageSender
}

func (s eventSink) Caps() connect.Capabilities {
	return connect.Capabilities{Buttons: true, RichCards: true}
}

func (s eventSink) Post(ctx context.Context, in connect.Intent) error {
	switch in.Kind {
	case connect.IntentApproval:
		// Approval requests go out as interactive cards; on any card
		// failure the plain text (with /approve commands) is the fallback (降级).
		if in.Approval != nil && in.Approval.ID != "" {
			if err := s.c.sendApprovalCard(ctx, s.client, *in.Approval); err == nil {
				return nil
			}
		}
		s.c.sendToOwner(ctx, s.client, in.Text)
	case connect.IntentQuestion:
		if in.Question != nil {
			s.c.questions.Add(*in.Question)
		}
		s.c.sendToOwner(ctx, s.client, in.Text)
	case connect.IntentQuestionResolved:
		s.c.questions.Remove(in.ID)
	case connect.IntentApprovalResolved:
		// Feishu does not replace cards from events — the resolved card is
		// rendered in the button callback. Nothing to push.
	case connect.IntentResult, connect.IntentFailed:
		s.sendResult(ctx, in.Text)
	case connect.IntentStopped:
		s.c.sendToOwner(ctx, s.client, in.Text)
	}
	return nil
}

// sendResult delivers a run outcome as a markdown card (card JSON 2.0): the
// text is LLM-produced markdown and the 2.0 markdown component renders it
// (headings, lists, tables, code blocks) where plain text would lose it.
// Overlong content is truncated to a conservative size first.
func (s eventSink) sendResult(ctx context.Context, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	cfg, err := loadConfig()
	if err != nil {
		return
	}
	card := markdownCard(connect.TruncateRunes(text, maxMarkdownRunes))
	for _, chatID := range cfg.pairedChatIDs() {
		// 卡片发送失败时降级为原始纯文本(与审批卡片同一降级模式)。
		if err := s.client.sendCard(ctx, chatID, card); err != nil {
			_ = s.client.sendText(ctx, chatID, text)
		}
	}
}

// sendApprovalCard delivers the approval card to the paired chat and
// remembers the view so the button callback can render the resolved card.
func (c *Connector) sendApprovalCard(ctx context.Context, client messageSender, p controlruntime.ApprovalView) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	chats := cfg.pairedChatIDs()
	if len(chats) == 0 {
		return fmt.Errorf("no paired chat")
	}
	if err := client.sendCard(ctx, chats[0], approvalCard(p)); err != nil {
		return err
	}
	c.approvals.Store(p.ID, p)
	return nil
}

// handleCardAction processes approval button clicks: decode the action
// value, gate on the paired owner, decide via the runtime, and answer with
// a toast plus a button-free replacement card.
func (c *Connector) handleCardAction(ctx context.Context, ev *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error) {
	if ev == nil || ev.Event == nil || ev.Event.Action == nil {
		return toastResponse("error", "无法识别的卡片操作"), nil
	}
	approvalID, decision, err := decodeCardActionValue(ev.Event.Action.Value)
	if err != nil {
		return toastResponse("error", "无法识别的卡片操作"), nil
	}

	// Approval is fail-closed: both a readable pairing and an identified
	// operator are required before the runtime is called.
	cfg, cfgErr := loadConfig()
	if cfgErr != nil || ev.Event.Operator == nil || !cfg.isAllowed(ev.Event.Operator.OpenID) {
		return toastResponse("error", "只有已配对的用户才能执行审批"), nil
	}

	runtimeDecision, err := connect.ApprovalDecisionFor(decision)
	if err != nil {
		return toastResponse("error", "无法识别的卡片操作"), nil
	}
	if err := c.rt.DecideApproval(ctx, approvalID, runtimeDecision); err != nil {
		if connect.IsResolvedErr(err) {
			// Double click, or already handled via the /approve text
			// command — not an error worth alarming about.
			c.approvals.Delete(approvalID)
			return toastResponse("info", "该审批已处理"), nil
		}
		return toastResponse("error", "审批失败: "+err.Error()), nil
	}

	resp := toastResponse("success", connect.VerdictForAction(decision))
	if v, ok := c.approvals.LoadAndDelete(approvalID); ok {
		if view, ok := v.(controlruntime.ApprovalView); ok {
			resp.Card = &larkcallback.Card{Type: "raw", Data: resolvedCard(view, decision)}
		}
	}
	return resp, nil
}

func (c *Connector) sendToOwner(ctx context.Context, client messageSender, text string) {
	cfg, err := loadConfig()
	if err != nil {
		return
	}
	for _, chatID := range cfg.pairedChatIDs() {
		_ = client.sendText(ctx, chatID, text)
	}
}

func helpText() string {
	return connect.SharedHelp("", "Send any other text to chat with the current session.")
}
