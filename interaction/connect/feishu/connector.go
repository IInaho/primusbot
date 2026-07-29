package feishu

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	larkcallback "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"nekocode/interaction/connect/core"
	controlruntime "nekocode/runtime"
)

// Connector bridges feishu (Lark) and the control runtime: inbound DM
// messages become runtime inputs, outbound runtime events are rendered to
// the paired chat. The run-state machine, pairing primitives, shared
// commands, and stream throttle come from the connector core.
type Connector struct {
	rt   controlruntime.Runtime
	base *core.Base

	stream *core.StreamBuffer

	// approvals tracks cards in flight (approval ID → view) so button
	// callbacks can render the resolved replacement card.
	approvals sync.Map
}

func New(rt controlruntime.Runtime) *Connector {
	return &Connector{
		rt:     rt,
		base:   core.NewBase(rt, "feishu", "Feishu"),
		stream: &core.StreamBuffer{},
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
		if cfg.Pairing.Valid(text, time.Now().Unix()) {
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

	// Shared commands (/stop /help /approve /reject), then chat.
	cmds := core.CommandHandler{RT: c.rt, Help: helpText()}
	if reply, handled := cmds.Handle(ctx, text); handled {
		_ = client.sendText(ctx, chatID, reply)
		return
	}
	_, err = c.rt.Submit(ctx, controlruntime.Input{
		Kind:   controlruntime.InputMessage,
		Source: controlruntime.SourceRef{Kind: "feishu", ID: chatID},
		Sender: controlruntime.SenderRef{ID: openID},
		Text:   text,
	})
	if err != nil {
		_ = client.sendText(ctx, chatID, "Error: "+err.Error())
	}
}

// eventLoop subscribes to the runtime broadcast and delivers rendered
// events to the paired owner's chat.
func (c *Connector) eventLoop(ctx context.Context, client *feishuClient) {
	_ = core.DispatchEvents(ctx, c.rt, c.renderOutbound, func(sendCtx context.Context, ev controlruntime.Event, text string) {
		// Approval requests go out as interactive cards; on any card
		// failure the rendered plain text (with /approve commands) is the
		// fallback (降级).
		if ev.Type == controlruntime.EventApprovalRequested {
			if p, ok := ev.Payload.(controlruntime.ApprovalView); ok && p.ID != "" {
				if err := c.sendApprovalCard(sendCtx, client, p); err == nil {
					return
				}
			}
		}
		c.sendToOwner(sendCtx, client, text)
	})
}

// sendApprovalCard delivers the approval card to the paired chat and
// remembers the view so the button callback can render the resolved card.
func (c *Connector) sendApprovalCard(ctx context.Context, client *feishuClient, p controlruntime.ApprovalView) error {
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

	// Only the paired owner may approve.
	if cfg, cfgErr := loadConfig(); cfgErr == nil && ev.Event.Operator != nil && !cfg.isAllowed(ev.Event.Operator.OpenID) {
		return toastResponse("error", "只有已配对的用户才能执行审批"), nil
	}

	runtimeDecision, err := approvalDecisionFor(decision)
	if err != nil {
		return toastResponse("error", "无法识别的卡片操作"), nil
	}
	if err := c.rt.Approve(ctx, approvalID, runtimeDecision); err != nil {
		if isAlreadyResolvedErr(err) {
			// Double click, or already handled via the /approve text
			// command — not an error worth alarming about.
			c.approvals.Delete(approvalID)
			return toastResponse("info", "该审批已处理"), nil
		}
		return toastResponse("error", "审批失败: "+err.Error()), nil
	}

	verdictText, _ := verdict(decision)
	resp := toastResponse("success", verdictText)
	if v, ok := c.approvals.LoadAndDelete(approvalID); ok {
		if view, ok := v.(controlruntime.ApprovalView); ok {
			resp.Card = &larkcallback.Card{Type: "raw", Data: resolvedCard(view, decision)}
		}
	}
	return resp, nil
}

// renderOutbound converts one event into zero or more outbound texts,
// merging assistant/reasoning deltas through the stream buffer.
func (c *Connector) renderOutbound(ev controlruntime.Event) []string {
	switch ev.Type {
	case controlruntime.EventRunStarted:
		c.stream.Reset()
		return nil
	case controlruntime.EventAssistantDelta, controlruntime.EventReasoningDelta:
		p, ok := ev.Payload.(controlruntime.DeltaPayload)
		if !ok {
			return nil
		}
		var out []string
		if chunk := c.stream.Add(p.Delta, time.Now()); chunk != "" {
			out = append(out, chunk)
		}
		if p.Done {
			if chunk := c.stream.Drain(); chunk != "" {
				out = append(out, chunk)
			}
		}
		return out
	case controlruntime.EventRunDone:
		// The streamed text already covered the final answer; only send the
		// output when nothing was streamed (non-streaming provider path).
		var out []string
		if chunk := c.stream.Drain(); chunk != "" {
			out = append(out, chunk)
		}
		if !c.stream.StreamedAny() {
			if p, ok := ev.Payload.(controlruntime.DonePayload); ok && strings.TrimSpace(p.Output) != "" {
				out = append(out, p.Output)
			}
		}
		return out
	default:
		if text := renderEvent(ev); text != "" {
			return []string{text}
		}
		return nil
	}
}

func (c *Connector) sendToOwner(ctx context.Context, client *feishuClient, text string) {
	cfg, err := loadConfig()
	if err != nil {
		return
	}
	for _, chatID := range cfg.pairedChatIDs() {
		_ = client.sendText(ctx, chatID, text)
	}
}

func helpText() string {
	return strings.Join([]string{
		"Commands:",
		"  /stop          Stop the current run",
		"  /approve <id>  Approve a pending tool call once",
		"  /always <id>   Approve and remember (always allow)",
		"  /reject <id>   Reject a pending tool call",
		"  /help          Show this help",
		"",
		"Send any other text to chat with the current session.",
	}, "\n")
}
