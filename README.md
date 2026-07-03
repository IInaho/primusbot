# NekoCode 🐱

<p align="center">
  <b>终端里的猫娘 AI 助手 &nbsp;·&nbsp; MIT 开源 &nbsp;·&nbsp; Go 单二进制</b>
</p>

<p align="center">
  <sub>多模型 · Agent 循环 · 子 Agent · TUI + GUI 双前端 · Plugin / MCP / Skill 生态 · Hook 治理 · 权限沙箱</sub>
</p>

<p align="center">
  <img src="docs/images/tui1.png" width="45%" alt="聊天界面">&nbsp;
  <img src="docs/images/gui1.png" width="45%" alt="GUI 界面">
</p>

---

## 这是什么

NekoCode 是一个运行在终端里的 AI 编程助手。你像聊天一样交代任务，它读代码、改文件、跑命令、搜资料，帮你把事情做完。

**不锁定供应商** — 同时支持 Anthropic 原生协议和 OpenAI 兼容协议（DeepSeek、MiniMax 等），一个 `/model` 就能切换。

**两套界面，一个核心** — TUI（终端）和 GUI（桌面窗口）共享同一套 Bot 引擎，换壳不改逻辑。

---

## 亮点

<table>
<tr>
<td width="50%">

### 🧠 Agent 循环
- Reason → Execute → Feedback 三轮循环
- 并行工具调度，自动判断依赖
- Mid-run BTW 中断：随时插入新指令
- 子 Agent 委派（executor / researcher / verify）
- **Plan Mode**：`/plan` 进入只读探索，方案审批后才执行
- **TodoWrite**：Agent 自动拆解任务，TUI 主题化渲染

### 🛡️ 多层安全防护
- **权限规则引擎** — deny → ask → allow 三级，bash 命令通配 / 文件路径 gitignore / 域名匹配
- **沙箱隔离** — Linux namespace 六重隔离（user/mount/net/pid/ipc/uts）+ Landlock 文件写保护回退
- 没读过的文件不准改，二进制文件不碰
- `sudo`、`ssh`、`dd`、`| bash` 直接拒绝
- LLM 输出的垃圾自动过滤，不写进文件
- 改完 Go 文件自动跑 gofmt，语法有错立刻提示

### 🪝 Hook 治理系统
- 8 种事件点：PreTurn / PreModelRequest / PreToolUse / PostToolUse / PostTool / PostTurn / UserSubmit / Stop
- 12 内置 Hook：配额预警、读前写检查、探索耗尽检测、进度停滞告警、乱码熔断、完成质量校验等
- 插件 `hooks.json` 声明式配置 + JS runner
- **Agent 治理层**：Ledger 全记录 + 语义分类器 + 探索预算追踪

### 🔧 工具系统
- 16 内置 + 条件/动态注册（read / write / edit / list / tree / glob / grep / diff / bash / web_search / web_fetch / task / todo_write / question / project_info / image_gen）
- 统一权限引擎（内置规则 + 用户声明 + 记住授权）
- oldString/newString 内容锚定编辑 + gofmt 自动检查
- Web 搜索/抓取、图片生成、代码索引

</td>
<td width="50%">

### 📦 生态扩展
- **Plugin** — GitHub / 本地路径安装，Claude Code 插件兼容
- **MCP** — JSON-RPC 2.0，外部工具服务器接入
- **Skill** — YAML 定义技能包，`/<skill>` 一键加载

### 💾 会话记忆
- `/new` 新对话保留上一轮摘要，不消耗 API token
- 会话存档/恢复，`/sessions` 管理
- `NEKOCODE.md` 项目上下文自动发现 + `@include` 递归加载
- `~/.nekocode/memory.md` 手动维护项目记忆，自动注入上下文
- 五级智能压缩（Normal → Warning → MicroCompact → Compact → Blocking）

### 🎨 双前端
- **TUI** — Bubble Tea + Lip Gloss，终端原生体验
- **GUI** — Wails v2 + React 18 + Tailwind CSS 4
- 共享 `bot.UI` / `bot.GUI` 契约，TUI/GUI 不直接依赖 bot 内部实现

### 🏗️ 工程基础
- 纯 Go SQLite（零 CGO），单二进制部署
- Tree-sitter 多语言代码索引（Go/JS/TS/Python/Rust）+ FTS5 全文搜索
- 全局调试日志、文件缓存（LRU）、Token 预算管理
- Thinking 跨协议统一控制（子 Agent / 摘要自动关闭，节省 token）
- AgentMD 解析：兼容 Claude Code `agents/*.md` 格式

</td>
</tr>
</table>

---

## 快速开始

> **前提**：Go 1.25.8+。GUI 还需要 Node/npm、Wails CLI，Linux 上需要 GTK/WebKitGTK。

### 安装

