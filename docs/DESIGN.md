# NekoCode 设计文档

> **本文档职责**: 描述产品设计——UI 布局、交互模式、视觉主题、Agent 能力设计、上下文管理策略、防幻觉设计原则。不包含代码实现细节、文件路径、函数名等属于 ARCHITECTURE.md 的内容。更新时请保持此边界。

## 产品定位

NekoCode 是一个运行在终端中的 AI 助手。它能理解自然语言、执行本地操作（文件读写、Shell 命令、文件搜索），并在执行可能有影响的操作前征求用户确认。

**人设形象**：一只软萌的二次元黑猫娘——元气、治愈、无攻击性。信息密度高时省略语气词，保持精练。

核心体验：**像和一位终端里的伙伴聊天一样，自然地交代任务，它帮你完成。**

## 品牌与标识

### Logo 设计

![NekoCode Logo](images/logo.png)

- **主体**：圆润黑猫脸 — 对应"猫娘"人设
- **眼睛**：青绿荧光发光瞳 — 代表"AI 智能"
- **顶部符号**：`</>`（完整版）— 代表"编程助手"
- **底板**：暖紫→暖粉渐变、大圆角方形 — 软萌、现代
- **点缀**：腮红、W 形笑嘴、胡须 — 增加"猫感"与亲和力

### 品牌色

| 用途 | 色值 | 说明 |
|------|------|------|
| 主渐变起点 | `#7C5CFC` | 暖紫 |
| 主渐变终点 | `#F472B6` | 暖粉 |
| 猫脸主色 | `#241F3D` | 深空蓝紫 |
| 猫耳内侧 / 腮红 | `#F472B6` | 腮红粉 |
| AI 眼发光 | `#22D3EE` / `#7AF8D0` | 青绿荧光 |
| 鼻子 / 嘴 | `#FBCFE8` | 浅粉 |

### 资源文件

源文件位于 `interaction/gui/web/public/logo/`：

- `logo.svg` — 完整品牌版（含 `</>` 与 `NEKOCODE` 字）
- `logo-icon.svg` — 纯图标，App 图标源
- `logo-icon-app.svg` — GUI 浏览器标签 favicon
- `appicon.icns` / `appicon.ico` — macOS / Windows 应用图标
- `icon_16..1024.png` — 各尺寸 PNG 中间产物
- `build_icns.js` — 任意平台生成 ICNS 的 Node 脚本（无需 macOS `iconutil`）

GUI Logo 组件：`interaction/gui/web/src/components/LogoMark.tsx`（32×32 viewBox，适配 17–28px 全尺寸）

### 主题适配

Logo 同时适配亮/暗背景：
- 底板饱和渐变在亮/暗背景都有足够对比
- 猫脸深色调在浅色底板内稳定
- 眼睛发光在暗色背景更醒目，亮色背景也有高光托底

## 交互模式

用户说"你好"等纯对话时，助手自然语言回复，不触发工具；说"帮我看看 main.go"等操作请求时，自动选择合适工具执行。**用户无需手动切换模式**——助手内部自动判断该聊天还是该操作。

### 斜杠命令

以 `/` 开头的输入为系统命令：

| 命令 | 效果 |
|------|------|
| `/help` | 显示可用命令列表（自动生成，含动态 skill） |
| `/new` | 开始新对话（保留上一任务摘要） |
| `/clear` | 清空所有对话历史和摘要 |
| `/context` | 上下文窗口详细分解 + 彩色 bar + used/total |
| `/summarize` | 手动触发上下文压缩，返回压缩前后对比 |
| `/config` | 显示当前 provider 和 model |
| `/model [name]` | 列出或切换模型 |
| `/plan <任务>` | 进入只读探索模式，设计方案后审批执行 |
| `/plugin <操作>` | 插件安装/卸载/列表/详情 |
| `/sessions [id]` | 会话管理：列表、恢复存档 |
| `/export` | 导出对话上下文到 JSON 文件 |
| `/<skill>` | 加载指定技能的工作流 |

输入 `/` 弹出补全，Tab/Shift+Tab 选择，Enter 填入。

### Plan Mode

`/plan <任务>` 进入只读探索模式，Agent 仅允许使用只读工具（read/grep/glob/list/web_search/web_fetch），禁止 write/edit/bash/task。

**工作流**：探索代码库 → 设计方案 → 呈现计划（变更摘要/文件列表/步骤/验证/风险/假设）→ 用户审批 → 退出 plan mode 开始执行。

## TUI 界面设计

### 视觉主题：深夜书房 × 猫娘

