# NekoCode 架构文档

> **本文档职责**: 描述项目架构——目录结构、包依赖、模块实现、代码层面的机制。不包含 UI 设计、交互设计、设计原则等属于 DESIGN.md 的内容。

## 项目概述

NekoCode 是一个基于 Go 的终端 AI 助手，使用 Bubble Tea v2 构建 TUI，支持多 LLM provider（OpenAI 兼容 / Anthropic 兼容协议），具备 Agent 循环、Native Function Calling、工具执行、权限确认、Plan Mode、Plugin 系统、事件驱动 Hooks、MCP 客户端、子 Agent、上下文管理、项目 Memory、AI 文生图等机制。

## Bot 层目标结构

Bot 层按子系统组织，目录结构和依赖方向都应接近树状：上层负责装配，下层负责能力实现；子系统之间通过窄接口交互，避免横向穿透和反向依赖。

目标分层：

```
bot/
├── bot.go               # 对外 Bot API + 生命周期装配入口
├── bot_run.go           # RunHost、运行与命令执行
├── bot_services.go      # 模型、配置、上下文、指标领域查询
├── bot_session.go       # 会话领域操作
├── bot_extension.go     # 扩展领域操作
├── bot_subagent.go      # 子 Agent 接线
├── agent/               # Agent 主循环：turn、LLM 调用、工具反馈、停止条件
├── agent/subagent/      # 子 Agent 执行引擎
├── contextmgr/          # 上下文、压缩、memory、token 统计
├── provider/            # LLM 协议、客户端工厂、stream/http 类型
├── tools/               # 工具定义、注册、执行（builtin/ 实现 + runtime/ 执行编排）
├── policy/              # 策略系统：Policy、Hook 引擎（Registry/builtin/plugin）、ledger、budget、tool semantics
├── extension/           # plugin、skill、mcp 扩展实现与管理
├── command/             # slash command 注册和生命周期命令
├── session/             # 会话持久化
├── prompt/              # system/plan prompt 构建
└── config/
```

依赖规则：

- `bot` 根包是装配层，可以依赖各子系统；其他子系统不能依赖 `bot` 根包。
- `bot/**` 不依赖 `runtime` 或展示层；Bot 只公开领域对象和
  `protocol` 中的中立运行契约。
- `runtime/standard` 是完整 Bot 到 runtime 协议的唯一适配边界，view DTO
  的投影集中在其 `internal/viewmodel`。
- `agent` 只依赖 LLM、context、tools、policy 等运行时接口，不关心 plugin/skill/mcp 的安装和发现。
- `tools` 只定义和执行工具，不反向依赖 agent 主循环或 `agent/subagent`；需要委托子 Agent 时通过 `TaskRunner` 接口接线，具体适配器放在 `bot` 根包。
- `agent/subagent` 不依赖 `agent` 主循环；经 `RunConfig.Policy` 注入主 agent 的 Policy，共享治理账本与探索预算。
- `extension.Manager` 是扩展系统唯一的高层入口；`bot` 不直接持有 plugin/skill/mcp 子 Manager。
- `plugin` 只管理插件清单、安装和启停状态；扩展激活由 `extension.Manager` 统一编排。
- `mcp.Manager` 同时拥有 server 和对应工具的生命周期，外层不手工同步工具切片。
- `extension.Manager` 直接满足 `command.SkillProvider`，`bot` 不增加 Skill provider 包装层。
- 无引用的 alias 包应删除；内部新代码应直接依赖真实实现包。

## 目录结构

