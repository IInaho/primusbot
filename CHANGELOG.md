# 更新日志

## v0.5.0 - 2026-08-01

### 重点更新

- 重构 Shell 任务生命周期：短命令直接返回，长命令自动转为按会话托管的后台任务；新增 `process` 工具，支持事件式 `wait`、`watch`、`list` 和 `stop`，并提供硬超时、输出截断和可靠关闭。
- 加强会话与权限隔离：工作区授权改为每个 Bot 独立管理，临时目录权限按会话隔离；切换、清理或删除会话以及变更沙箱权限前，会先停止归属该会话的托管进程。
- 改进上下文可靠性：稳定系统提示与动态环境信息分层注入，恢复旧会话时自动使用当前规则；中断运行不再丢弃已经完整提交的消息和工具结果。
- 改进上下文压缩与子 Agent：强化摘要的提示注入边界、失败保留和合并策略，更新 executor、researcher、verify 的职责与验证要求，并让子 Agent 实时获取工作区环境。
- 完善工具 JSON Schema：支持 enum、array items、嵌套 object properties 和 required 字段；`question`、`todo_write`、`task` 及 MCP 工具获得结构化参数定义。
- 统一 OpenAI 与 Anthropic 的多段 system context 组装，并改善权限失败反馈、一次性授权持久化边界和运行时提示。

### 体验与安全修复

- TUI 命令建议按 Enter 后只补全命令并保留参数输入，不再直接发送未填写参数的命令。
- 优化 Shell/Process 工具记录的名称、摘要、长输出首尾展示和 ANSI 控制序列清理。
- 文件工具的读取、写入与预览统一使用请求所属的工作区权限，避免 Bot 或会话之间共享授权状态。
- `web_fetch` 忽略进程代理变量，确保目标地址经过实际连接 IP 的 SSRF 校验。
- 会话加载改为先验证、清理旧运行时资源，再激活新快照；沙箱权限变更前会先回收旧权限下启动的进程。

### 开发者注意

这是一个包含 Go API 破坏性变更的次版本更新：

- 内置工具注册入口改为 `catalog.NewToolbox`，调用方应在退出时调用 `Toolbox.Close`。
- 原 `shell` 的后台任务管理能力迁移到独立的 `process` 工具。
- `session.Manager.Resume` 拆分为 `Load` 与 `Activate`，用于在切换会话前执行可失败的资源清理。
- `Bot.NewSession` 和 `ShellTool.Shutdown` 现在返回错误；`prompt.Builder.Build` 拆分为 `BuildStatic` 与 `BuildEnvironment`。
- 原 package-level workspace 授权 API 替换为每个 registry 独立的 `workspace.Manager`。
- `todo_write.todos` 从 JSON 字符串升级为结构化数组参数。

### 运行时说明

- 未设置 `shell.timeout_ms` 的托管进程会持续运行，直到进程自行退出、调用 `process stop`、关闭所属会话或 NekoCode 退出。不确定任务生命周期时建议显式设置硬超时。
