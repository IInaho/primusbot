# NekoCode Runtime Architecture

> 本文档定义 NekoCode 从“UI 直接调用 Bot”升级为“Bot 核心基座 + Runtime 控制层 + UI/远程交互壳”的目标架构、迁移方案和完成度判断。

## 目标

NekoCode 的长期目标是让底层 Agent 能力既能服务内置 TUI/GUI，也能作为 SDK 或 daemon 被其他业务系统、远程通讯软件和自动化流程集成。

目标形态：

```text
cmd/interaction → runtime core interfaces → bot implementation → foundation
                    ↘ runtime/defaultbot composes the built-in bot
```

其中：

- `bot` 是 Agent 核心基座，提供模型调用、上下文、工具、权限、Hooks、Session 等智能体能力。
- `runtime` 是控制层，负责输入路由、事件流、确认同步、展示查询、连接器生命周期和 SDK/API 暴露；runtime core 只依赖自己声明的 `RuntimeBot` / `GUIBot` 接口。
- `runtime/defaultbot` 是默认装配层，负责把内置 `bot.New()` 注入 runtime；其他业务系统可以直接用 `runtime.NewSessionRuntime(customBot)` 注入自有实现。
- `interaction` 是最外层交互壳，只负责输入输出适配和渲染，不直接耦合 Bot 内部。

## 目标目录层级

```text
cmd/
  nekocode-tui
  nekocode-gui
  nekocode-daemon

interaction/
  tui/
  gui/
    app/
    web/
  connect/
    telegram/
      api.go              # Telegram HTTP API adapter
      config.go           # local profile/pairing config
      connector.go        # connector lifecycle, command/update routing
      qr.go               # terminal pairing QR rendering
      internal/
        taskview/         # Telegram task/status/diff/question rendering
    slack/
    feishu/

runtime/
  protocol.go           # public protocol DTOs and event aliases
  session.go            # public RuntimeBot and SessionRuntime entrypoint
  helpers.go            # public helper aliases and constructors
  management.go         # management/control-plane API for rich interaction surfaces
  internal/
    botcore/            # RuntimeBot/GUIBot interface definitions
    core/               # protocol DTOs and runtime interfaces
    session/            # command/event/query orchestration implementation
    eventbus/           # publish/subscribe and event history
    broker/             # approval and question coordination
    runstore/           # run/artifact read model
    recording/          # events.jsonl and artifact persistence
    connectors/         # connector manager implementation
    redaction/          # sensitive command redaction
  defaultbot/
    defaultbot.go
  httpapi/
  view/
  sdk/

common/
  view/

bot/
  app/
  agent/
  tools/
  contextmgr/
  policy/
  hooks/
  extension/
  command/
  session/
  view/

foundation/
  provider/
  config/
  storage/
  util/
  sandbox/
```

当前项目可以渐进迁移，不要求一次性重排目录。目前已将 `connect/` 迁入 `interaction/connect/`，将 `tui/` 迁入 `interaction/tui/`，并将 GUI 拆入 `interaction/gui/app/` 与 `interaction/gui/web/`；新代码必须遵守依赖方向。

## 分层职责

### Bot Core

`bot` 负责 Agent 核心能力：

- 接收一条用户输入并运行 Agent 循环。
- 构建和压缩上下文。
- 调用模型 provider。
- 执行工具和子 Agent。
- 应用权限策略、Hooks 和沙箱。
- 维护核心 session、token、context、tool 状态。
- 产出核心回调：文本流、reasoning、工具事件、confirm/question、最终结果。

`bot` 不应知道：

- TUI、GUI、Telegram、Slack、飞书、Web。
- Wails events、HTTP、SSE、WebSocket、二维码。
- 多端同步、设备绑定、通讯软件消息格式。
- UI 组件形态或移动端展示策略。

### Runtime Control Layer

`runtime` 是交互控制面，负责把多入口、多 UI 和 Bot 核心隔离开。

runtime core 不直接 import `bot` 包，而是通过本层声明的接口工作：