黑猫蜷在屏幕旁的意象——teal 色偶尔闪现，像暗处的猫眼。品牌主色（暖紫 `#7C5CFC` → 暖粉 `#F472B6`）用于 GUI；TUI 沿用 teal 主色 `#4ec9b0"。

**色彩体系**（`interaction/tui/styles/colors.go` 统一定义）：
- 主文字：`#a0a0a0`
- Teal 主色：`#4ec9b0`（styles.Primary），用于 Assistant 色条、spinner
- User 金：`#c9a96e`（styles.Yellow）
- 蓝：`#7a8ba0`（styles.Blue）
- 红：`#e06c75`（styles.Red）
- Diff 绿：`#98c379`（styles.DiffGreen）
- 弱化文字：`#666666`，中间：`#808080`
- 边框线：`#333333`

### 启动页

```
          /\___/\
         ( ◉   ◉ )
          =  ▾  =
         /|     |\
        (_|     |_)
           || ||

         NEKOCODE
          v0.3.2

      ──── ◆ ────

         Press Enter
```

猫眼 `◉` 闪烁 teal 光。用户按下 Enter 进入聊天界面。

### 聊天界面布局（厚左色条）

```
(=^.^=) NEKOCODE v0.3.2

▐ You                                                        ┃
▐ 帮我分析下项目架构                                           ┃

▐ Assistant                                                  ┃
▐                                                            ┃
  ┌ ◆ read ×5 [+] 展开 ─────────────────────────────────┐     ┃
  │ ◆ grep "func" .  [+]                                │     ┃
  └──────────────────────────────────────────────────────┘     ┃
▐                                                            ┃
▐ ## 项目架构                                                 ┃
▐ ...                                                        ┃
▐ Duration: 12.3s  ↑670 ↓128                                 ┃
```

- **左侧**：`▐`（U+2590）厚色条 + `PaddingLeft(1)` 统一缩进
- **右侧**：独立 Scrollbar 组件，`┃` thumb + `│` track
- **工具卡片**：暖金色 `NormalBorder`，单次 edit 块显示 `[+]`/`[-]` 折叠展开 diff
- **edit 工具组**：`◆ edit ×3 [-] 收起` 展开后直接内联每个文件的 diff，`▍ path` 标注文件，一次展开全部可见
- **其他工具组**：同名单行工具折叠为 `◆ read ×5 [+]`，展开后逐条显示
- **处理卡片**：teal 边框，分隔线横跨全宽区分 output/reasoning 区块

### 处理阶段

```
▐ ◉ Thinking (3.2s) ↑670 ↓56 🧹3    ← 当前阶段 + 耗时 + token + 微压缩计数

▐   ▍ output ──────────────────────   ← 分隔线（teal）
▐   正在分析项目结构...                ← 模型流式输出（动态 2-6 行）

▐   ▍ reasoning ───────────────────   ← 分隔线（蓝色）
▐   让我读取所有源文件来分析...        ← 推理过程（动态 2-6 行）

▐   ◆ glob ×2 [-] 收起                ← 收折工具组
▐     ◆ glob *.go                     ← 展开：逐条显示
▐     ◆ glob *.md
```

阶段流转：Waiting → Thinking → Reasoning → Running → Thinking → ... → Ready

- **Waiting**: LLM 调用已发出，等待首 token
- **Thinking**: ReasoningContent 到达（模型 CoT 推理）
- **Reasoning**: Content token 到达，模型生成文本中
- **Running**: 工具执行中
- **🧹N**: 累计微压缩清除的工具结果数

### 工具组折叠

```
◆ read ×15 [+] 展开    ← 收起（单行）
◆ read ×15 [-] 收起    ← 展开逐条：
  ◆ read (1/15) /path/to/file1.go
  ◆ read (2/15) /path/to/file2.go

◆ edit ×3 [-] 收起     ← edit 组展开，diff 内联
  ▍ server/main.go
    ── diff ──
    - old code
    + new code
  ▍ server/game.go
    ── diff ──
    - old line
    + new line
```

### 工具确认栏

```
Confirm
  bash go test ./...  [safe]
  Proceed?  [enter] yes  [esc] no
```

- 展示具体命令/路径而非仅工具名（如 `bash go build`、`write server/main.go`）
- 等级标签：`[safe]`/`[modify]`/`[danger]`/`[blocked]`
- `[modify]`/`[danger]` 黄色，`[blocked]` 红色（直接拒绝不弹框）
- `[safe]` 命令自动放行，不弹确认框

### 输入交互

