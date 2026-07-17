# NekoCode 功能对比报告

> 基于对 `/home/lznauy/precode/NekoCode` 源码的逐模块审查，与 Claude Code 成熟能力进行对比，分析 NekoCode 当前已实现与缺失的功能。

---

## 一、已实现功能总览

### 工具系统（14 个内置工具 + 3 个条件/动态工具）

| 工具 | 文件 | 说明 |
|------|------|------|
| `bash` | shell/tool_bash.go | Shell 执行；多层沙箱隔离（Linux 原生命名空间 + Landlock，非 Linux 跨平台沙箱）；权限由统一 permission engine 决定 |
| `read` | filesystem/read/tool_read.go | 文件读取（文本/图片/PDF），支持行范围，输出 `[path#TAG]` 和行号内容 |
| `write` | filesystem/write/tool_write.go | 文件创建/覆写，自动创建父目录 |
| `edit` | filesystem/edit/tool_edit.go | oldString/newString 内容锚定编辑，唯一匹配校验，自动快照 + 撤销 + gofmt lint |
| `list` | filesystem/list/tool_list.go | 目录列表 |
| `tree` | filesystem/tree/tool_tree.go | 目录树，支持深度/条目限制 |
| `glob` | filesystem/search/tool_glob.go | 文件 glob 匹配，支持 `**` 递归 |
| `grep` | filesystem/search/tool_grep.go | 内容搜索（优先 rg，fallback grep），支持正则 + glob 过滤 + 上下文行 |
| `web_search` | web/tool_websearch.go | 网页搜索 |
| `web_fetch` | web/tool_webfetch.go | 网页抓取，URL 验证 + 重定向限制 |
| `question` | question/tool_question.go | 向用户提问（结构化问题，支持单选/多选/自定义输入，等待回答） |
| `diff` | diff/ | Unified diff between two texts or files (preview changes, compare versions) |
| `todo_write` | todo/tool_todo.go | 任务列表管理，支持 GUI/TUI 回调 |
| `task` | tasktool/tool_task.go | 子 Agent 调度（researcher/executor/verify） |
| `index` | tools/builtin/index/tool.go | 代码索引查询（符号/文件/依赖/全文搜索）— 条件注册 |
| `image_gen` | media/tool_image_gen.go | 图片生成（多模型配置）— 条件注册 |
| `skill` | extension/skill/tool_skill.go | 技能加载工具 — 动态注册 |
| MCP 工具 | extension/mcp/tool.go | 动态注册的 MCP 服务器工具 |

### Agent 系统

- **运行时架构**：分层设计 — loopRunner（主循环）→ turnRunner（单轮）→ model.Runner（LLM 调用）+ toolrun.Runner（工具执行）
- **主循环**：消息驱动，PreTurn → Reason → Execute → PostTurn → Stop 完整生命周期
- **推理器**：LLM 调用 + 响应解析，支持 tool calls 和纯文本两种模式，响应分类（chat/tool_call/garbled/error）
- **工具执行**：quota 过滤 → PreToolUse hook → 执行 → PostToolUse hook → 结果反馈
- **子 Agent**：SlotManager 管理最多 8 个并发子代理，支持 researcher/executor/verify 三种类型，文件缓存隔离
- **中断机制**：Steer() 支持处理中注入新消息打断当前 LLM 调用，replaceContext() 支持上下文替换重试
- **预算管理**：ExplorationTracker 衰减分数机制（200 分起，工具扣分，edit 恢复）
- **账本记录**：Ledger 追踪读取/修改文件、被阻止工具、验证结果
- **策略分类**：Semantics 分类（Exploratory/Mutating/Verifying/SourceProducing 等）
- **安全防护**：maxAgentSteps=150、maxConsecutiveHints=3、maxConsecutiveFailures=5、maxFinalCheckHints=2
- **Edit 后 Lint**：`.go` 文件编辑后自动 gofmt 检查，发现语法错误及时注入提示

### 上下文管理

- **分层架构**：Layer 0（系统提示词 + 记忆）→ Layer 0.5（Archive 摘要）→ 消息层
- **自动压缩**：Head-Tail-Summary 重建，保留最近 3 轮，旧消息 LLM 摘要
- **Token 追踪**：估算 token 用量，触发自动压缩
- **持久化记忆**：结构化 markdown 文件（Tech Stack / Active Goals / Completed Tasks / Architecture Map / Preferences）
- **子 Agent 上下文**：独立 Manager，可注入项目上下文 + 工作目录