- `RuntimeBot`：运行、命令、转向、终止、统计、session messages 和回调配置。
- `GUIBot`：在 `RuntimeBot` 之上补充 GUI 需要的配置、context、session、skill/plugin 管理查询。

默认应用入口使用 `runtime/defaultbot.NewSessionRuntime()` 创建内置 bot；SDK 或其他业务系统可以绕过默认装配，直接调用 `runtime.NewSessionRuntime(customBot)`。

职责：

- 提供统一 Command API：`Submit`、`Steer`、`Abort`、`Approve`、`Answer`、`Connect`、`Disconnect`。
- 提供统一 Event API：`Subscribe`，把 Bot callbacks 转换为标准事件流。
- 提供统一 Query API：context、session、stats、run、connect、artifact、memory 等展示视图。
- 管理输入来源：TUI、GUI、Telegram、Web、SDK、daemon API。
- 管理运行状态：idle、running、waiting approval、waiting question、done、failed、aborted。
- 管理审批 broker：任意端批准或拒绝后，同步解除其他端的确认态。
- 管理连接器生命周期：配对、认证、断开、审计。
- 管理 runtime 级 artifacts：diff、review、patch、events.jsonl。
- 对外形成 SDK surface，可被其他业务系统嵌入。

### Presentation / Connect Adapters

UI 和通讯软件只做适配：

- TUI：键盘输入转 `runtime.Input`，runtime event 转 Bubble Tea 渲染。
- GUI：Wails method 转 `runtime.Input`，runtime event 转 Wails events。
- Telegram：Bot API update 转 `runtime.Input`，runtime event 转文本、按钮、附件。
- Web：HTTP/SSE 转 runtime API 和 event stream；WebSocket 属于后续扩展。

这些外壳不直接调用 `bot.Run`、`bot.Steer`、`bot.Abort`，也不直接读取 `bot/contextmgr`、`bot/session` 等内部状态。

## 核心数据流

普通输入：

```text
TUI Enter / GUI Send / Telegram Message / SDK Submit
        ↓
runtime.Submit(Input)
        ↓
InputRouter
        ↓
RunCoordinator
        ↓
bot.Run 或 bot.Steer
        ↓
Bot callbacks
        ↓
runtime.EventBus
        ↓
TUI / GUI / Telegram / Web / SDK subscribers
```

确认流：

```text
Tool needs approval
        ↓
bot confirm callback
        ↓
ApprovalBroker.Create(req)
        ↓
EventBus emits approval_requested
        ↓
TUI ConfirmBar / GUI Dialog / Telegram approval message
        ↓
任意端 Approve 或 Reject
        ↓
ApprovalBroker.Decide()
        ↓
req.Response receives decision
        ↓
EventBus emits approval_resolved
        ↓
所有 UI 同步解除确认态
```

查询流：

```text
UI starts or reconnects
        ↓
runtime.QueryService
        ↓
ContextView / SessionView / StatsView / RunView / ConnectView
        ↓
UI renders snapshot
        ↓
Subscribe receives incremental events
```

## Runtime API

Command API：

```go
Submit(ctx context.Context, input Input) (RunID, error)
Steer(ctx context.Context, runID RunID, input Input) error
Abort(ctx context.Context, runID RunID) error
Approve(ctx context.Context, approvalID string, decision ApprovalDecision) error
Answer(ctx context.Context, questionID string, reply QuestionReply) error
Connect(ctx context.Context, spec ConnectorSpec) error
Disconnect(ctx context.Context, channel string) error
```

实现约束：

- 敏感 runtime command 输入必须先脱敏再进入 `EventInputAccepted`、RunView 和事件持久化；例如 `/connect telegram token ...` 记录为 `/connect telegram token [redacted]`，`/connect telegram add personal ...` 记录为 `/connect telegram add personal [redacted]`。
- 当 run 处于 `waiting_approval` 或 `waiting_question` 时，普通 message 不会被当作 steer；远程端必须通过 `Approve` 或 `Answer` 完成 pending 交互。

Event API：

```go
Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, error)
```

HTTP/SSE adapter 支持 `GET /events?replay=1` 回放当前进程已缓存事件；启用事件记录时，runtime 启动会读取已有 `events.jsonl` 重建 RunView 和内存 event history。