- **发送**：Enter 提交，消息即时显示
- **处理中输入（BTW）**：Enter 注入新消息打断当前 LLM 调用
- **历史翻阅**：↑/↓ 翻阅历史
- **命令提示**：输入 `/` 弹出命令列表，Tab/Shift+Tab 选择
- **块切换**：Ctrl+E 展开/收起工具组和 edit diff

## 上下文管理

### 五级策略

| 层 | 触发条件（buffer 剩余） | 动作 |
|----|---------|------|
| **Normal** | > 44,800 | 无操作 |
| **Warning** | ≤ 44,800 | 仅告警，不操作 |
| **MicroCompact** | ≤ 35,200 | 清除旧 compactable 工具结果（read、grep、glob 等），保留最近 5 个 |
| **Compact** | ≤ 25,600 | LLM 生成结构化摘要压缩最旧消息 |
| **Blocking** | ≤ 6,400 | 拒绝新输入，强制压缩后继续 |

> 阈值针对 128K 上下文窗口自动缩放（DefaultConfig 基准为 64K）。


### 上下文锚点

压缩前自动标记应保留的关键消息——用户核心指令、系统约束、API 版本要求等。压缩过程中这些消息优先保留，防止关键上下文被误清除。

### 摘要验证

LLM 生成的摘要需要经过二次校验：检查是否保留了代码片段原文、错误信息原文、文件路径和行号等关键内容。验证失败则重新生成摘要，确保压缩保真度。

### Session Memory

上下文超过 10k token 后开始异步提取，每 +5k token + 3 个 tool call 再次触发。提取内容写入 `~/.nekocode/sessions/<id>/memory.md`（10 section Markdown 文件）。`/new` 命令优先用 session memory 作为免费摘要。

## Agent 能力

### 运行时架构

Agent 运行时采用分层设计，将单体循环拆分为独立运行器：

- **loopRunner** — 主循环驱动，管理生命周期状态（idle/running/steering/stopped）
- **turnRunner** — 单轮处理：PreTurn hooks → 模型推理 → 工具执行 → PostTurn hooks
- **modelRunner** — LLM 调用封装，含重试、流回调、响应分类（chat/tool_call/garbled/error）
- **toolRunner** — 工具执行编排，含配额过滤、结果收集、子代理槽位管理

**中断机制**：`Steer()` 支持处理中注入新消息打断当前 LLM 调用，`replaceContext()` 支持上下文替换重试。

**子代理槽位**：`slotManager` 管理最多 8 个并发子代理，通过回调路由分发结果。

### 工具清单

| 工具 | 功能 | 安全等级 | 执行模式 |
|------|------|----------|----------|
| **bash** | Shell 命令（只读命令自动 Safe） | Safe～Forbidden | Sequential |
| **read** | 文件读取 + 二进制检测 + 文件未找到建议 | Safe | Parallel |
| **write** | 文件创建/覆盖（先读后改强制） | Write | Sequential |
| **edit** | oldString/newString 内容锚定替换 + diff preview + gofmt 语法检查 | Write | Sequential |
| **list** | 目录列表 | Safe | Parallel |
| **glob** | 文件模式匹配（支持 **） | Safe | Parallel |
| **grep** | ripgrep 内容搜索 | Safe | Parallel |
| **web_search** | Web 搜索 + 强制 Sources 引用 | Safe | Parallel |
| **web_fetch** | 网页抓取 + HTML→Markdown 转换 | Safe | Parallel |
| **tree** | 目录树可视化 | Safe | Parallel |
| **index** | 代码索引查询（symbol/deps/file/search/skeleton） | Safe | Parallel |
| **image_gen** | AI 文生图（即梦 Jimeng t2i_v31）· 自动下载保存 | Safe | Sequential |
| **question** | 向用户提问（单选/多选/自定义输入） | Safe | Sequential |
| **task** | 子 agent 委派 | Safe | Parallel |
| **todo_write** | 任务列表更新 | Safe | Sequential |
| **skill** | 技能包加载 | Safe | Sequential |

### Question 工具

Agent 在需要用户决策时通过 `question` 工具发起结构化提问——当存在多种合理解读、需要偏好选择、或需要澄清时主动询问。

**参数**：`questions` 数组，每个包含 `question`、`header`、`options`、`multiple`（多选）、`custom`（自定义输入）。

**交互**：通过回调与 UI 通信——TUI 弹出 `question_bar`，GUI 弹出 `QuestionDialog`。支持单选、多选、自定义输入、拒绝。

### 子 Agent 类型（3 种）