```
nekocode/
├── main.go                         # Wails GUI 入口 + interaction/gui/web/dist embed
├── wails.json                      # Wails 构建配置
├── cmd/
│   ├── tui/
│   │   └── main.go                 # TUI 程序入口
│   └── daemon/
│       └── main.go                 # HTTP API daemon 入口
├── interaction/gui/app/            # Wails 后端桥接实现
│   ├── app.go                      #   App 结构体 + 事件推送
│   ├── appicon.icns                #   macOS 应用图标（多分辨率）
│   └── appicon.ico                 #   Windows 应用图标（多分辨率）
├── interaction/gui/web/            #   Wails 前端（React + Vite）
│   ├── index.html
│   ├── src/
│   │   └── components/
│   │       └── LogoMark.tsx        #   Logo 组件（猫娘图标，32×32 viewBox）
│   └── public/
│       └── logo/                   #   品牌资产源文件
│           ├── logo.svg             #     完整品牌版（含 NEKOCODE 字）
│           ├── logo-icon.svg        #     纯图标源
│           ├── logo-icon-app.svg    #     favicon
│           ├── appicon.icns         #     macOS 导出
│           ├── appicon.ico          #     Windows 导出
│           ├── icon_16..1024.png    #     中间产物
│           └── build_icns.js        #     任意平台生成 ICNS 的脚本
├── protocol/                       # Bot/runtime 共用的中立运行契约
├── logger/                         # 项目诊断日志
├── util/                           # 通用工具包（duration / fs / http / registry / sse / text / url / yaml / version / tui_snapshot）
├── runtime/                        # 交互控制层（TUI/GUI/HTTP/connector 的统一入口）
│   ├── runtime.go                  #   入口：Manager + New(Runner)
│   ├── runner.go                   #   必需 Runner/RunHost 执行协议
│   ├── runtime_services.go         #   可选能力接口
│   ├── runtime_run.go              #   单次 run host 与事件转换
│   ├── runtime_control.go          #   start/steer/cancel/approval/question
│   ├── runtime_events.go           #   事件、录制与 connector 通道
│   ├── runtime_status.go           #   状态、查询、指标与能力发现
│   ├── protocol.go                 #   稳定 DTO、状态常量与协议错误
│   ├── agentrunner/                #   通用 Agent 到 Runner 的适配
│   ├── standard/                   #   标准应用 runtime 组装
│   │   ├── adapter.go              #   Bot 领域对象 → runtime 协议
│   │   ├── internal/viewmodel/     #   Bot 领域对象 → runtime DTO
│   │   └── standard.go             #   New(): Bot + 事件录制 + connector
│   ├── httpapi/                    #   HTTP/SSE API server
│   └── internal/                   #   内部实现
│       ├── core/                   #     protocol/interaction/views 契约源
│       ├── broker/                 #     approval/question 同步交互
│       ├── eventbus/               #     事件总线
│       ├── runstore/               #     run 状态存储
│       ├── recording/              #     事件录制
│       └── connectors/             #     connector 注册表
├── bot/                            # 核心逻辑
│   ├── bot.go                      #   入口：Bot + New()
│   ├── bot_run.go                  #   运行、命令与 RunHost 回调转发
│   ├── bot_services.go             #   配置、模型、指标、上下文与 Memory 服务
│   ├── bot_extension.go            #   extension 组装、生命周期与领域操作
│   ├── bot_session.go              #   session 状态编排与领域操作
│   ├── bot_subagent.go             #   task tool 到 subagent engine 接线
│   ├── agent/                      #   Agent 循环
│   │   ├── agent.go                #     入口：Agent + New(Config)
│   │   ├── loop.go                 #     Run 主循环
│   │   ├── agent_turn.go           #     单轮策略与结果处理
│   │   ├── model.go                #     LLM 调用、流回调与响应分类
│   │   ├── tools.go                #     工具过滤、执行、反馈与子代理回调
│   │   ├── agent_state.go          #     run、stream 与 token 状态
│   │   ├── slots.go                #     子代理槽位
│   │   ├── internal/kernel/        #     通用循环、生命周期与重试门
│   │   ├── internal/llmstream/     #     LLM 流式调用与工具调用解析
│   │   ├── subagent/               #     子 Agent 系统
│   │   │   ├── subagent.go         #       入口、运行循环与结果
│   │   │   ├── subagent_engine.go  #       上下文、推理与工具执行
│   │   │   ├── registry.go         #       内置/插件 Agent 与 AgentMD
│   │   │   ├── safety.go           #       结果安全审核
│   │   │   └── prompts/            #       子 Agent prompt 模板
│   │   │       ├── executor.md     #         executor prompt
│   │   │       ├── researcher.md   #         researcher prompt
│   │   │       └── verify.md       #         verify prompt
│   ├── config/                     #   配置管理
│   │   └── config.go               #     Config + Load()
│   ├── command/                    #   斜杠命令系统
│   │   ├── command.go              #     Handler 入口
│   │   ├── parser.go               #     命令解析与注册
│   │   └── lifecycle.go            #     ForceSummarize / ContextReport
│   ├── contextmgr/                 #   上下文管理
│   │   ├── contextmgr.go           #     入口：Manager + Config + New
│   │   ├── contextmgr_context.go   #     Build 管线、设置与 token 用量
│   │   ├── contextmgr_history.go   #     消息存取与截断
│   │   ├── contextmgr_compaction.go #    压缩控制
│   │   ├── compression.go          #     私有压缩阈值、预算与摘要函数
│   │   ├── replacement.go          #     摘要替换压缩实现
│   │   ├── contextmgr_snapshot.go  #     Snapshot / Restore
│   │   ├── contextmgr_report.go    #     上下文诊断报告
│   │   ├── content.go              #     私有上下文内容与分层构建
│   │   ├── memory/                 #     五段式项目 Memory 只读加载
│   │   └── token/                  #     Token 估算
│   │       ├── estimate.go         #       启发式估算
│   │       └── tracker.go          #       API 校准追踪
│   ├── provider/                   #   LLM 抽象层
│   │   ├── types/                  #     核心类型定义（Message/Response/ToolDef + HTTP 客户端）
│   │   ├── anthropic/              #     Anthropic Messages API 兼容实现
│   │   ├── openai/                 #     OpenAI Chat Completions 兼容实现（DeepSeek / MiniMax 等）
│   │   ├── provider.go             #     入口：LLM 接口、Config 与 New 工厂
│   │   └── retry.go                #     指数退避重试
│   ├── policy/                    #   治理引擎 + 治理语义
│   │   ├── policy.go              #     唯一入口：Policy 构造、生命周期、快照
│   │   ├── policy_tools.go        #     BeforeTool / RecordTool(s)
│   │   ├── events.go              #     Turn / ToolRequest / ToolResult / Facts
│   │   ├── hook.go                #     Hook / State / Result 对外类型
│   │   ├── hooks.go               #     Hook 注册、私有状态、评估
│   │   ├── audit.go               #     Hook 审计事件与输出格式化
│   │   ├── plugin.go              #     LoadPluginHooks（声明式 hooks）
│   │   ├── builtin/               #     内置 Hook 实现
│   │   │   ├── all.go             #       All() 列表
│   │   │   ├── quota_rules.go     #       QuotaHook
│   │   │   ├── tool_rules.go      #       工具安全与工具结果 guardrail
│   │   │   ├── exploration_rules.go#      探索预算与探索防护
│   │   │   ├── progress_rules.go  #       防卡进度规则
│   │   │   ├── verification_rules.go #    验证规则
│   │   │   └── garbled_rules.go   #       乱码响应熔断
│   │   ├── plugin/                #     声明式 Hook 实现
│   │   │   ├── types.go           #       Point / Event / Hint / Result / Hook 类型
│   │   │   ├── config.go          #       配置加载
│   │   │   ├── schema.go          #       JSON Schema
│   │   │   ├── hook.go            #       Hook 执行
│   │   │   ├── matcher.go         #       Tool name matcher
│   │   │   └── runner.go          #       命令执行 + 超时
│   │   ├── ledger/                #     工具执行账本
│   │   │   ├── ledger.go          #       入口：事件记录、turn 状态、快照
│   │   │   └── paths.go           #       shell 读写路径提取
│   │   ├── exploration/           #     探索分数
│   │   └── semantics/             #     工具语义分类
│   ├── extension/                  #   扩展系统
│   │   ├── extension.go            #     入口：Manager（插件/Skill/Hook/Agent/MCP 生命周期）
│   │   ├── extension_commands.go   #     /plugin 命令与安装交互
│   │   ├── mcp/                    #     MCP 客户端
│   │   │   ├── mcp.go              #       入口：Manager（server + tool 生命周期 + 健康状态）
│   │   │   ├── client.go           #       client：进程连接句柄（Start/Close/ListTools/CallTool）
│   │   │   ├── types.go            #       ServerConfig 等类型定义
│   │   │   └── tool.go             #       mcpTool 适配器
│   │   ├── plugin/                 #     Plugin 系统
│   │   │   ├── plugin.go           #       入口：Plugin 核心类型 + Manager（清单与状态）
│   │   │   ├── plugin_commands.go  #       插件命令展示与安装预览
│   │   │   ├── registry.go         #       registry（存储 + 状态持久化）
│   │   │   ├── install.go          #       安装获取（git clone / 本地复制）
│   │   │   ├── manifest.go         #       Manifest 解析（plugin.json）
│   │   │   ├── source.go           #       源解析 / env 展开 / 远程获取
│   │   │   └── format.go           #       插件展示文本
│   │   └── skill/                  #     Skill 系统
│   │       ├── skill.go            #       入口：Skill 核心类型 + Manager（加载、上下文刷新、skill tool 接线）
│   │       ├── registry.go         #       skill registry（包内）
│   │       ├── load.go             #       获取链：目录发现 → 读取 → 解析 → 内置技能（go:embed）
│   │       ├── format.go           #       格式化
│   │       ├── tool.go             #       技能工具适配
│   │       └── bundled/            #       内置技能文件
│   ├── prompt/                     #   System Prompt 构建
│   │   ├── prompt.go               #     入口：Builder + New
│   │   ├── system_zh.md            #     中文 System Prompt 模板
│   │   ├── env.go                  #     环境信息格式化
│   │   └── os_release.go           #     OS 检测
│   ├── session/                    #   Session 管理
│   │   ├── session.go              #     入口：Manager + New(cwd)
│   │   └── snapshot.go             #     Snapshot / Meta 持久化与 context 映射
│   └── tools/                      #   工具系统
│       ├── tools.go                #     入口：工具注册表
│       ├── builtin/                #     内置工具实现
│       │   ├── catalog/            #       Toolbox 工具与生命周期
│       │   ├── filesystem/         #       read/write/edit/list/tree/glob/grep
│       │   ├── shell/              #       Bash 执行 + 危险分级
│       │   ├── web/                #       web_search / web_fetch / html2md
│       │   ├── media/              #       image_gen（即梦文生图）
│       │   ├── task/               #       子 Agent 任务工具
│       │   ├── todo/               #       todo_write 工具
│       │   ├── question/           #       question 工具
│       │   ├── diff/               #       diff 工具
│       │   └── index/              #       代码索引工具（条件注册）
│       └── runtime/                #     工具执行编排
│           ├── core/               #       Tool 接口 + ToolCallItem/Result + Descriptor
│           ├── runner/             #       执行引擎（单工具/批量/预览/权限）
│           ├── execution/          #       执行状态 + 缓存 + 文件快照
│           ├── permission/         #       权限确认
│           ├── sandbox/            #       沙箱隔离
│           ├── workspace/          #       工作区访问控制
│           ├── taskbridge/         #       task 工具到 subagent 的桥接
│           └── toolutil/           #       路径/文本等通用工具
├── interaction/connect/            # 外部 IM connector
│   ├── connect.go                  #   入口：connector 基座和事件分发
│   ├── pairing.go/store.go         #   配对状态与 connect.json 分段配置
│   ├── commands.go/stream.go       #   共享指令与流式节流
│   ├── telegram/                   #   Telegram connector（polling + pairing + taskview 渲染）
│   ├── feishu/                     #   飞书 connector（Lark SDK WS 长连接 + DM 配对，MVP）
│   └── qqbot/                      #   QQ connector（腾讯官方机器人平台：Gateway WS + v2 消息 API，纯文本）
├── interaction/interaction.go      # 多种交互端共享的工具参数展示格式
├── interaction/tui/                # TUI 交互界面
│   ├── tui.go                      #   package tui 入口（Run 函数）
│   ├── agent.go                    #   runtime 事件到 TUI 消息的投影
│   ├── model.go                    #   Model 结构体
│   ├── update.go                   #   Update() 消息分发
│   ├── view.go                     #   View() 视图布局组装
│   ├── handlers.go                 #   按键处理
│   ├── helpers.go                  #   token 统计文案
│   ├── types.go                    #   状态枚举 + 消息类型
│   ├── components/                 #   UI 组件
│   │   ├── block/                  #     内容块渲染
│   │   │   ├── block.go            #       Block 结构体 + Done 字段
│   │   │   ├── block_render.go     #       渲染逻辑
│   │   │   └── block_tool.go       #       工具块 + edit 预览渲染
│   │   ├── message/                #     消息项渲染
│   │   │   ├── message.go          #       Message 结构体
│   │   │   ├── message_assistant.go#       助手消息渲染
│   │   │   ├── message_user.go     #       用户消息渲染
│   │   │   ├── message_system.go   #       系统消息渲染
│   │   │   ├── message_error.go    #       错误消息渲染
│   │   │   ├── message_shared.go   #       共享 helper
│   │   │   └── markdown.go         #       Markdown 渲染（段落级分离）
│   │   ├── processing/             #     处理中状态渲染
│   │   │   ├── processing.go       #       Processing 结构体
│   │   │   ├── processing_render.go#       渲染逻辑
│   │   │   └── render_text.go      #       文本渲染
│   │   ├── messages.go             #     消息列表
│   │   ├── input.go                #     输入框
│   │   ├── header.go               #     顶部状态栏
│   │   ├── splash.go               #     启动页
│   │   ├── confirm_bar.go          #     确认栏
│   │   ├── list_widget.go          #     列表组件
│   │   ├── suggestions.go          #     命令补全
│   │   └── scrollbar.go            #     滚动指示器
│   └── styles/                     #   样式
│   │   ├── colors.go               #     色彩体系
│   │   └── charset.go              #     制表符字符集
```

