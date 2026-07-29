# NekoCode 架构文档

> **本文档职责**: 描述项目架构——目录结构、包依赖、模块实现、代码层面的机制。不包含 UI 设计、交互设计、设计原则等属于 DESIGN.md 的内容。

## 项目概述

NekoCode 是一个基于 Go 的终端 AI 助手，使用 Bubble Tea v2 构建 TUI，支持多 LLM provider（OpenAI 兼容 / Anthropic 兼容协议），具备 Agent 循环、Native Function Calling、工具执行、权限确认、Plan Mode、Plugin 系统、事件驱动 Hooks、MCP 客户端、子 Agent、上下文管理、Session Memory、AI 文生图等机制。

## Bot 层目标结构

Bot 层按子系统组织，目录结构和依赖方向都应接近树状：上层负责装配，下层负责能力实现；子系统之间通过窄接口交互，避免横向穿透和反向依赖。

目标分层：

```
bot/
├── app/                 # 对外 Bot API + 生命周期装配，保持薄层
├── agent/runtime/       # Agent 主循环：turn、LLM 调用、工具反馈、停止条件
├── agent/subagent/      # 子 Agent 执行引擎
├── contextmgr/          # 上下文、压缩、memory、token 统计
├── provider/            # LLM 协议、客户端工厂、stream/http 类型
├── tools/               # 工具定义、注册、执行（builtin/ 实现 + runtime/ 执行编排）
├── policy/              # 策略系统：Policy、Hook 引擎（Registry/builtin/plugin）、ledger、budget、tool semantics
├── extension/           # plugin、skill、mcp 扩展实现与管理
├── command/             # slash command 注册和生命周期命令
├── session/             # 会话持久化
├── prompt/              # system/plan prompt 构建
├── view/                # common/view 类型 re-export + bot 内部类型 → view DTO 转换
├── todo/                # Todo 列表管理
└── config/
```

依赖规则：

- `app` 是装配层，可以依赖各子系统；其他子系统不能依赖 `app`。
- `agent/runtime` 只依赖 LLM、context、tools、policy 等运行时接口，不关心 plugin/skill/mcp 的安装和发现。
- `tools` 只定义和执行工具，不反向依赖 agent 主循环或 `agent/subagent`；需要委托子 Agent 时通过 `TaskRunner` 接口接线，具体适配器放在 `app`。
- `agent/subagent` 不能依赖 `agent/runtime`；经 `RunConfig.Policy` 注入主 agent 的 Policy，共享治理账本与探索预算。
- `extension.Manager` 是扩展系统唯一的高层入口；`app` 不直接持有 plugin/skill/mcp 子 Manager。
- `plugin` 只管理插件清单、安装和启停状态；扩展激活由 `extension.Manager` 统一编排。
- `mcp.Manager` 同时拥有 server 和对应工具的生命周期，外层不手工同步工具切片。
- `extension.Manager` 直接满足 `command.SkillProvider`，`app` 不增加 Skill provider 包装层。
- 无引用的 alias 包应删除；内部新代码应直接依赖真实实现包。

## 目录结构