### Hook 系统

- **5 个 Hook 点**：PreTurn、PreModelRequest、PreToolUse、PostTool、PostTurn
- **Hook 能力**：注入 Hint、阻止工具（BlockTool）、要求工具（RequireTool）、阻止最终输出（BlockFinal）
- **内置 Hook**：10 个（quota / tool_result_guardrail / read_before_write / read_only_spiral / verification / exploration_exhausted / explore_cascade / progress_stall / garbled_circuit_breaker / final_check）
- **插件 Hook**：支持从外部插件加载声明式 Hook，支持 shell 命令和 JavaScript 两种执行方式

### TUI 界面

- **框架**：Bubble Tea + Lipgloss
- **Markdown 渲染**：chroma/glamour 库，tokyo-night 主题
- **Diff 预览**：edit 工具结果中展示增删行（绿色/红色背景）+ 结构化 diff 模型
- **鼠标支持**：滚轮滚动消息列表
- **命令补全**：Tab/Shift+Tab 选择，`/` 弹出补全菜单
- **组件**：消息列表、输入框、确认栏、滚动条、块渲染、处理状态、splash 屏
- **子 Agent 显示**：颜色编码的子 Agent 状态

### 其他

- **代码索引**：tree-sitter 多语言解析 + 图数据库 + 符号/依赖/全文搜索
- **沙箱系统**：Linux 双后端（原生命名空间 pivot_root + Landlock），非 Linux 跨平台沙箱库
- **权限引擎**：声明式规则引擎，三层来源（builtin/declared/remembered），BashMatcher/PathMatcher/DomainMatcher
- **插件系统**：manifest 解析 + 命令注册 + Hook 注册 + 子 Agent 注册
- **技能系统**：bundled 技能 + 文件加载 + 工具化 + 上下文注入
- **命令系统**：`/help`、`/clear`、`/summarize`、`/new`、`/context`（含原 /stats 功能）、`/config`、`/model`、`/plan`、`/plugin`、`/sessions`、`/export` 共 11 个命令（含动态 skill 命令）
- **MCP 客户端**：stdio 子进程模式，JSON-RPC 通信，工具发现
- **LLM 层**：Anthropic + OpenAI 双协议，流式 API，重试机制
- **配置**：provider/model/apiKey/baseURL + 图片生成模型配置
- **结构化 Diff 模型**：`EDIT_PREVIEW_JSON_B64` base64 编码 diff，TUI 直接解析渲染

---

## 二、功能缺失清单

### 🔴 P0 — 阻碍基本可用性

#### 1. Bash 安全机制 — 沙箱执行 ✅ 已实现

```
当前：统一 permission engine（deny / ask / allow）+ 多层沙箱隔离
已实现：
  ✅ 沙箱执行 — Linux 双后端（原生命名空间 pivot_root + Landlock 文件写保护），
     非 Linux 使用跨平台沙箱库（macOS sandbox-exec / Windows Low Integrity Level）
  ✅ BashProfile — 控制工作区范围、网络访问、缓存路径白名单
  ✅ UnavailableError — 沙箱不可用时请求主机执行权限
仍缺失：
  ❌ 路径约束检查 — 无法限制文件访问范围（如只允许项目目录内操作）
说明：沙箱执行已完整实现，路径约束是下一层细化。
```

#### 2. 权限系统 — 声明式引擎已升级

```
当前：统一 permission engine（deny / ask / allow）+ remembered allow + 声明式规则匹配
已实现：
  ✅ 声明式规则引擎 — 三层来源（builtin/declared/remembered），BashMatcher/PathMatcher/DomainMatcher
  ✅ 规则持久化 — Store 持久化 grants 和 remembered rules
  ✅ 沙箱能力授权 — 沙箱不可用时请求主机执行权限，用户确认后授权
仍缺失：
  ❌ 权限审计视图 — 用户缺少集中查看/撤销 remembered rules 的界面
说明：权限引擎已从简单危险命令分级升级为通用声明式引擎，沙箱授权已打通。审计 UI 是下一步。
```

#### 3. CLI 主入口仍需完善

```
当前：TUI (cmd/nekocode-tui) + GUI (main.go) 两个入口
缺失：
  ❌ 子命令系统（version/help/config 等）
  ❌ 命令行参数解析（--model, --config, --debug）
说明：--version/--help 属于工程基本功，即便桌面 UI 为主入口也值得补上。
```