## Runtime 控制契约

`runtime.New(runner)` 只要求一个 `Runner`：`Run(context.Context, input, RunHost)`。流输出、工具/子 Agent 事件、phase、todo、approval 和 question 都经本次调用独占的 `RunHost` 传递；命令在一次同步生命周期中返回完整结果。

`Manager` 是 UI/HTTP/connector 的唯一交互入口，核心方法为 `StartRun`、`SteerRun`、`CancelRun`、`Status`、`LookupRun`、`Events`、`DecideApproval` 和 `AnswerQuestion`。`Commander`、`Steerer`、`MetricsProvider`、`ModelService`、`ContextService`、`ExtensionService`、`ConfigurationService`、`SessionService` 均为 Runner 侧可选能力；UI 先读 `Capabilities()`，再直接调用同一个 Manager 的只读方法，不接触底层 service 实例。关闭统一使用 `Close() error`，connector、Runner 和 recorder 的错误不会被吞掉。

事件协议版本为 `2.0`，每个事件携带单调 `sequence`；订阅可用 `EventFilter.After` 续传。协议错误使用稳定 `ErrorCode`。标准应用由 `runtime/standard.New()` 装配完整 Bot、事件录制和 connector；`examples/web-assistant` 则展示只组合 web 工具的独立应用。