```bash
# 方式一：源码编译（TUI 版本）
git clone https://github.com/lznauy/NekoCode.git
cd NekoCode
go build -o nekocode-tui ./cmd
./nekocode-tui
```

### 配置

最小配置只需要 `active`、`context_window`、`models` 三项。完整字段见 [ARCHITECTURE.md](docs/ARCHITECTURE.md)。

```bash
mkdir -p ~/.nekocode
cat > ~/.nekocode/config.json << 'EOF'
{
  "active": "deepseek",
  "context_window": 128000,
  "models": [
    {
      "name": "deepseek",
      "provider": "deepseek",
      "api_key": "sk-your-key-here",
      "model": "deepseek-chat",
      "base_url": "https://api.deepseek.com/v1",
      "protocol": "openai"
    }
  ]
}
EOF
```

<details>
<summary>完整配置示例（含图片生成等可选字段）</summary>

```json
{
  "active": "deepseek",
  "context_window": 128000,
  "flash_model": "deepseek",
  "models": [
    {
      "name": "deepseek",
      "provider": "deepseek",
      "api_key": "sk-...",
      "model": "deepseek-chat",
      "base_url": "https://api.deepseek.com/v1",
      "protocol": "openai"
    }
  ],
  "image_gen_models": [
    {
      "name": "jimeng",
      "provider": "jimeng",
      "api_key": "AKLT...",
      "secret_key": "...",
      "model": "jimeng_t2i_v31"
    }
  ]
}
```

| 核心字段 | 说明 | 必填 |
|---------|------|:--:|
| `active` | 当前激活的模型名 | ✓ |
| `context_window` | 上下文窗口（token） | ✓ |
| `models` | 模型列表（provider/api_key/model/base_url/protocol） | ✓ |
| `flash_model` | 子 Agent / 摘要用的轻量模型 | |
| `image_gen_models` | 文生图配置（即梦/火山引擎） | |

</details>

### 运行 GUI（可选）

```bash
cd gui && npm install && npm run build && cd ..
wails build
./build/bin/nekocode-gui

# 或开发模式
wails dev
```

---

## 内置命令

| 命令 | 说明 |
|------|------|
| `/help` | 显示所有命令 |
| `/model [name]` | 列出或切换模型 |
| `/plan <任务>` | 只读探索 → 方案审批 → 执行 |
| `/new` · `/clear` | 新对话 / 清空历史 |
| `/context` | 上下文用量 / 详细分解 |
| `/summarize` | 手动压缩上下文 |
| `/config` | 当前 provider 和 model |
| `/plugin` | 插件安装/卸载/列表 |
| `/sessions` | 会话存档/恢复 |
| `/export` | 导出对话到 JSON |
| `/<skill>` | 加载技能（动态注册） |

输入 `/` 弹出补全菜单，`Tab` / `Shift+Tab` 选择。

---

## 权限与安全

### 权限规则引擎（deny → ask → allow）

所有工具调用经过统一权限引擎裁决，支持三级规则来源：

| 来源 | 说明 |
|:--|:--|
| **builtin** | 内置默认策略（sudo deny、rm ask、read allow） |
| **declared** | `config.json` 中 `permissions.allow/ask/deny` 声明 |
| **remembered** | 用户在弹框中批准并「记住」的规则，持久化到 `~/.nekocode/permissions.json` |

规则匹配支持 bash 命令通配（`npm run *`）、文件路径 gitignore 模式（`/src/**`）、域名匹配（`github.com`）。

### Bash 内置策略

| 规则效果 | 行为 | 示例 |
|:--|:--|:--|
| `allow` | 自动放行 | `ls` `cat` `git log` `go version` |
| `ask` | 弹框确认，可记住 | `rm` `chmod` `git push` `git reset --hard` |
| `deny` | 直接拒绝 | `sudo` `ssh` `dd` `| bash` |

未匹配的 Bash 命令默认询问；批准并选择「记住」后会写入当前项目的 remembered allow 规则。

### 沙箱隔离（Linux）

Bash 命令在沙箱中执行，双层回退：

| 后端 | 隔离能力 |
|:--|:--|
| **Native** | user/mount/net/pid/ipc/uts 六重 namespace + pivot_root 文件系统隔离 |
| **Landlock** | 文件写保护（仅 workspace 可写），作为 Native 不可用时的回退 |

沙箱不可用时降级为请求用户授权后主机执行。

---

## 文档

| 文档 | 内容 |
|------|------|
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | 完整目录结构、Agent 循环、工具系统、Hook/Plugin/MCP 实现细节 |
| [DESIGN.md](docs/DESIGN.md) | 交互设计、TUI 视觉规范、上下文策略、防幻觉设计原则 |
| [PLAN.md](docs/PLAN.md) | 开发路线图：已完成功能 & 计划中 |
| [FEATURE_GAP.md](docs/FEATURE_GAP.md) | 与 Claude Code 的功能对比分析 |

---

## License

MIT License