```
nekocode/
├── main.go                         # Wails GUI 入口 + interaction/gui/web/dist embed
├── wails.json                      # Wails 构建配置
├── cmd/
│   ├── tui/
│   │   └── main.go                 # TUI 程序入口
│   ├── tui_snapshot/
│   │   └── main.go                 # TUI 快照调试入口
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
├── common/
│   ├── view/                       # 跨层共享的 view DTO（bot / runtime / interaction 统一使用）
│   └── debug/                      # 全局调试日志
├── util/                           # 通用工具包（duration / fs / http / registry / sse / text / url / yaml）
├── runtime/                        # 交互控制层（TUI/GUI/HTTP/connector 的统一入口）
│   ├── runtime.go                  #   入口：Manager + New(Backend)
│   ├── backend.go                  #   单一 Bot Backend 契约 + 可选管理能力
│   ├── backend_run.go              #   Bot 回调注入与运行事件转换
│   ├── control.go                  #   submit/steer/abort/approve/answer
│   ├── events.go                   #   事件订阅、历史与录制
│   ├── query.go                    #   run/artifact/stats/session 查询
│   ├── management.go               #   管理操作（session/config/skill 等）
│   ├── connectors.go               #   connector 注册、连接与状态
│   ├── protocol.go                 #   稳定交互协议的对外 re-export
│   ├── redaction.go                #   输入脱敏入口
│   ├── standard/                   #   标准应用 runtime 组装
│   │   └── standard.go             #     New(): Bot + 事件录制 + connector
│   ├── httpapi/                    #   HTTP/SSE API server
│   └── internal/                   #   内部实现
│       ├── core/                   #     protocol/interaction/views 契约源
│       ├── broker/                 #     approval/question 同步交互
│       ├── eventbus/               #     事件总线
│       ├── runstore/               #     run 状态存储
│       ├── recording/              #     事件录制
│       ├── connectors/             #     connector 注册表
│       └── artifact/               #     产物管理
├── bot/                            # 核心逻辑
│   ├── bot.go                      #   package bot 入口（类型别名导出）
│   ├── app/                        #   Bot 应用层（依赖注入 + 生命周期编排）
│   │   ├── bot.go                  #     Bot 结构体 + New() 初始化编排
│   │   ├── api.go                  #     Bot 对外方法：run/query/config/skill/session/runtime views
│   │   ├── callbacks.go            #     前端回调状态 + 插件安装确认适配
│   │   ├── session.go              #     session.Manager 构造、命令与 Bot API
│   │   ├── subagent.go             #     task tool 到 subagent engine 接线
│   │   ├── extension.go            #     extension.Manager 构造 + config MCP + 管理快照
│   │   ├── extension_view.go       #     extension 领域快照 → view DTO
│   │   └── extension_view_test.go
│   ├── agent/                      #   Agent 循环
│   │   ├── runtime/                #     Agent 运行时核心
│   │   │   ├── agent.go            #       Agent 结构体 + New()
│   │   │   ├── loop.go             #       Run() 主循环
│   │   │   ├── turn.go             #       单轮处理：PreTurn → 推理 → 工具执行 → PostTurn
│   │   │   ├── state.go            #       运行状态（runState：步数/停止原因/计数器）
│   │   │   ├── model_runner.go     #       LLM 调用封装：调用 + 重试 + 流回调 + 最终答案合成
│   │   │   ├── reasoning.go        #       响应分类（chat/tool_call/garbled/error）
│   │   │   ├── tool_runner.go      #       工具执行编排：执行 + 结果收集 + PostTool hooks
│   │   │   ├── tool_filter.go      #       工具调用过滤（配额 + PreToolUse hooks）
│   │   │   ├── tool_results.go     #       工具结果处理
│   │   │   ├── tool_subagents.go   #       子代理回调路由
│   │   │   ├── slots.go            #       子代理槽位管理（最多 8 个）
│   │   ├── kernel/                 #     Agent 循环核心控制流（零业务依赖）
│   │   │   ├── loop.go             #       Loop/RunLoop：通用步进循环骨架
│   │   │   ├── lifecycle.go        #       Lifecycle：可打断可复活的运行上下文 + steering 信箱
│   │   │   └── gate.go             #       Gate：通用重试预算
│   │   ├── llmstream/              #     LLM 流式调用 + 工具调用解析 + 重试
│   │   ├── subagent/               #     子 Agent 系统
│   │   │   ├── agents.go           #       内置 agent 类型定义（3 种：executor/verify/researcher）
│   │   │   ├── agent_md.go         #       AgentMD 解析（Claude Code 格式）
│   │   │   ├── engine.go           #       子 Agent 执行引擎入口
│   │   │   ├── engine_context.go   #       上下文构建
│   │   │   ├── engine_executor.go  #       工具执行
│   │   │   ├── engine_reason.go    #       推理循环
│   │   │   ├── engine_state.go     #       状态管理
│   │   │   ├── registry.go         #       注册表（builtins + plugins）
│   │   │   ├── result.go           #       结果类型
│   │   │   ├── result_builders.go  #       结果构建器
│   │   │   ├── safety.go           #       安全审核
│   │   │   └── prompts/            #       子 Agent prompt 模板
│   │   │       ├── executor.md     #         executor prompt
│   │   │       ├── researcher.md   #         researcher prompt
│   │   │       └── verify.md       #         verify prompt
│   ├── config/                     #   配置管理
│   │   ├── config.go               #     Config + Load()
│   │   └── view.go                 #     GUI/API 配置视图 DTO
│   ├── command/                    #   斜杠命令系统
│   │   ├── parser.go               #     Parser + Callbacks
│   │   └── lifecycle.go            #     SummarizeIfNeeded / ForceFreshStart / ContextStats
│   ├── contextmgr/                 #   上下文管理
│   │   ├── manager.go              #     Manager 组装（8 个 internal 子组件）+ New/NewSub + MakeSummarizer
│   │   ├── api.go                  #     Manager 对外方法 + 公开 DTO（ManagerSnapshot/ContextReport）
│   │   ├── compression/            #     压缩子系统
│   │   │   ├── levels.go           #       五级预警阈值
│   │   │   ├── strategy.go         #       Strategy 接口
│   │   │   ├── budget.go           #       工具结果预算截断
│   │   │   ├── prompt.go           #       压缩 prompt 模板
│   │   │   ├── merge.go            #       Archive 摘要合并
│   │   │   ├── replacement/        #       现行压缩策略（摘要替换式）
│   │   │   └── legacy/             #       旧 5 层管线（保留未用）
│   │   ├── context/                #     上下文内容定义（package content）
│   │   │   └── content.go          #       Content 结构体 + BuildLayer*（含 Memory 字段）
│   │   ├── internal/               #     内部实现
│   │   │   ├── builder/            #       Build 管线（孤儿过滤）
│   │   │   ├── compaction/         #       压缩控制器
│   │   │   ├── history/            #       消息存取与截断
│   │   │   ├── report/             #       上下文诊断报告 + 彩色 bar
│   │   │   ├── settings/           #       系统提示/技能/Hints/Todo setter
│   │   │   ├── snapshot/           #       Snapshot / Restore（会话持久化）
│   │   │   ├── state/              #       共享状态结构
│   │   │   └── usage/              #       token 用量统计
│   │   ├── memory/                 #     Session Memory（五段式项目记忆文件）
│   │   └── token/                  #     Token 估算
│   │       ├── estimate.go         #       启发式估算
│   │       └── tracker.go          #       API 校准追踪
│   ├── provider/                   #   LLM 抽象层
│   │   ├── types/                  #     核心类型定义（Message/Response/ToolDef + HTTP 客户端）
│   │   ├── anthropic/              #     Anthropic Messages API 兼容实现
│   │   ├── openai/                 #     OpenAI Chat Completions 兼容实现（DeepSeek / MiniMax 等）
│   │   ├── llm.go                  #     LLM 接口定义
│   │   ├── factory.go              #     NewClientWithProtocol 工厂
│   │   └── retry.go                #     指数退避重试
│   ├── policy/                    #   治理引擎 + 治理语义
│   │   ├── policy.go              #     唯一入口：Policy 构造、生命周期、快照
│   │   ├── tools.go               #     BeforeTool / RecordTool(s)
│   │   ├── events.go              #     Turn / ToolRequest / ToolResult / Facts
│   │   ├── hook.go                #     Hook / State / Result 对外类型
│   │   ├── hooks.go               #     Hook 注册、私有状态、评估
│   │   ├── audit.go               #     Hook 审计事件与输出格式化
│   │   ├── plugin.go              #     LoadPluginHooks（声明式 hooks）
│   │   ├── builtin/               #     内置 Hook 实现
│   │   │   ├── all.go             #       All() 列表
│   │   │   ├── register.go        #       Register（9 个内置 Hook）
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
│   │   ├── budget/                #     单轮工具配额
│   │   └── semantics/             #     工具语义分类
│   ├── extension/                  #   扩展系统
│   │   ├── extension.go            #     入口：Manager（插件/Skill/Hook/Agent/MCP 生命周期）
│   │   ├── commands.go             #     /plugin 命令与安装交互
│   │   ├── mcp/                    #     MCP 客户端
│   │   │   ├── mcp.go              #       入口：Manager（server + tool 生命周期 + 健康状态）
│   │   │   ├── client.go           #       client：进程连接句柄（Start/Close/ListTools/CallTool）
│   │   │   ├── protocol.go         #       JSON-RPC 2.0 协议
│   │   │   ├── types.go            #       ServerConfig 等类型定义
│   │   │   └── tool.go             #       mcpTool 适配器
│   │   ├── plugin/                 #     Plugin 系统
│   │   │   ├── plugin.go           #       入口：Plugin 核心类型 + Manager（清单与状态）
│   │   │   ├── commands.go         #       插件命令展示与安装预览
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
│   │   ├── system/                 #     System Prompt 子模块
│   │   │   ├── builder.go          #       构建器
│   │   │   ├── system_zh.md        #       中文 System Prompt 模板
│   │   │   └── os_release.go       #       OS 检测
│   │   └── planmode/               #     Plan Mode
│   │       └── prompt.go           #       Plan Mode prompt
│   ├── session/                    #   Session 管理
│   │   ├── session.go              #     Snapshot / Meta 持久化
│   │   ├── manager.go              #     Session Manager / 导出 / 恢复
│   │   └── view_messages.go        #     DisplayMessages 转换
│   ├── view/                       #   View DTO 转换层
│   │   ├── view.go                 #     common/view 类型 re-export + 通用转换
│   │   ├── config.go               #     配置视图转换
│   │   ├── messages.go             #     消息显示转换
│   │   └── image.go                #     图片引用转换
│   ├── todo/                       #   Todo 列表管理
│   └── tools/                      #   工具系统
│       ├── registry.go             #     工具注册表
│       ├── builtin/                #     内置工具实现
│       │   ├── catalog/            #       RegisterAll() 注册清单
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
│           ├── execution/          #       执行状态 + 缓存
│           ├── permission/         #       权限确认
│           ├── sandbox/            #       沙箱隔离
│           ├── workspace/          #       工作区访问控制
│           ├── snapshot/           #       文件快照
│           ├── taskbridge/         #       task 工具到 subagent 的桥接
│           ├── toolhelpers/        #       辅助函数
│           └── toolutil/           #       路径/文本等通用工具
├── interaction/connect/            # 外部 IM connector
│   ├── core/                       #   共享构件：connect.json 分段配置层 / 配对状态机 / connector 基座 / 共享指令 / 流式节流 / 事件分发骨架
│   ├── telegram/                   #   Telegram connector（polling + pairing + taskview 渲染）
│   ├── feishu/                     #   飞书 connector（Lark SDK WS 长连接 + DM 配对，MVP）
│   └── qqbot/                      #   QQ connector（腾讯官方机器人平台：Gateway WS + v2 消息 API，纯文本）
├── interaction/tui/                # TUI 交互界面
│   ├── tui.go                      #   package tui 入口（Run 函数）
│   ├── agent.go                    #   Agent 桥接 + startChat
│   ├── model.go                    #   Model 结构体
│   ├── update.go                   #   Update() 消息分发
│   ├── view.go                     #   View() 视图布局组装
│   ├── handlers.go                 #   按键处理
│   ├── helpers.go                  #   token 统计文案（工具简报已统一到 common/view/toolbrief.go）
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

```go
type Control interface {
    Submit(ctx context.Context, input Input) (RunID, error)
    Steer(ctx context.Context, runID RunID, input Input) error
    Abort(ctx context.Context, runID RunID) error
    Approve(ctx context.Context, approvalID string, decision ApprovalDecision) error
    Answer(ctx context.Context, questionID string, reply view.QuestionReply) error
    Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, error)
    Connect(ctx context.Context, name string, args []string) (string, error)
    Disconnect(name string) (string, error)
}
```

`runtime.Manager` 是唯一生产实例，使用 `runtime.New(backend)` 构造；标准应用通过 `runtime/standard.New()` 一次性完成 Bot、默认事件录制和 connector 装配，并显式处理初始化错误。TUI、GUI、HTTP 只使用 `Control`/`Query`/`Management` 能力面，connector 只获得更窄的 `ConnectorRuntime`，但这些能力面都指向同一个 Manager，不创建额外 facade 实例。

协议源定义在 `runtime/internal/core`，按 `protocol.go`、`interaction.go`、`views.go`、`backend.go` 拆分。view DTO（ConfirmRequest / QuestionRequest / DisplayMessage / BotStats 等）alias 到 `common/view`，bot、runtime、interaction 三层共享同一份类型定义，跨层传递无需转换。`Input`、`Event`、`RunView` 的 JSON 契约由根 `runtime/protocol.go` 稳定导出。

## Bot 应用层（bot/app/）

`bot/app/` 是核心依赖注入和生命周期编排层。`Bot` 结构体持有所有子系统引用，通过 `New()` 按顺序初始化：

```
New()
  ├── initConfig()        → config.Load() + prompt/system.NewBuilder()
  ├── initCtxMgr()        → contextmgr.New() + index.Apply()
  ├── initSession()       → session.NewManager()
  └── rebuildRuntime()
      ├── initToolRegistry()     → catalog.RegisterAll() + index tool（条件注册）
      ├── initPolicy()           → policy.New() + builtin.Register()
      ├── initExtensions()       → extension.New().Load() + config MCP
      ├── initAgent()            → provider.NewClientWithProtocol() + runtime.New(AgentConfig)
      ├── ctxMgr.SetSummarizer() → 使用 initAgent 设置的 merge client
      └── initCommands()         → command.RegisterAll() + session/export/plugin 命令