#### 4. 工具种类不足

```
已有 14 内置 + 3 条件/动态工具，真正缺失的关键工具：
  ❌ LSP 工具 — 跳转定义、查找引用、诊断（当前 index 项目可做符号搜索，但不具备 LSP 的编辑器协议）
  ❌ task 子工具 — task_list/get/update/stop/output（当前只有 task 创建）
说明：MCP 资源工具（list/read_mcp_resource）和配置类工具（schedule_cron、config 工具）属于产品化阶段诉求，列入 P2。
```

---

### 🟡 P1 — 影响核心体验

#### 5. TUI 功能不完整

```
已有：Markdown 渲染（glamour + chroma 语法高亮）、diff 预览、鼠标滚轮、命令补全、基础组件、
      token 用量仪表盘（/context 显示 bar + used/total + sys/tools/todo/skills/msgs 分解）、
      顶栏状态区（model/tokens/compact count 等实时状态）、tree/glob/grep 工具
真正缺失（按体验影响排序）：
  ❌ 搜索界面 — 无交互式搜索结果浏览（grep 输出未做 TUI 化）
说明：
  - 文件树浏览器、多面板布局属于对标 IDE 的交互范式，TUI 流式对话不是 IDE 形态，不应照搬。
  - 进度指示器：长时间操作已被 per-turn spinner 覆盖，不必重做。
  - 顶栏已有 token/model/状态显示，底部快捷键栏锦上添花，不值得成为 P1。
```

#### 6. LLM 层功能不足

```
已有：Anthropic + OpenAI 双协议、流式 API、重试机制、Thinking 跨协议控制、
      API 精确 token 计数（prompt_tokens/completion_tokens 由服务端返回，tracker 自动校准）
真正缺失：
  ❌ 模型 Fallback — API 失败时无法自动切换备用模型（多模型配置场景刚需）
  ❌ Google Gemini 支持 — Gemini 协议差异大（response schema/流式协议），OpenAI 兼容层覆盖不住
说明：
  - 模型路由（按任务类型自动选模型）：需要依赖 Fallback 机制可工作，单独列无意义。
  - 并发控制/限流/请求队列：单会话串行处理为主，当前不需要。
  - "精确 token 计数"已解决 — API 每轮已给真精确值；本地 tokenizer 只在两次调用间的 UI 缝隙里有用，不值得引入依赖（API 已够准）。
```

#### 7. 上下文管理不完整

```
已有：Head-Tail-Summary 压缩、持久化记忆（在 /new /clear /compact 等关键节点同步写入 memory）、
      token 追踪、五级预警、micro-compact 优先级裁剪（priorityLow/Medium/High）、
      分级 memory 分类（Tech Stack / Active Goals / Completed Tasks / Architecture Map / Preferences）、
      Archive/Layer 0.5 摘要层
真正缺失（按成本排序）：
  ❌ 摘要验证 — 代码仅实现 XML block 提取（FormatCompactSummary），无实际质量校验逻辑
说明：
  - "记忆自动更新"已实现（在会话关键节点同步写入 memory）；DESIGN.md 所言"10k token 后异步提取"是"愿景文档"措辞，当前设计"在边界触发点同步写入"是合理的实现选择，不算缺失。
  - 智能裁剪/分层上下文/RAG：需要向量检索和语义匹配基建，属于大型架构改动，当前阶段过度设计。
```

#### 8. 命令系统可扩展

```
已有（11 个命令，含动态 skill）：
  /help、/clear、/summarize、/new、/context（含原 /stats 功能）、/config、/model、/plan、/plugin、/sessions、/export
真缺失（开发场景高频）：
  ❌ /commit — 自动 Git commit
  ❌ /review — 代码审查
  ❌ /diff — 工作区变更
说明：
  - 命令别名、命令历史搜索、权限分级属于体验优化，不是命令缺位。
```

---

### 🟢 P2 — 产品化完善

#### 9. MCP 客户端

```
已有：stdio 子进程模式 + JSON-RPC 工具发现
缺失：
  ❌ MCP 资源/提示 支持（当前仅 Tools，缺 list_mcp_resources / read_mcp_resource）
  ❌ SSE / StreamableHTTP 传输（连接远程 MCP 服务器时需求）
  ❌ OAuth 认证（连接商业化 MCP 服务时需求）
说明：
  - 多服务连接池、健康检查、自动重连、协议版本协商：是 SSE/HTTP 传输的附带需求。在 stdio 模式下不需要，只在切换传输模式时才成为真需求。
  - 服务发现：属于配套生态，不是核心传输能力。
```