上层应用、第三套 UI 和自定义 connector 的实现教程见 [RUNTIME_APP_GUIDE.md](RUNTIME_APP_GUIDE.md)。

## Bot 应用层（bot/）

`bot/` 根包是核心依赖注入和生命周期编排层。`Bot` 结构体持有所有子系统引用，通过 `New()` 按顺序初始化：

```
New()
  ├── initConfig()        → config.Load() + prompt.New()
  ├── initCtxMgr()        → contextmgr.New() + index.Apply()
  ├── initSession()       → session.New(cwd)
  └── rebuildRuntime()
      ├── initToolRegistry()     → catalog.NewToolbox() + 条件工具
      ├── initPolicy()           → policy.New() + builtin.Register()
      ├── initExtensions()       → extension.New(Config).Load() + config MCP
      ├── initAgent()            → provider.New(Config) + agent.New(Config)
      ├── ctxMgr.SetCompactionModel() → 配置摘要与归档合并模型
      └── initCommands()         → command.RegisterAll() + session/export/plugin 命令
```

## 核心架构：Agent 循环

`bot/agent/agent.go` 是 L2 Agent 核心入口，公开构造使用 `agent.New(ctx, agent.Config{...})`。

```
用户输入
  │
  ▼
Run() 主循环 → runTurn(state)
  │
  ├─ AutoCompactIfNeeded() 看门狗
  ├─ Policy.BeginTurn()：重置 turn 事件、计算配额、发布事实
  ├─ drainSteering() 排空中途输入
  │
  ├─ Reason(state) → ReasoningResult
  │   ├─ phase(PhaseThinking)
  │   ├─ ctxMgr.Build(true) 组装上下文（全部消息，不再截断）
  │   ├─ Policy.BeforeModel() → request-scoped hints
  │   ├─ callLLMForTool() 流式调用
  │   └─ withRetry() 指数退避重试
  │
  ├─ [工具调用] executeAndFeedback(calls, reasoning, state)
  │   ├─ Policy.BeforeTool()：配额 + PreToolUse 决策
  │   ├─ 工具执行
  │   └─ Policy.RecordTools()：账本 + 探索分数 + PostToolUse/PostToolBatch
  │
  ├─ [文本响应] handleText(reasoning, state)
  │   ├─ Emit garbled/chat Turn
  │   └─ Policy.BeforeStop() → 接受、阻止或要求修正最终回答
  │
  └─ synthesizeAndReturn() 兜底总结
```