```

## 核心架构：Agent 循环

`bot/agent/runtime` 是 L2 agent 核心，其公开契约（构造/控制/内省/流式接线/事件）冻结在包注释中，见 `bot/agent/runtime/doc.go`。

```
用户输入
  │
  ▼
Run() 主循环 → runTurn(state)
  │
  ├─ AutoCompactIfNeeded() 看门狗
  ├─ Policy.BeginTurn()：重置 turn 事件、计算配额、评估 PreTurn
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
  │   └─ Policy.RecordTools()：账本 + 探索分数 + PostToolUse/PostTool
  │
  ├─ [文本响应] handleText(reasoning, state)
  │   ├─ Emit garbled/chat Turn
  │   └─ Policy.AfterTurn() → Stop/Hint
  │
  └─ synthesizeAndReturn() 兜底总结
```

Agent 循环硬限制：
- `maxAgentSteps = 150`：最大迭代步数
- `maxConsecutiveHints = 3`：连续纯文本提示上限
- `maxConsecutiveFailures = 5`：连续 LLM 调用失败上限
- `maxFinalCheckHints = 2`：最终检查重试上限

### Agent 子包结构

`bot/agent/` 的实现位于子包：

| 子包 | 职责 |
|------|------|
| `runtime/` | Agent 结构体 + `Run()` 主循环 + 单轮处理 + LLM 调用封装 + 工具执行编排 + 状态管理 |
| `kernel/` | Agent 循环核心控制流（Loop/RunLoop、Lifecycle、Gate），零业务依赖 |
| `llmstream/` | LLM 流式调用 + 工具调用解析 + 重试 |
| `subagent/` | 子 Agent 引擎 + 注册表 + 安全审核 |
| `policy/` | Policy：组合 hook、ledger、exploration 与 quota 的唯一入口 |
| `policy/ledger/` | 工具执行账本：readFiles / modifiedFiles / blockedTools / verifications |
| `policy/exploration/` | 并发安全的探索分数 |
| `policy/budget/` | 单轮 ToolQuota |

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
2. Layer 0: Memory（项目记忆，内容通过 context.Content.Memory 字段承载）
3. Layer 0.5: Archive（压缩摘要）
4. Layer 1: Messages（全部保留，不再截断；Compactor 负责压缩）
5. Layer 2: Todo + Hints（动态层）

### Manager 关键方法

| 方法 | 说明 |
|------|------|
| `Build(withTools)` | 组装完整消息列表（含孤儿过滤） |
| `NewSub(prompt, window, mergeClient)` | 创建子 Agent 轻量 Manager |
| `AutoCompactIfNeeded()` | 自动压缩看门狗 |
| `Summarize()` | 手动触发完整压缩 + Archive 合并 |
| `Snapshot() / Restore()` | 会话持久化 |
| `FreshStart()` | 清空所有消息 |

### Session 持久化实现

保存链路（`bot/app/api.go` `RunAgent`）：

```
ag.Run()  →  ctxMgr.SetSystemPrompt()  →  SummarizeIfNeeded()  →  sess.Save()
```

- `sess.Save()`（`bot/session/manager.go`）调 `ctxMgr.Snapshot()` 捕获当前状态，再 `sess.Save()` 落盘到 `~/.nekocode/sessions/<id>/session.json`
- `Snapshot()`（`bot/contextmgr/snapshot.go`）**深拷贝** `Messages` 后再返回，避免后续 `append` 共享 backing array 导致已捕获内容被覆盖
- 触发时机：每次 `RunAgent` 结束自动保存；`/sessions`、`/export`、`/new` 等命令也会触发

Agent 结束时的文本处理（`bot/agent/runtime/loop.go` `finishRun`）：

- `finalText` 优先，为空时回退到 `lastText`
- 仅在真正无文本产出时才调 `Synthesize()`（追加的总结消息是最后手段）
- `applyFinalPolicyBlock`（`bot/agent/runtime/turn.go`）会同步设 `finalText = lastText`，避免无谓的 `Synthesize` 追加

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

`bot/tools/builtin/catalog/register.go` 中的 `RegisterAll()` 注册内置工具（bash/read/write/list/tree/glob/edit/grep/web_search/web_fetch/todo_write/task）。`image_gen` 在 `RegisterAll` 中条件注册（需要 imageGenModels 非空）；`bot/app/bot.go` 负责把工具注册表、skill manager 和 command parser 接线，`skill` 作为 slash command 的上下文注入入口，不是独立工具。

### 内置工具

| 工具 | 模式 | 危险等级 | 位置 |
|------|------|----------|------|
| bash | Sequential | 智能分级（Safe～Forbidden），多层沙箱隔离 | `bot/tools/builtin/shell/` |
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
| `builtin/catalog/` | RegisterAll() 注册清单 |
| `builtin/filesystem/{read,write,edit,list,tree,search}/` | 文件系统工具 |
| `builtin/shell/` | Bash 执行 + 危险分级 |
| `builtin/web/` | Web 搜索/抓取/HTML2MD |
| `builtin/media/` | 图片生成（即梦文生图） |
| `builtin/task/` | 子 Agent 任务工具 |
| `builtin/todo/` | Todo 管理工具 |
| `builtin/question/` | 用户提问工具 |
| `builtin/diff/` | Diff 工具 |
| `builtin/index/` | 代码索引工具（条件注册） |
| `runtime/core/` | Tool 接口 + ToolCallItem/Result + Descriptor |
| `runtime/runner/` | 工具执行引擎（单工具/批量/预览/权限） |
| `runtime/execution/` | 执行状态 + 缓存 |
| `runtime/permission/` | 权限确认 |
| `runtime/sandbox/` | 沙箱隔离 |
| `runtime/workspace/` | 工作区访问控制 |
| `runtime/snapshot/` | 文件快照 |
| `agent/llmstream/` | LLM 流式调用 + 工具调用解析 |
| `runtime/taskbridge/` | task 工具到 subagent 的桥接 |
| `runtime/toolhelpers/` | 辅助函数 |
| `runtime/toolutil/` | 路径/文本等通用工具 |

## Hook 系统（事件驱动）

### 五种触发点

| Point | 时机 | 注入方式 |
|-------|------|---------|
| PreTurn | LLM 推理前 | Layer2 hints |
| PreModelRequest | 模型请求前（request-scoped） | `[System]` 消息 |
| PreToolUse | 单个工具执行前（per-tool） | `[System]` 消息 |
| PostTool | 全部工具执行后（batch） | `[System]` + Stop |
| PostTurn | LLM 纯文本返回后 | `[System]` + Stop |

### 事件与状态

Runtime 只向 `Policy` 提交 `Turn`、`ToolRequest`、`ToolResult` 和
`TurnResult`。Policy 组合 ledger 与 exploration，生成只读 `Facts` 供
hook 判断。Hook 的计数器和去重标记按 hook 名隔离，不能读写其他规则或
runtime 的内部字段。

### 内置 Hook（9 个）

| Hook | Point | 功能 |
|------|-------|------|
| quota | PreTurn | 读取配额不足时告警，引导优先实质性修改 |
| tool_result_guardrail | PreModelRequest | 工具结果过多时注入 request-scoped 提醒 |
| read_before_write | PreToolUse | edit/write 前检查目标文件读取记录 |
| read_only_spiral | PostTool | 连续只读探索后提醒综合发现并停止继续读 |
| verification | PostTurn | 有未完成任务但本轮无工具调用时提醒继续 |
| exploration_exhausted | PreTurn | 探索调用 ≥10 且分数耗尽时提醒收敛 |
| explore_cascade | PostTool | 本轮启动 ≥4 个 researcher 时提醒综合信息 |
| progress_stall | PostTool | 连续多轮无进展后提醒推进或报告阻塞 |
| garbled_circuit_breaker | PostTurn | 累计 5 次 garbled 工具调用则强制停止 |

## Plugin 系统

`bot/extension/`：
- 唯一高层入口是 `Manager`（`extension.go`）：`New(ctx, tools, policy, contextWindow)` 后调用 `Load()`
- 统一激活和停用插件提供的 Skills / Agents / Hooks / MCP Servers
- 插件安装、启停和 Reload 都经过同一条生命周期；Skill 每次只按当前启用插件集合重载一次
- 通过 `Snapshot()` 一次返回管理视图状态，并提供 Skill 查询、插件启停等精简子方法

`bot/extension/plugin/`：
- 入口是 `Manager`（`plugin.go`）：只拥有 registry、安装管线和启停状态
- 安装源：GitHub URL / user:repo / 本地路径
- 扩展点：Skills / Agents / Hooks / MCP Servers
- manifest / registry / 安装源模型
- 插件自身不注册 Agent、Hook、MCP 或 Skill，不持有宿主回调
- 不依赖 `common/view`：GUI/API 管理快照由 `bot/app` 装配层投影

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
- 唯一入口是 `Manager`（`skill.go`）：`New(ctx, tools, contextWindow)`；registry 与 tool 适配器均为包内实现
- YAML 格式技能定义
- 目录发现 + 加载
- 内置技能通过 `bundled/` go:embed
- `skill` 工具动态注册到 toolRegistry
- `Load(pluginDirs)` / `Reload(pluginDirs)` 显式接收插件 Skill 目录，不持有目录回调
- 不依赖 `common/view`：GUI/API 管理快照由 `bot/app` 装配层用 `Manager.List()/LoadedSet()` 公开快照投影而成

## 子 Agent 系统

### 内置类型（3 种）

| Agent | 用途 | 工具 | 特殊配置 |
|-------|------|------|---------|
| executor | 执行代码修改 | read/write/edit/bash/grep/glob/list/question | — |
| verify | 验证修改 | read/grep/glob/list/bash | — |
| researcher | 代码探索/调研 | read/grep/glob/list/web_search/web_fetch/question | OmitProjectContext: true |

### Engine 特性

- 独立 ctxmgr（NewSub），可选接入 Compactor
- FileCache 从主 Agent 种子预热（Seed/Merge）
- 上下文窗口、Thinking 开关等参数从主 Agent 配置继承
- 安全审核（关键词匹配 + 敏感路径检测）
- DisableThinking 默认关闭，researcher 支持 Thoroughness 深度控制
- Handoff 上下文注入（`<handoff>` 块追加到 system prompt）
- ConfirmFn 覆盖（edit 操作需用户确认）
- Partial result 恢复（中断/错误时返回部分结果）
- Metadata 追踪（totalTokens、toolUseCount、durationMs、cacheHitTokens、cacheMissTokens）
- Phase 回调（cfg.OnPhase 通知阶段变化）
- 子 Agent 并发通过 `agent/runtime` 的 `slotManager` 管理（最大 8 并发 + 颜色分配）

### AgentMD 解析

`bot/agent/subagent/agent_md.go`：解析 Claude Code 格式的 `agents/*.md`（YAML frontmatter）。

## 治理系统

### Ledger（工具执行账本）

`bot/policy/ledger/ledger.go` 是账本入口，追踪所有工具执行事件，记录：
- `readFiles`：已读取文件集合
- `modifiedFiles`：已修改文件集合
- `blockedTools`：被阻止的工具调用
- `toolErrors`：工具执行错误
- `verifications`：验证记录

### ResponseGate（响应门控）

`bot/agent/runtime/state.go`：防止治理内部信号泄漏到模型可见输出。默认最多 2 次重试。

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
| Runtime 控制层 | `runtime/` | 交互层统一入口：Manager + Control/Query/Management + HTTP API |
| 共享 DTO | `common/view/` | 跨层 view 类型定义（bot/runtime/interaction 共用） |
| Bot 应用层 | `bot/app/` | 依赖注入 + 生命周期编排 + Bot 公开兼容面实现 |
| View 转换 | `bot/view/` | bot 内部类型 → view DTO 转换 |
| Agent 循环 | `bot/agent/runtime/` | Reason→Execute→Feedback，中断，重试 |
| 治理系统 | `bot/policy/` | Policy：hook + ledger + exploration + quota |
| 工具账本 | `bot/policy/ledger/` | 工具执行追踪（读/写/阻止/错误/验证） |
| 推理格式 | `bot/agent/runtime/`（reasoning.go） | LLM 响应分类 + GarbledToolCall 检测 |
| 工具策略 | `bot/policy/` | 配额、工具前后决策、事件收集、hook 注入 |
| 子 Agent | `bot/agent/subagent/` | 独立循环，3 种内置类型 + 插件扩展 |
| 子槽位 | `bot/agent/runtime/slots.go` | 并发控制（8 槽位 + 颜色） |
| 探索分数 | `bot/policy/exploration/` | 工具事件驱动的分数衰减与恢复 |
| 工具配额 | `bot/policy/budget/` | 单轮读取配额 |
| LLM 网关 | `bot/provider/` | OpenAI/Anthropic 双协议，统一接口 |
| 工具系统 | `bot/tools/` | Registry + builtin 实现 + runtime 执行编排 |
| 工具注册 | `bot/tools/builtin/catalog/` | RegisterAll() 内置工具注册清单 |
| 工具执行 | `bot/tools/runtime/runner/` | 执行引擎（单工具/批量/预览/权限） |
| 文件系统工具 | `bot/tools/builtin/filesystem/` | read/write/edit/list/tree/glob/grep |
| Shell 工具 | `bot/tools/builtin/shell/` | bash 执行与风险分级 |
| Web 工具 | `bot/tools/builtin/web/` | web_search/web_fetch/html2md |
| 媒体工具 | `bot/tools/builtin/media/` | image_gen（即梦文生图） |
| 任务工具 | `bot/tools/builtin/task/`, `bot/tools/builtin/todo/` | sub-agent task 与 todo_write |
| 代码索引工具 | `bot/tools/builtin/index/` | 代码索引（条件注册） |
| 上下文管理 | `bot/contextmgr/` | Build 管线 + 五级压缩 + token 估算 |
| Session Memory | `bot/contextmgr/memory/` | Memory 文件持久化 |
| Plugin 系统 | `bot/extension/plugin/` | manager 编排 + manifest/registry |
| MCP 客户端 | `bot/extension/mcp/` | JSON-RPC 2.0 |
| Skill 系统 | `bot/extension/skill/` | manager + 管理快照 + YAML 技能加载 |
| Hook 系统 | `bot/policy/` | 事件驱动（5 种触发点）+ 声明式（plugin/） |
| 内置 Hook | `bot/policy/builtin/` | 9 个内置 Hook 实现 |
| 声明式 Hook | `bot/policy/plugin/` | JSON 配置驱动 Hook |
| 命令系统 | `bot/command/` | 斜杠命令解析 |
| 调试日志 | `common/debug/` | 全局 debug.Log（时间戳 + subagent 标签） |
| 工具语义 | `bot/policy/semantics/` | Semantics 分类（SourceProducing/Mutating/Verifying） |
| Session 视图 | `bot/session/` | DisplayMessages 转换 |
| TUI | `interaction/tui/` | Bubble Tea v2 组件化 |
| Connector | `interaction/connect/` | Telegram 等外部 IM 接入 |
