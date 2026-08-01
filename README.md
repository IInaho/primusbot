# NekoCode 🐱

<p align="center">
  <b>你的猫娘 AI 编程助手 &nbsp;·&nbsp; MIT 开源 &nbsp;·&nbsp; 一个文件就能跑</b>
</p>

<p align="center">
  <sub>双界面 · 多模型自由切换 · 读代码 / 改文件 / 跑命令，全靠聊</sub>
</p>

<p align="center">
  <img src="docs/images/tui1.png" width="45%" alt="聊天界面">&nbsp;
  <img src="docs/images/gui1.png" width="45%" alt="桌面界面">
</p>

---

## 这是什么？

NekoCode 是一个装在终端里的 **AI 编程助手**，把你的代码项目变成"能对话的同事"。

你不需要记命令行、不需要翻文档，**像聊天一样说出你的需求**，它就能帮你：

- 📖 **看懂项目** — 快速理解陌生代码库，讲清楚某段逻辑在干嘛
- ✏️ **改代码** — 加功能、修 bug、重构，改完自动检查格式和语法
- 🖥️ **跑命令** — 执行构建、测试等命令，替你盯着输出
- 🔍 **查资料** — 联网搜索、抓取网页，不懂的知识现查现用
- 🎨 **生成图片** — 需要示意图、封面图，直接让 AI 画一张

> 💡 想先试别的模型？内置 DeepSeek 默认模板，改一行就能换成其他兼容接口（见下文）。

---

## 为什么选 NekoCode？

| 优势 | 说明 |
|------|------|
| 🆓 **零绑定** | 不锁死任何一家大模型。Anthropic、DeepSeek、MiniMax 等，只要能调 OpenAI 或 Anthropic 接口都能用；随时 `/model` 切换 |
| 🪶 **轻量不臃肿** | 单二进制文件，编译完直接运行，不需要装一堆依赖 |
| 🖥️ **两套界面随便挑** | 终端版（TUI）和桌面窗口版（GUI）同一套大脑，命令行党、图形界面党都顺手 |
| 🛡️ **改代码不闯祸** | 内置权限保护：危险命令自动拒绝、关键操作先问你、未读过的文件不允许乱改 |
| 🧠 **不怕聊爆** | 对话太长自动压缩、保留摘要，新对话能接着旧话题，不浪费钱 |
| 🧩 **能长能力** | 支持插件、技能包（Skill）、外部工具接入（MCP），缺什么补什么 |
| 🐱 **可可爱爱** | 猫娘主题，干活心情好（这是认真的） |

---

## 快速开始

### 第一步：一键安装（推荐）

在终端里执行一行命令，自动下载最新版并安装：

```bash
curl -fsSL https://raw.githubusercontent.com/lznauy/NekoCode/master/scripts/install.sh | sh
```

脚本会自动：

- 检测你的系统（Linux / macOS）和架构（Intel / Apple 芯片），下载对应版本
- 安装到 `~/.local/bin`（不需要 sudo；目录不可写时提示你处理）
- 完成后运行 `nekocode-tui` 即可启动

> 💡 想装特定版本？ `curl -fsSL ... | sh -s -- --version v0.5.0`
>
> 💡 想装到别的位置？ `curl -fsSL ... | sh -s -- --dir /你的/目录`