#### 10. 插件与技能系统

```
已有：manifest 解析 + 命令/Hook/Agent 注册 + install/uninstall/enable/disable +
      bundled 技能 + 文件加载 + 工具化 + context 真正缺失：
  ❌ 技能参数 Schema 验证（当前基本信任技能定义）
  ❌ 技能单元测试框架
说明：
  - 插件市场/技能市场/包管理/依赖管理/自动更新：是社区运营设施，不是工具能力。
  - 沙箱隔离：重大架构改动，需专门评估。
```

#### 11. 会话管理

```
已有：创建 + 存储 + 列出 + 恢复 + 导出（已落盘会话通过 /sessions 查看）
缺失：
  ❌ 会话自动过期清理（长期运行的磁盘膨胀）
  ❌ 会话分支/合并
说明：
  - 会话历史浏览/搜索：当前按会话文件打开即是浏览，搜索诉求弱于 /sessions 列表。
```

#### 12. 配置系统

```
已有：provider/model/apiKey/baseURL + image_gen_models + ApplyConfig（GUI 保存配置即热重载，重新 reinit）
说明：
  - "热重载"已实现 — interaction/gui/app 的 SaveConfig 前端调 ApplyConfig 后端，动态替换 model/protocol/context_window 并重建 LLM client。
  - 分层覆盖 / 多环境 / Schema 验证 / 导出导入：属于多用户场景优化，单用户当前不急。
```

---

### 🔵 P3 — 运维与规模化（企业级，非当前阶段目标）

本节原 P2 列出的基础设施需求，例如：
- 统一日志框架（当前 debug.Log 已够用）
- 错误码体系 / 全局事件总线
- goroutine 工作池 / 请求限流器
- 健康检查端点 / 指标监控

这些**对标的是 Claude Code 百万用户级 SaaS 运维**，NekoCode 桌面单用户工具架构下不需要。保留此节作为"未来规模化的标竿"仅供方向参考，**不作为 P2 实施**。

---

## 三、NekoCode 的独特优势

相比 Claude Code，NekoCode 有以下亮点：

1. **代码索引系统（index）** — 自研 tree-sitter 多语言解析 + 图数据库，支持符号搜索、依赖分析和全文搜索，Claude Code 依赖 LSP 无此独立能力
2. **完善的测试覆盖** — 各模块均有测试代码，Claude Code 几乎无测试
3. **Go 语言实现** — 编译为单一二进制，部署简单，性能优异，内存安全
4. **架构清晰** — 模块边界明确，依赖关系简洁，易于理解和贡献
5. **双前端架构** — TUI + GUI 共享 Bot 核心，`bot.UI` / `bot.GUI` 契约解耦
6. **Hook 系统成熟** — 5 事件点 + 10 内置 Hook + 声明式插件 Hook
7. **纯 Go SQLite** — 零 CGO 依赖，简化交叉编译
8. **Edit Lint 集成** — Go 文件编辑后自动 gofmt 检查，防止语法错误积累
9. **品牌一致性** — 独立设计的猫娘主题 logo + 全套平台图标资源（icns/ico/png），GUI/TUI 视觉统一
10. **Prompt 编码纪律** — 针对 LLM 编码四大毛病（Karpathy 观察）的系统性 prompt 约束，覆盖主 agent + 子 agent + plan mode
11. **多层安全防护** — 沙箱隔离 + 声明式权限引擎 + Hook 护栏，纵深防御

---

## 四、优先级建议

```
P0（真阻碍使用）：
  1. CLI 入口完善（--version/--help + 子命令）
  2. 补充核心工具（LSP、task 子系统）
  3. 权限审计视图（集中查看/撤销 remembered rules）

P1（影响核心体验，应尽快实现）：
  4. LLM Fallback + Gemini 原生支持
  5. 摘要语义验证（FormatCompactSummary 之后加质量校验）
  6. /commit /review /diff 命令
  7. TUI 搜索界面（grep 结果 TUI 化）

P2（产品化，可逐步迭代）：
  8. MCP 资源 + SSE/HTTP 传输
  9. 技能 Schema 验证 + 测试框架
  10. 会话自动过期清理

P3（企业级，非当前阶段，仅供方向参考）：
  11. 规模化运维基建（日志/监控/请求池/限流等）