Agent 循环硬限制：
- `maxAgentSteps = 150`：最大迭代步数
- `maxConsecutiveHints = 3`：连续纯文本提示上限
- `maxConsecutiveFailures = 5`：连续 LLM 调用失败上限
- `maxFinalCheckHints = 2`：最终检查重试上限

### Agent 相关包结构

| 路径 | 职责 |
|------|------|
| `bot/agent/agent.go` | Agent 入口、配置与生命周期 |
| `bot/agent/internal/kernel/` | Agent 循环核心控制流（Loop/RunLoop、Lifecycle、Gate），零业务依赖 |
| `bot/agent/internal/llmstream/` | LLM 流式调用 + 工具调用解析 + 重试 |
| `bot/agent/subagent/` | 子 Agent 引擎 + 注册表 + 安全审核 |
| `bot/policy/` | Policy：组合 hook、ledger、exploration 与 quota 的唯一入口 |
| `bot/policy/ledger/` | 工具执行账本：readFiles / modifiedFiles / blockedTools / verifications |
| `bot/policy/exploration/` | 并发安全的探索分数 |

## 上下文管理

### 五级预警阈值

| Level | 剩余 buffer | 动作 |
|-------|------------|------|
| Normal | > 44,800 | 无 |
| Warning | ≤ 44,800 | 告警 |
| MicroCompact | ≤ 35,200 | 微压缩 |
| Compact | ≤ 25,600 | 完整压缩 |
| Blocking | ≤ 6,400 | 拒绝 |

### Build 管线

1. Layer 0: SystemPrompt + Skills（静态前缀）
2. Layer 0: Memory（项目记忆，内容通过 content.Content.Memory 字段承载）
3. Layer 0.5: Archive（压缩摘要）
4. Layer 1: Messages（全部保留，不再截断；Compactor 负责压缩）
5. Layer 2: Todo + Hints（动态层）

### Manager 关键方法

| 方法 | 说明 |
|------|------|
| `Build()` | 组装完整消息列表（含孤儿过滤） |
| `New(Config)` | 使用同一配置协议创建主 Agent 或子 Agent 上下文 |
| `AutoCompactIfNeeded()` | 自动压缩看门狗 |
| `Summarize()` | 手动触发完整压缩 + Archive 合并 |
| `Snapshot() / Restore()` | 会话持久化 |
| `FreshStart()` | 清空所有消息 |

### Session 持久化实现

保存链路（`bot/bot_run.go` `runAgent`）：

```
ag.Run()  →  ctxMgr.SetSystemPrompt()  →  ctxMgr.Summarize()（按需）  →  saveSession()
```

- `bot.saveSession()` 显式收集 Context、token、Skill 和 ledger，再交给
  `session.Manager.Save(snapshot)` 落盘到 `~/.nekocode/sessions/<id>/session.json`。
  Session 包不通过回调访问 Agent、Extension 或 Policy。
- `Snapshot()`（`bot/contextmgr/contextmgr_snapshot.go`）**深拷贝** `Messages` 后再返回，避免后续 `append` 共享 backing array 导致已捕获内容被覆盖
- 触发时机：每次 agent run 结束自动保存；`/sessions`、`/export`、`/new` 等命令也会触发

Agent 结束时的文本处理（`bot/agent/loop.go` `finishRun`）：

- `finalText` 优先，为空时回退到 `lastText`
- 仅在真正无文本产出时才调 `Synthesize()`（追加的总结消息是最后手段）
- `applyFinalPolicyBlock`（`bot/agent/agent_turn.go`）会同步设 `finalText = lastText`，避免无谓的 `Synthesize` 追加

> **历史 bug**: `Snapshot()` 曾只拷贝 slice 头，叠加 `applyFinalPolicyBlock` 只设 `lastText` 不设 `finalText`，导致 `finishRun` 误走 `Synthesize` 路径，保存了一条用户没见过的假摘要覆盖真实最后内容。已通过深拷贝 + `finalText`/`lastText` 同步修复。