| 类型 | 用途 | 工具 |
|------|------|------|
| **executor** | 执行编码任务 | read/write/edit/bash/grep/glob/list/question |
| **verify** | 验证修改 | read/grep/glob/list/bash |
| **researcher** | 代码探索/调研 | read/grep/glob/list/web_search/web_fetch/question |

子 agent 通过独立 LLM 客户端运行（共享上下文窗口 128K、接入 Compactor），edit 操作需用户确认。Handoff 机制支持上下文传递。

### MCP 管理

支持通过 MCP（Model Context Protocol）接入外部工具服务器，扩展 Agent 能力。

**协议**：JSON-RPC 2.0 over stdin/stdout，协议版本 `2024-11-05`。

**生命周期**：`NewClient()` → `Start()`（启动子进程 + initialize 握手）→ `ListTools()`/`CallTool()` → `Close()`。

**工具命名空间**：`{clientName}__{toolName}`（如 `github__search_repos`），防止与内置工具冲突。

**超时**：默认 15 秒。错误通过 `isError` 字段区分工具错误和协议错误。

**管理**：GUI 技能管理面板的 MCP 标签页支持搜索、过滤、状态监控（ready/error/disabled/starting）。

### 声明式权限引擎

权限系统从简单的 bash 危险命令分级升级为通用声明式权限引擎，适用于所有工具（bash、edit、read、web_fetch 等）。

**规则结构**：每条规则包含 `effect`（allow/ask/deny）、`specifier`（匹配器类型 + 模式）、`priority`。

**三层规则来源**：
1. **Builtin** — 硬编码安全规则（sudo/eval/dd/mkfs/ssh 硬拒绝，rm/kill/git push 询问，ls/cat/head 允许）
2. **Declared** — 用户 `config.json` 中声明的自定义规则
3. **Remembered** — 用户通过确认框批准的持久化规则

**优先级**：deny > ask > allow，deny 永远不可覆盖。

**匹配器类型**：
- `BashMatcher` — 命令前缀通配（如 `npm run *`）
- `PathMatcher` — 文件路径匹配
- `DomainMatcher` — 域名匹配（web_fetch）

**持久化**：`Store` 持久化 grants 和 remembered rules，跨会话生效。

**确认等级**：`[safe]` 自动放行、`[modify]`/`[danger]` 弹框确认、`[blocked]` 直接拒绝。

### 沙箱系统

bash 命令执行采用多层沙箱隔离，与权限引擎独立互补——权限决定"能不能做"，沙箱决定"能做多少"。

**Linux 三层策略**（按优先级回退）：
1. **原生命名空间** — pivot_root + 6 个命名空间隔离（user/mount/net/pid/ipc/uts），最严格
2. **Landlock** — 仅文件写保护，内核不支持命名空间时回退
3. **主机执行** — 无沙箱，作为最后手段

**非 Linux**：使用跨平台沙箱库（macOS sandbox-exec / Windows Low Integrity Level）。

**BashProfile**：控制工作区范围、网络访问开关、缓存路径白名单（npm/pnpm/yarn/go/cargo）。

**UnavailableError**：沙箱不可用时抛出，作为调用者请求主机执行权限的信号。

### 并行工具执行

互不依赖的工具并发执行，worker pool 上限 10。并行启动前检查 ctx 取消状态。subagent 共享同一个 Executor 实例。

### Hook 治理

Agent 运行时通过钩子系统在关键节点注入策略控制，分为内置钩子和插件钩子两类。

**内置钩子（9 个）**，由 `Policy` 在 Agent 循环的关键节点触发：

| 钩子 | 触发点 | 职责 |
|------|--------|------|
| QuotaHook | PreTurn | 配额限制检查 |
| ToolResultGuardrailHook | PreModelRequest | 工具结果护栏 |
| ReadBeforeWriteHook | PreToolUse | 写前必读强制 |
| ReadOnlySpiralHook | PostTool | 只读螺旋提醒 |
| VerificationHook | PostTurn | 未完成任务最终检查 |
| ExplorationExhaustedHook | PreTurn | 探索耗尽提醒 |
| ExploreCascadeHook | PostTool | 探索级联提醒 |
| ProgressStallHook | PostTool | 进度停滞提醒 |
| GarbledCircuitBreaker | PostTurn | 乱码断路器 |
**钩子事件点**：PreTurn → PreModelRequest → PreToolUse → PostToolUse → PostTool → PostTurn

**钩子动作**：`Hint`（建议）、`BlockTool`（阻止工具）、`RequireTool`（强制工具）、`BlockFinal`（阻止最终回复）

运行时只调用 `Policy.BeginTurn/BeforeModel/BeforeTool/RecordTools/AfterTurn`。
Hook 读取类型化 `Facts`，去重状态按 hook 隔离，不共享字符串 key。

