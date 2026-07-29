package qqbot

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"nekocode/interaction/connect/core"
	controlruntime "nekocode/runtime"
)

// Connector 通过腾讯官方 QQ 机器人开放平台把 QQ 桥接到控制 runtime：
// 入站消息（群 @ / C2C 私聊，平台已做触发过滤，全部受理）成为 runtime
// 输入，出站 runtime 事件渲染为纯文本发回已知会话。QQ 不能编辑消息，
// 因此不做流式预览。
type Connector struct {
	rt   controlruntime.Runtime
	base *core.Base

	mu              sync.Mutex
	chats           map[string]chatSession // 已知会话（受理入站消息时记录）
	pendingQuestion string                 // 当前待回答的问题 ID
}

func New(rt controlruntime.Runtime) *Connector {
	return &Connector{
		rt:    rt,
		base:  core.NewBase(rt, "qqbot", "QQBot"),
		chats: make(map[string]chatSession),
	}
}

func (c *Connector) Name() string { return "qqbot" }

func (c *Connector) Start(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if !cfg.configured() {
		return fmt.Errorf("qqbot app credentials are not configured")
	}
	if c.base.IsRunning() {
		return nil
	}
	// base 会脱离调用方 ctx（可能是单次 HTTP 请求）并取消上一轮运行。
	runCtx, generation := c.base.Start(ctx)
	client := newAPIClient(cfg.AppID, cfg.AppSecret, cfg.Sandbox)

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
	running := c.base.IsRunning()

	view := controlruntime.ConnectorView{
		Name:        "qqbot",
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
	view.Configured = cfg.configured()
	if cfg.AppID != "" {
		view.Metadata["app_id"] = cfg.AppID
	}
	view.Metadata["sandbox"] = cfg.Sandbox
	if !view.Configured {
		view.Status = "unconfigured"
		view.Message = "Run /connect qqbot add <appid> <appsecret> first."
	}
	return view
}

// wsLoop 维持 gateway 连接：断线后指数退避重连（上限 30s），ctx 取消则退出。
// session 跨重连保留 sessionID/lastSeq，优先 Resume 续传。
func (c *Connector) wsLoop(ctx context.Context, client *apiClient, generation int) {
	defer c.base.MarkStopped(generation)
	session := &gatewaySession{client: client}
	backoff := time.Second
	for {
		err := session.run(ctx, func(msg inboundMessage) {
			c.handleMessage(ctx, client, msg)
		}, func() {
			c.base.PublishStatus("running", "QQBot gateway ready.")
		})
		if ctx.Err() != nil {
			return
		}
		c.base.PublishStatus("error", fmt.Sprintf("QQBot gateway disconnected: %v", err))
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}

func (c *Connector) handleMessage(ctx context.Context, client *apiClient, msg inboundMessage) {
	// 受理后记录会话与触发消息 id，出站事件据此投递被动回复。
	c.rememberChat(msg)

	text := msg.text
	if text == "" {
		return
	}

	// 共享命令（/stop /help /approve /always /reject），其次是 /answer，
	// 最后作为消息提交给 runtime。
	cmds := core.CommandHandler{RT: c.rt, Help: helpText()}
	if reply, handled := cmds.Handle(ctx, text); handled {
		c.reply(ctx, client, msg, reply)
		return
	}
	if text == "/answer" || strings.HasPrefix(text, "/answer ") {
		c.handleAnswer(ctx, client, msg, text)
		return
	}
	_, err := c.rt.Submit(ctx, controlruntime.Input{
		Kind:   controlruntime.InputMessage,
		Source: controlruntime.SourceRef{Kind: "qqbot", ID: msg.sourceID()},
		Sender: controlruntime.SenderRef{ID: msg.authorID},
		Text:   text,
	})
	if err != nil {
		c.reply(ctx, client, msg, "错误: "+err.Error())
	}
}

// handleAnswer 处理 /answer [question-id] <回答>：省略 id 时回答当前待答问题。
func (c *Connector) handleAnswer(ctx context.Context, client *apiClient, msg inboundMessage, text string) {
	rest := strings.TrimSpace(strings.TrimPrefix(text, "/answer"))
	id := ""
	if fields := strings.Fields(rest); len(fields) > 0 && strings.HasPrefix(fields[0], "q_") {
		id = fields[0]
		rest = strings.TrimSpace(strings.TrimPrefix(rest, fields[0]))
	}
	if id == "" {
		id = c.pendingQuestionID()
	}
	if id == "" {
		c.reply(ctx, client, msg, "当前没有待回答的问题。")
		return
	}
	if rest == "" {
		c.reply(ctx, client, msg, "用法：/answer <回答内容>")
		return
	}
	err := c.rt.Answer(ctx, id, controlruntime.QuestionReply{Answers: [][]string{{rest}}})
	if err != nil {
		c.reply(ctx, client, msg, "错误: "+err.Error())
		return
	}
	c.clearPendingQuestion(id)
	c.reply(ctx, client, msg, "答案已发送。")
}

// reply 回发触发会话；入站触发的回复必带触发消息 id（被动回复窗口内）。
func (c *Connector) reply(ctx context.Context, client *apiClient, msg inboundMessage, text string) {
	if text == "" {
		return
	}
	_ = sendChat(ctx, client, msg.chatKind, msg.chatID, text, msg.msgID)
}

// sendChat 按会话类型发文本；msgID 非空时作为被动回复携带。
func sendChat(ctx context.Context, client *apiClient, kind, id, text, msgID string) error {
	if kind == "group" {
		return client.sendGroupMessage(ctx, id, text, msgID)
	}
	return client.sendC2CMessage(ctx, id, text, msgID)
}

// eventLoop 订阅 runtime 事件并渲染成文本广播到已知会话。
func (c *Connector) eventLoop(ctx context.Context, client *apiClient) {
	_ = core.DispatchEvents(ctx, c.rt, func(ev controlruntime.Event) []string {
		if text := renderOutboundEvent(ev); text != "" {
			return []string{text}
		}
		return nil
	}, func(sendCtx context.Context, ev controlruntime.Event, text string) {
		// 跟踪待答问题，/answer 可以省略 question id。
		switch ev.Type {
		case controlruntime.EventQuestionRequested:
			if p, ok := ev.Payload.(controlruntime.QuestionView); ok {
				c.setPendingQuestion(p.ID)
			}
		case controlruntime.EventQuestionResolved:
			if p, ok := ev.Payload.(controlruntime.QuestionView); ok {
				c.clearPendingQuestion(p.ID)
			}
		}
		c.broadcast(sendCtx, client, text)
	})
}

// broadcast 向所有已知会话投递文本：带被动回复窗口内的 msg_id，
// 过期则尝试主动消息；失败写进 connector 状态。
func (c *Connector) broadcast(ctx context.Context, client *apiClient, text string) {
	now := time.Now()
	for _, chat := range c.knownChats() {
		msgID := chat.freshMsgID(now)
		if err := sendChat(ctx, client, chat.kind, chat.id, text, msgID); err != nil {
			c.base.PublishStatus("error", fmt.Sprintf("QQBot send to %s:%s failed: %v", chat.kind, chat.id, err))
		}
	}
}

// renderOutboundEvent 把 runtime 事件渲染为出站文本；"" 表示不产生消息。
func renderOutboundEvent(ev controlruntime.Event) string {
	switch ev.Type {
	case controlruntime.EventRunDone:
		p, ok := ev.Payload.(controlruntime.DonePayload)
		if !ok {
			return ""
		}
		if out := strings.TrimSpace(p.Output); out != "" {
			return out
		}
		if p.Error != "" {
			return "Error: " + p.Error
		}
		return ""
	case controlruntime.EventApprovalRequested:
		p, ok := ev.Payload.(controlruntime.ApprovalView)
		if !ok || p.ID == "" {
			return ""
		}
		var b strings.Builder
		fmt.Fprintf(&b, "需要审批: %s", p.ToolName)
		if cmd, ok := p.Args["command"].(string); ok && cmd != "" {
			fmt.Fprintf(&b, "\n%s", truncateRunes(cmd, 600))
		} else if path, ok := p.Args["path"].(string); ok && path != "" {
			fmt.Fprintf(&b, "\n%s", path)
		}
		fmt.Fprintf(&b, "\n回复 /approve %s 批准一次，/always %s 永久允许，/reject %s 拒绝", p.ID, p.ID, p.ID)
		return b.String()
	case controlruntime.EventQuestionRequested:
		p, ok := ev.Payload.(controlruntime.QuestionView)
		if !ok || len(p.Questions) == 0 {
			return ""
		}
		var b strings.Builder
		b.WriteString("NekoCode 提问:")
		for _, q := range p.Questions {
			fmt.Fprintf(&b, "\n- %s", q.Question)
			for _, opt := range q.Options {
				fmt.Fprintf(&b, "\n    · %s", opt.Label)
			}
		}
		b.WriteString("\n回复 /answer <回答内容> 作答")
		return b.String()
	default:
		return ""
	}
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func (c *Connector) rememberChat(msg inboundMessage) {
	c.mu.Lock()
	c.chats[msg.sourceID()] = chatSession{
		kind:  msg.chatKind,
		id:    msg.chatID,
		msgID: msg.msgID,
		msgAt: time.Now(),
	}
	c.mu.Unlock()
}

func (c *Connector) knownChats() []chatSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Collect(maps.Values(c.chats))
}

func (c *Connector) setPendingQuestion(id string) {
	if id == "" {
		return
	}
	c.mu.Lock()
	c.pendingQuestion = id
	c.mu.Unlock()
}

func (c *Connector) pendingQuestionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pendingQuestion
}

func (c *Connector) clearPendingQuestion(id string) {
	c.mu.Lock()
	if c.pendingQuestion == id {
		c.pendingQuestion = ""
	}
	c.mu.Unlock()
}

func helpText() string {
	return strings.Join([]string{
		"Commands:",
		"  /stop          停止当前任务",
		"  /approve <id>  批准一次工具调用",
		"  /always <id>   批准并永久允许",
		"  /reject <id>   拒绝工具调用",
		"  /answer <内容> 回答进行中的问题",
		"  /help          显示帮助",
		"",
		"群聊中需要 @机器人，私聊直接发送即可。",
	}, "\n")
}