> Windows 用户：请通过 [WSL](https://learn.microsoft.com/windows/wsl/install) 运行，NekoCode 支持 Linux 和 macOS。

看到聊天界面，NekoCode 就跑起来了 🎉

<details>
<summary>备选一：手动下载（不想要脚本）</summary>

从 [Releases 页面](https://github.com/lznauy/NekoCode/releases) 下载对应平台的二进制：

| 平台 | 文件名 |
|------|--------|
| Linux Intel | `nekocode-tui-linux-amd64` |
| Linux Apple 芯片 | `nekocode-tui-linux-arm64` |
| macOS Intel | `nekocode-tui-darwin-amd64` |
| macOS Apple 芯片 | `nekocode-tui-darwin-arm64` |

下载后加执行权限直接运行：

```bash
chmod +x nekocode-tui-linux-amd64
./nekocode-tui-linux-amd64
```

</details>

<details>
<summary>备选二：源码编译（想自己动手）</summary>

需要 [Go 1.25.8+](https://go.dev/dl/)。

```bash
git clone https://github.com/lznauy/NekoCode.git
cd NekoCode
go build -o nekocode-tui ./cmd/tui
./nekocode-tui
```

</details>

### 第二步：填上你的模型 API Key（必须，只填一次）

NekoCode 内置了 DeepSeek 作为默认模型，但**不会自动生成配置文件**，也不会自动填 API Key。没有配置 = 没有 API Key = 聊天会报错。

首次使用请先创建配置文件并填上 Key（把 `sk-你的Key写这里` 换成真实的）：

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
      "api_key": "sk-你的Key写这里",
      "model": "deepseek-chat",
      "base_url": "https://api.deepseek.com/v1",
      "protocol": "openai"
    }
  ]
}
EOF
```

保存后重新启动 NekoCode，就可以开始聊天了。

> ✨ **想换模型？** 在 `models` 数组里加一个条目，然后在聊天里输入 `/model 名字` 就能切换。
> 支持任意 OpenAI 或 Anthropic 兼容接口，一张配置表就能管理多个模型。

### （可选）用桌面窗口版

桌面版暂未随 Release 发布，需要源码编译（需要 Node/npm 和 [Wails](https://wails.io/)）：

```bash
git clone https://github.com/lznauy/NekoCode.git && cd NekoCode
wails build
./build/bin/nekocode-gui
```

---

## 常用命令速查

聊天框里用 `/` 开头输入命令。忘了就输入 `/help`。

| 命令 | 作用 |
|------|------|
| `/help` | 查看全部命令 |
| `/model` | 列出或切换模型 |
| `/new` | 开新对话（自动带上上一轮的摘要，不丢失上下文） |
| `/clear` | 清空当前对话记录 |
| `/context` | 看看上下文用了多少 |
| `/plan` | 让 AI 先只读探索、给出方案，你批准了它才动手改 |
| `/plugin` | 管理插件 |
| `/sessions` | 管理保存的会话，随时回来接着聊 |
| `/export` | 把对话导出成 JSON 存档 |

输入 `/` 会弹出命令补全菜单，`Tab` 键选择，很方便。

---

## 这些细节，放心交给你

NekoCode 在背后做了很多安全工作，让你可以专注聊需求：

- 🔒 **危险命令自动拦** — `sudo`、`ssh`、`dd` 这类命令直接拒绝，`rm`、`git push` 这类先问你
- ✅ **批准一次记一次** — 你批准过的操作会被记住，下次不再反复打扰
- 🗂️ **只动该动的** — 没有读过的文件不允许修改，二进制文件不碰
- 🧹 **改错代码不沉默** — 改完 Go 文件自动做语法检查，写错了当场回报，不会悄悄改坏
- 💬 **长了会自动压缩** — 对话太长自动压缩摘要，新对话接着聊，省 token

> 想深入了解架构、命令与工具体系、如何扩展？技术文档见 [docs/](docs/)。

---

## 核心技术（写给懂行的人）

如果你关心架构含金量，下面是 NekoCode 的几个核心设计：

### 🧠 Agent 引擎

- **Reason → Execute → Feedback 三阶段循环**，每轮自动决策下一步，运行中随时可打断、可恢复
- **并行工具调度**：无依赖的工具调用自动并行，有依赖的自动串行等待
- **子 Agent 委派**：内置 executor（改代码）/ researcher（调研）/ verify（验证）三类专家 Agent，大型任务拆给它们并行干；子 Agent 不能无限嵌套，防止失控
- **Plan Mode**：`/plan` 进入只读探索，先出方案、你批准后才动代码

### 🛡️ 治理与安全

- **沙箱隔离**：Bash 命令在受限环境执行，Linux 上采用 namespaces → Landlock 多层回退，网络默认隔离、按需放行
- **权限规则引擎**：deny → ask → allow 三级裁决，规则来自内置策略 + 你的声明 + 你「记住」的授权，危险命令（sudo / ssh / dd 等）一律硬拒绝
- **Ledger 行为账本**：全程记录读/写/验证轨迹——改未读过的文件会被拦下，改完必须通过验证才信服
- **读前写检查**：要修改的文件必须先读过，防止 AI 瞎改
- **质量校验 Hook**：改完 Go 文件自动 gofmt 检查，语法错误当场回报

### 🪝 Hook 治理层

6 个稳定事件点（用户提交 / 模型调用前 / 工具调用前后 / 工具批次后 / 停止时）+ 9 个内置治理 Hook：配额预警、读前写检查、探索耗尽检测、进度停滞告警、乱码熔断、完成质量校验等。相当于给 Agent 装了一套「行为仪表盘」，可以精确观察和干预。

### 🧩 生态与扩展

- **Plugin**：GitHub / 本地路径安装，兼容 Claude Code 插件格式
- **Skill**：YAML 定义技能包，`$<skill>` 一键加载
- **MCP**：JSON-RPC 2.0 协议，接入任意外部工具服务器
- **代码索引**：Tree-sitter 多语言解析 + FTS5 全文搜索，让 Agent 能高效检索大型代码库

### 💾 会话与上下文

- **会话快照 / 恢复**：随时存档、随时回来接着聊，不丢上下文
- **记忆注入**：`~/.nekocode/memory.md` 手动维护的长期记忆自动注入对话上下文，跨会话生效
- **五级上下文压缩**：按剩余空间分级（正常 → 预警 → 微压缩 → 必压缩 → 阻塞），自动压缩旧对话为摘要、保留最近轮次，长聊不断线、不浪费 token
- **Thinking 统一控制**：子 Agent / 摘要等非核心场景自动关闭深度思考，省 token 不省质量

> 完整架构细节见 [ARCHITECTURE.md](docs/ARCHITECTURE.md)。

---

## 开源

NekoCode 使用 **MIT 许可证**，完全开源，随意使用、修改、分发。

---

## 给开发者

- [ARCHITECTURE.md](docs/ARCHITECTURE.md) — 整体架构、Agent 循环、工具系统
- [DESIGN.md](docs/DESIGN.md) — 交互设计、界面规范、上下文策略
- [RUNTIME_APP_GUIDE.md](docs/RUNTIME_APP_GUIDE.md) — 基于底座组装上层 AI 应用
- [RUNTIME_HTTP_API.md](docs/RUNTIME_HTTP_API.md) — Runtime HTTP/SSE 协议