**插件钩子**：用户可通过 JSON 配置自定义钩子，支持 shell 命令和 JavaScript 两种执行方式。事件点包括 PreToolUse、PostToolUse、PostToolUseFailure、UserPromptSubmit、SessionStart、Stop。

## 幻觉防治

基于纵深防御思想，在 6 个代码层面实现多层防幻觉机制，辅以 prompt 级设计补充：

### 代码层

- **第 1 层 — 工具安全**: 统一 permission engine、bash 命令规则匹配、路径验证、二进制检测、URL 内网 IP 拒绝
- **第 2 层 — 执行拦截**: deny 直接拒绝、ask 弹框确认、remembered allow 持久化、先读后改强制校验
- **第 3 层 — 输出完整性**: 工具结果边界标记、输出截断（2000行/50KB）、Garbled tool call 过滤
- **第 4 层 — Agent 循环控制**: 末日循环检测、收益递减检测、搜索断路器、finish_reason=length 处理
- **第 5 层 — 上下文保真**: 关键约束锚定、摘要二次验证、五级自动压缩、孤儿消息过滤
- **第 6 层 — LLM 调用控制**: 跨协议 thinking 开关控制、子 Agent thinking 强制关闭、token 超限时自动降级

### Prompt/设计级补充

- System Prompt 反幻觉指令（禁止生成 URL、忠实报告、推理长度限制）
- System Prompt 编码纪律（借鉴 Karpathy 观察的 LLM 编码四大毛病）：
  - **Think Before Coding**：先列假设、暴露困惑、多义性时让用户选、发现更简单方案主动 push back
  - **Simplicity First**：不添加超出需求的功能/抽象/配置，自检"senior 会觉得过度设计吗？"
  - **Surgical Changes**：只触碰请求要求的部分，禁止顺手重构/改注释/改格式，每一行改动追溯到用户请求
- 子 Agent prompt 同步落实上述纪律（executor 强调外科手术式改动，verify 增加简洁性审查）
- verify agent 格式强制 + 自检清单
- Session memory 模板警告（"记忆说 X 存在 ≠ X 现在存在"）
- web_search/fetch 的 Sources 引用格式要求
- edit 组内联展开（diff 一次可见，无需二次折叠）
- bash 复杂命令显示截断（只展示首行 + …）
- edit 后自动 gofmt 检查，发现语法错误及时提示

### 跨目录编辑

编辑工具允许操作工作目录外的文件——`validatePath` 不再拒绝跨目录路径，确认系统负责用户同意。危险等级依据命令类型分级，而非路径位置。


### 设计原则

- **Ground Everything** — 每个决策锚定在可验证的现实中（文件系统、命令输出、URL 来源）
- **Assume Deception** — 任何 LLM 输出（包括子 agent）都可能包含幻觉，需独立验证
- **Make It Checkable** — 所有输出格式服务于可验证性（file_path:line_number、Sources、Command run）
- **Fail Loudly** — 幻觉不能被静默：先读后改违规 → 报错，末日循环 → 强制停止，二进制 → 明确拒绝
- **Budget Reasoning** — 推理有成本：按任务类型限制思考长度，禁止在未读代码前凭空分析
- **Self-Serve First** — 主 Agent 优先自己完成任务，子 agent 仅在满足三个条件（5+ 文件跨包 / 独立上下文 / 单回合确实太复杂）时才使用
- **Progressive Compression** — 上下文逐级压缩，不急丢信息：先微压缩后完整压缩，优先用 session memory 做免费摘要
- **Anchor & Verify** — 压缩时锚定关键信息，压缩后二次验证保真度，确保压缩不是"遗忘"
- **Know The Project** — 会话启动时自动发现 NEKOCODE.md，一次性预加载项目约定，后续所有对话受益

## GUI 技能管理

GUI 提供技能管理面板（`SkillPanel`），三个标签页统一管理扩展能力：

- **Skills** — 按名称/描述/目录/插件搜索，过滤（all/loaded/builtin/local/plugin），显示加载状态
- **Plugins** — 按名称/描述/目录搜索，过滤（all/enabled/disabled），显示启用状态、关联技能/代理/命令/MCP 服务器/钩子
- **MCP** — 按名称/插件/命令搜索，过滤（all/enabled/disabled/config/plugin），显示状态（ready/error/disabled/starting）

通过 Wails 与 Go 后端通信，支持启用/禁用插件、刷新管理视图。

## 非交互模式

```bash
nekocode "帮我看看当前目录有什么文件"
```