Query API：

```go
CurrentRun(ctx context.Context) (RunView, error)
RunView(ctx context.Context, runID RunID) (RunView, error)
ContextView(ctx context.Context, sessionID string) (ContextView, error)
SessionList(ctx context.Context) ([]SessionView, error)
SessionMessages(ctx context.Context, sessionID string) ([]MessageView, error)
StatsView(ctx context.Context) (StatsView, error)
ConnectView(ctx context.Context) (ConnectView, error)
ArtifactView(ctx context.Context, runID RunID) (ArtifactView, error)
MemoryView(ctx context.Context, scope MemoryScope) (MemoryView, error)
```

## 展示数据归属

UI 所需展示数据由 runtime 对外提供。Bot 提供核心事实，runtime 组装 read model，UI 只消费 runtime view/event。

共享展示 DTO 放在 `common/view`，保持纯数据结构，不 import `bot`、`runtime` 或任意外层交互包。`bot/view` 负责把 bot 内部类型转换成共享 DTO，`runtime/view` 负责向外层暴露 runtime API 使用的 DTO 别名。

| 数据 | 权威来源 | 对 UI 暴露层 |
| --- | --- | --- |
| Context 用量 | `bot/contextmgr` | `runtime.Query.ContextView` |
| Session 列表和消息 | `bot/session` 或 runtime session store | `runtime.Query.SessionList` / `SessionMessages` |
| Token 统计 | `bot/agent/runtime` | `runtime.Query.StatsView` |
| 工具执行状态 | Bot callbacks | `runtime.EventBus` / `RunView` |
| Confirm / Question | Bot callbacks | `runtime.ApprovalBroker` / `QuestionBroker` |
| Connector 状态 | runtime connector manager | `runtime.Query.ConnectView` |
| Diff / Review artifact | runtime artifact store | `runtime.Query.ArtifactView` |
| Memory | bot memory + runtime memory coordinator | `runtime.Query.MemoryView` |

## Command 分类

Runtime command 管理交互控制面，不进入模型上下文：

- `/connect`
- `/disconnect`
- `/devices`
- `/stop`
- `/status`

Bot command 管理 Agent 能力，可以继续由 Bot command parser 处理：

- `/plan`
- `/model`
- `/context`
- `/sessions`
- `/plugin`
- `/summarize`

## Connect 设计原则

`/connect telegram` 的定位是把 Telegram 连接为当前 session 的远程输入和确认设备，而不是另起一套任务平台。

第一阶段：

- Telegram 使用 Bot API long polling，不要求公网 webhook。
- Telegram 管理模型是多 bot profile、单 active profile、单 owner user。
- 常用命令只暴露 `/connect telegram`、`/connect telegram add <bot-token>`、`/connect telegram use <name>`、`/disconnect telegram`、`/connect telegram reset`。
- `/connect telegram` 是智能入口：无 profile 时提示添加 bot；未配对时进入首次配对；已配对时只启动连接并显示状态，不重复二维码。
- `/connect telegram add <bot-token>` 新增或更新 bot profile，profile 名默认从 bot username 自动生成；`/connect telegram add <name> <bot-token>` 作为高级用法保留。
- `/connect telegram use <name>` 切换当前 active profile；同一 NekoCode 实例同时只监听一个 active bot。
- `/connect telegram reset` 清空本地 Telegram 配置。
- 高级命令保留 `/connect telegram profiles`、`/connect telegram pair [name]`、`/connect telegram unpair [name]`、`/connect telegram remove <name>`、`/connect telegram status`。
- `/disconnect telegram` 只停止当前 connector，不删除 token，不解绑 owner。
- TUI/GUI/Telegram 的输入都走 runtime。
- Telegram 普通文本来自 active profile 的 owner；未绑定账号只能收到未连接提示。
- Telegram 可处理 approval/question。
- Telegram approval 和单题单选 question 使用 inline keyboard；复杂 question 继续使用 `/answer` 文本命令。
- Telegram 默认只回 compact mirror，不逐 token 刷屏。
- 远程审批默认只允许 approve once。