### Tool 接口

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() []Parameter
    ExecutionMode(args map[string]any) ExecutionMode
    Execute(ctx context.Context, args map[string]any) (string, error)
}
```

### 工具注册

`bot/tools/builtin/catalog/toolbox.go` 中的 `Toolbox` 注册内置工具（shell/process/read/write/list/tree/glob/edit/grep/web_search/web_fetch/question/todo_write/task/diff/index）。`image_gen` 按配置条件注册；Extension 加载后注册 `skill` 工具。`bot/bot.go` 只负责工具注册表、Extension manager 和 command parser 的组装。

### 内置工具

| 工具 | 模式 | 危险等级 | 位置 |
|------|------|----------|------|
| shell | Sequential | 智能分级（Safe～Forbidden），多层沙箱隔离 | `bot/tools/builtin/shell/` |
| process | Sequential | Safe，管理既有托管进程 | `bot/tools/builtin/shell/` |
| read | Parallel | Safe | `bot/tools/builtin/filesystem/read/` |
| write | Sequential | Write | `bot/tools/builtin/filesystem/write/` |
| edit | Sequential | Write（oldString/newString 内容锚定 + gofmt lint） | `bot/tools/builtin/filesystem/edit/` |
| list | Parallel | Safe | `bot/tools/builtin/filesystem/list/` |
| glob | Parallel | Safe | `bot/tools/builtin/filesystem/search/` |
| grep | Parallel | Safe | `bot/tools/builtin/filesystem/search/` |
| web_search | Parallel | Safe | `bot/tools/builtin/web/` |
| web_fetch | Parallel | Safe | `bot/tools/builtin/web/` |
| question | Sequential | Safe | `bot/tools/builtin/question/` |
| diff | Parallel | Safe | `bot/tools/builtin/diff/` |
| task | Parallel | Safe | `bot/tools/builtin/task/` |
| todo_write | Sequential | Safe | `bot/tools/builtin/todo/` |
| tree | Parallel | Safe | `bot/tools/builtin/filesystem/tree/` |
| index | Parallel | Safe（条件注册） | `bot/tools/builtin/index/` |
| image_gen | Sequential | Safe（条件注册） | `bot/tools/builtin/media/` |
| skill | Parallel | Safe（动态注册） | `bot/extension/skill/` |

### 工具系统子包

| 子包 | 职责 |
|------|------|
| `builtin/catalog/` | Toolbox 工具组装与生命周期 |
| `builtin/filesystem/{read,write,edit,list,tree,search}/` | 文件系统工具 |
| `builtin/shell/` | Shell 执行、托管进程、事件式等待 + 危险分级 |
| `builtin/web/` | Web 搜索/抓取/HTML2MD |
| `builtin/media/` | 图片生成（即梦文生图） |
| `builtin/task/` | 子 Agent 任务工具 |
| `builtin/todo/` | Todo 管理工具 |
| `builtin/question/` | 用户提问工具 |
| `builtin/diff/` | Diff 工具 |
| `builtin/index/` | 代码索引工具（条件注册） |
| `runtime/core/` | Tool 接口 + ToolCallItem/Result + Descriptor |
| `runtime/runner/` | 工具执行引擎（单工具/批量/预览/权限） |
| `runtime/execution/` | 执行状态 + 缓存 + 文件快照 |
| `runtime/permission/` | 权限确认 |
| `runtime/sandbox/` | 沙箱隔离 |
| `runtime/workspace/` | 工作区访问控制 |
| `agent/internal/llmstream/` | LLM 流式调用 + 工具调用解析 |
| `runtime/taskbridge/` | task 工具到 subagent 的桥接 |
| `runtime/toolutil/` | 参数、路径与文本等无状态工具函数 |

## Hook 系统（事件驱动）

### 六种触发点

| Point | 时机 | 典型作用 |
|-------|------|---------|
| UserSubmit | 用户输入进入本次运行后 | 校验输入、注入运行级提示 |
| PreModel | 每次模型请求前 | 注入仅对本次请求生效的系统提示 |
| PreToolUse | 单个工具执行前 | 允许或阻止该工具调用 |
| PostToolUse | 单个工具执行后 | 根据单次结果追加提示或停止 |
| PostToolBatch | 一批工具全部执行后 | 根据整批活动和进度作出决策 |
| Stop | 模型准备返回最终文本时 | 接受、阻止或要求修正最终回答 |

### 事件与状态

Runtime 只向 `Policy` 提交 `Turn`、`ToolRequest`、`ToolResult` 和
`TurnResult`。Policy 组合 ledger 与 exploration，生成只读 `Facts` 供
hook 判断。Hook 的计数器和去重标记按 hook 名隔离，不能读写其他规则或
runtime 的内部字段。

### 内置 Hook（9 个）

| Hook | Point | 功能 |
|------|-------|------|
| quota | PreModel | 读取配额不足时告警，引导优先实质性修改 |
| tool_result_guardrail | PreModel | 工具结果过多时注入 request-scoped 提醒 |
| read_before_write | PreToolUse | edit/write 前检查目标文件读取记录 |
| read_only_spiral | PostToolBatch | 连续只读探索后提醒综合发现并停止继续读 |
| verification | Stop | 有未完成任务但本轮无工具调用时提醒继续 |
| exploration_exhausted | PreModel | 探索调用 ≥10 且分数耗尽时提醒收敛 |
| explore_cascade | PostToolBatch | 本轮启动 ≥4 个 researcher 时提醒综合信息 |
| progress_stall | PostToolBatch | 连续多轮无进展后提醒推进或报告阻塞 |
| garbled_circuit_breaker | Stop | 累计 5 次 garbled 工具调用则强制停止 |

## Plugin 系统

`bot/extension/`：
- 唯一高层入口是 `Manager`（`extension.go`）：`New(Config{Context, Tools, Policy, ContextWindow})` 后调用 `Load()`
- 统一激活和停用插件提供的 Skills / Agents / Hooks / MCP Servers
- 插件安装、启停和 Reload 都经过同一条生命周期；Skill 每次只按当前启用插件集合重载一次
- 通过 `Snapshot()` 一次返回管理视图状态，并提供 Skill 查询、插件启停等精简子方法

`bot/extension/plugin/`：
- 入口是 `Manager`（`plugin.go`）：只拥有 registry、安装管线和启停状态
- 安装源：GitHub URL / user:repo / 本地路径
- 扩展点：Skills / Agents / Hooks / MCP Servers
- manifest / registry / 安装源模型
- 插件自身不注册 Agent、Hook、MCP 或 Skill，不持有宿主回调
- 不依赖 Runtime DTO：GUI/API 管理快照由 `runtime/standard` 投影

## 声明式 Hooks

`bot/policy/plugin/`（`LoadPluginHooks`）：
- 事件类型：PreToolUse / PostToolUse / PostToolUseFailure / UserPromptSubmit / SessionStart / Stop（6 种）
- JSON 配置（hooks.json）
- Tool name matcher（`|` 分隔，regex 支持）
- 命令执行 + 超时
- 支持 `Once` 标记（仅首次触发）

## MCP 客户端

`bot/extension/mcp/`：
- 唯一入口是 `Manager`（`mcp.go`）：`New(toolRegistry)`；`Add`/`Remove`/`Close` 同时管理 server 和工具
- JSON-RPC 2.0 协议
- Server 生命周期管理（启动/初始化/tool 列举/关闭）
- `tools.Tool` 接口适配（包内 `mcpTool`）
- stable owner ID 与展示名称分离；config MCP 优先，同名插件 MCP 会被拒绝

## Skill 系统

`bot/extension/skill/`：
- 唯一入口是 `Manager`（`skill.go`）：`New(contextManager, toolRegistry, contextWindow)`；registry 与 tool 适配器均为包内实现
- YAML 格式技能定义
- 目录发现 + 加载
- 内置技能通过 `bundled/` go:embed
- `skill` 工具动态注册到 toolRegistry
- `Load(pluginDirs)` / `Reload(pluginDirs)` 显式接收插件 Skill 目录，不持有目录回调
- 不依赖 Runtime DTO：GUI/API 管理快照由 `runtime/standard` 使用
  `Manager.Snapshot()` 投影

## 子 Agent 系统

### 内置类型（3 种）

| Agent | 用途 | 工具 | 特殊配置 |
|-------|------|------|---------|
| executor | 执行代码修改 | read/write/edit/bash/grep/glob/list/question | — |
| verify | 验证修改 | read/grep/glob/list/bash | — |
| researcher | 代码探索/调研 | read/grep/glob/list/web_search/web_fetch/question | OmitProjectContext: true |

### Engine 特性

- 独立 ctxmgr（`New(Config)`），配置 merge model 时自动接入 Compactor
- FileCache 从主 Agent 种子预热（Seed/Merge）
- 上下文窗口、Thinking 开关等参数从主 Agent 配置继承
- 安全审核（关键词匹配 + 敏感路径检测）
- DisableThinking 默认关闭，researcher 支持 Thoroughness 深度控制
- Handoff 上下文注入（`<handoff>` 块追加到 system prompt）
- ConfirmFn 覆盖（edit 操作需用户确认）
- Partial result 恢复（中断/错误时返回部分结果）
- Metadata 追踪（totalTokens、toolUseCount、durationMs、cacheHitTokens、cacheMissTokens）
- Phase 回调（cfg.OnPhase 通知阶段变化）
- 子 Agent 并发通过 `agent` 的 `slotManager` 管理（最大 8 并发 + 颜色分配）

### AgentMD 解析

`bot/agent/subagent/registry.go`：注册内置/插件 Agent，并解析 Claude Code 格式的 `agents/*.md`（YAML frontmatter）。

## 治理系统

### Ledger（工具执行账本）

`bot/policy/ledger/ledger.go` 是账本入口，追踪所有工具执行事件，记录：
- `readFiles`：已读取文件集合
- `modifiedFiles`：已修改文件集合
- `blockedTools`：被阻止的工具调用
- `toolErrors`：工具执行错误
- `verifications`：验证记录

### ResponseGate（响应门控）

`bot/agent/internal/kernel/gate.go`：防止治理内部信号泄漏到模型可见输出。默认最多 2 次重试。

### 工具语义分类

`bot/policy/semantics/semantics.go`：定义工具语义标签：
- `SourceProducing`：产生源码信息（read/grep/glob/list）
- `Mutating`：修改文件（write/edit/bash）
- `Verifying`：验证操作
- `VerificationTrusted`：直接验证命令（如 `go test`、`pytest`）
- `VerificationProjectRule`：项目脚本/规则验证（如 `npm run test`、`make test`）

## TUI 组件树

```
Model
├── Header         — provider/model · tokens
├── Splash         — 启动页
├── Messages       — 消息列表 + Scrollbar
├── Suggestions    — 命令补全
├── Input          — 消息输入框（3 行固定高度，SetPromptFunc 控制换行）
├── ConfirmBar     — 确认栏（工具 + 插件安装）
├── QuestionBar    — 多选/文本问题栏
└── runtimeEvents  — runtime 事件订阅
```

## 模块职责

| 模块 | 位置 | 职责 |
|------|------|------|
| Runtime 控制层 | `runtime/` | 交互层统一入口：Manager + 核心 run 协议 + 可选能力 + HTTP API |
| 中立交互契约 | `protocol/` | Bot 与 runtime 适配器共享的步骤、待办、确认、提问和指标类型 |
| Runtime DTO | `runtime/protocol.go` | runtime 对交互层公开的配置、上下文、会话和扩展数据 |
| 展示格式 | `interaction/interaction.go` | TUI、Connector 等交互端共享的工具摘要 |
| Bot 底座 | `bot/` | Agent 能力装配、领域操作与生命周期，不感知 UI/runtime |
| 标准适配 | `runtime/standard/` | Bot 领域协议 → runtime 能力与 UI DTO |
| Agent 循环 | `bot/agent/` | Reason→Execute→Feedback，中断，重试 |
| 治理系统 | `bot/policy/` | Policy：hook + ledger + exploration + quota |
| 工具账本 | `bot/policy/ledger/` | 工具执行追踪（读/写/阻止/错误/验证） |
| 推理格式 | `bot/agent/model.go` | LLM 响应分类 + GarbledToolCall 检测 |
| 工具策略 | `bot/policy/` | 配额、工具前后决策、事件收集、hook 注入 |
| 子 Agent | `bot/agent/subagent/` | 独立循环，3 种内置类型 + 插件扩展 |
| 子槽位 | `bot/agent/slots.go` | 并发控制（8 槽位 + 颜色） |
| 探索分数 | `bot/policy/exploration/` | 工具事件驱动的分数衰减与恢复 |
| 工具配额 | `bot/policy/policy.go` | Policy 私有的单轮读取配额 |
| LLM 网关 | `bot/provider/` | OpenAI/Anthropic 双协议，统一接口 |
| 工具系统 | `bot/tools/` | Registry + builtin 实现 + runtime 执行编排 |
| 工具注册 | `bot/tools/builtin/catalog/` | Toolbox 内置工具组装与关闭 |
| 工具执行 | `bot/tools/runtime/runner/` | 执行引擎（单工具/批量/预览/权限） |
| 文件系统工具 | `bot/tools/builtin/filesystem/` | read/write/edit/list/tree/glob/grep |
| Shell/Process 工具 | `bot/tools/builtin/shell/` | Shell 执行、托管进程与风险分级 |
| Web 工具 | `bot/tools/builtin/web/` | web_search/web_fetch/html2md |
| 媒体工具 | `bot/tools/builtin/media/` | image_gen（即梦文生图） |
| 任务工具 | `bot/tools/builtin/task/`, `bot/tools/builtin/todo/` | sub-agent task 与 todo_write |
| 代码索引工具 | `bot/tools/builtin/index/` | 代码索引（条件注册） |
| 上下文管理 | `bot/contextmgr/` | Build 管线 + 五级压缩 + token 估算 |
| Project Memory | `bot/contextmgr/memory/` | 项目 Memory 文件加载与 prompt 注入 |
| Plugin 系统 | `bot/extension/plugin/` | manager 编排 + manifest/registry |
| MCP 客户端 | `bot/extension/mcp/` | JSON-RPC 2.0 |
| Skill 系统 | `bot/extension/skill/` | manager + 管理快照 + YAML 技能加载 |
| Hook 系统 | `bot/policy/` | 事件驱动（6 种触发点）+ 声明式（plugin/） |
| 内置 Hook | `bot/policy/builtin/` | 9 个内置 Hook 实现 |
| 声明式 Hook | `bot/policy/plugin/` | JSON 配置驱动 Hook |
| 命令系统 | `bot/command/` | 斜杠命令解析 |
| 诊断日志 | `logger/` | 项目文件日志（时间戳 + subagent 标签） |
| 工具语义 | `bot/policy/semantics/` | Semantics 分类（SourceProducing/Mutating/Verifying） |
| Session 持久化 | `bot/session/` | Manager、Snapshot 与 JSON 存取 |
| TUI | `interaction/tui/` | Bubble Tea v2 组件化 |
| Connector | `interaction/connect/` | Telegram 等外部 IM 接入 |