后续阶段：

- GUI 也能触发 `/connect telegram`。
- daemon/headless 也能运行 connector。
- 支持 Slack、飞书等 connector。
- 支持 Web/HTTP/SSE 作为 remote API。

## 迁移计划

### Phase 1: Runtime 骨架

- 新增 `runtime` 包。
- 定义 `Input`、`Event`、`RunID`、`Approval`、`Query` 相关类型。
- 实现基础 `EventBus`。
- 实现基础 `ApprovalBroker`。
- 实现依赖 `RuntimeBot` / `GUIBot` 接口的 `SessionRuntime`，默认 bot 由 `runtime/defaultbot` 装配。

### Phase 2: TUI 接入 Runtime

- TUI 不再直接持有 `bot.UI`，改持有 `runtime.Runtime`。
- TUI 的键盘输入走 `Submit` / `Steer`。
- TUI 的渲染来自 `Subscribe` 事件。
- TUI 的 confirm/question 通过 broker 决策。

### Phase 3: Telegram Connector

- 新增 Telegram connector。
- 实现 token setup、deep link QR pairing、long polling。
- Telegram update 转 runtime input。
- runtime events 转 Telegram compact replies。
- Telegram approval/question 决策接入 broker。

### Phase 4: GUI 接入 Runtime

- GUI 后端从 `bot.GUI` 迁移到 runtime。
- Wails events 由 runtime events 转换。
- GUI query 全部走 runtime QueryService。
- 删除 GUI 自己维护的 confirmation map 或迁移到 runtime broker。

### Phase 5: SDK / Daemon

- 暴露 Go SDK。
- 增加 `nekocode-daemon`。
- 增加 HTTP/SSE/WebSocket adapter。
- Connector 可在无 TUI/GUI 的情况下长期运行。

## 完成度判断

### 架构完成

- `interaction/*` 不再 import `nekocode/bot`。
- 所有用户输入都走 `runtime.Submit`、`runtime.Steer`、`runtime.Abort`。
- 所有展示数据都走 `runtime.Query` 或 `runtime.Subscribe`。
- Confirm/question 任意端处理后，其他端同步更新。
- Bot 层不知道 TUI、GUI、Telegram、HTTP、二维码或 Wails。

### 功能完成

- TUI 行为与现有版本一致。
- GUI 行为与现有版本一致。
- Telegram 发消息能驱动当前 session。
- Telegram 运行中发消息等价于 steer。
- Telegram approve/reject 能解除 TUI/GUI 的确认态。
- Context、session、token、tool、diff/review 都能通过 runtime 查询。

### SDK 完成

- Go SDK 示例能创建 runtime、提交输入、订阅事件、处理审批。
- Daemon API 能通过 HTTP/SSE 控制同一套 runtime。
- Event payload 不包含 TUI/GUI 专用字段。
- Connector adapter 可以脱离 TUI/GUI 运行。

当前 daemon HTTP demo 已提供：

- `POST /input`
- `GET /runs`、`GET /runs/current`、`GET /runs/{runID}`、`GET /runs/{runID}/artifacts`
- `GET /events?replay=1`
- `GET /stats`、`GET /context`、`GET /sessions`、`GET /sessions/current/messages`
- `GET /connect`、`POST /connect/{name}`、`POST /disconnect/{name}`
- `POST /approvals/{approvalID}`、`POST /questions/{questionID}`、`POST /runs/{runID}/abort`

### 可靠性完成

- UI 重连后能恢复当前 run/context/session view。
- Connector 断线不会影响本地 TUI/GUI 执行。
- 输入规则明确：idle 开新 run，running 变 steer，confirming 只处理 approval/question。
- Approval 绑定 tool call、args hash、source user，并且只能消费一次。
- Runtime 关键事件可持久化或可从 Bot/session/artifact 重建。

## 最终验收标准

最终验收标准：

```text
任何前端或通讯软件都只是 runtime 的客户端；
bot 只负责智能体能力；
runtime 提供统一控制、状态、事件、展示数据和 SDK 集成面。
```
