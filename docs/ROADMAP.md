# 任务清单：Reasonix 借鉴落地

> 来源：2026-08 对 DeepSeek-Reasonix 的架构调研。按优先级排序，每个任务单独提交、单独可回滚。
> 执行顺序：T1 → T2 → T3 → T4；T5 看完前四个的效果再议。

## T1. MCP 恒定 schema 代理（缓存收口）

**目标**:MCP 工具上下线不再击穿前缀缓存。

- [x] 新增 `capability` 代理工具，schema 恒定：`action: list|inspect|call` + `server`/`tool`/`args` 参数
- [x] 代理后端接现有 MCP host；list 只读取已发现能力，call 在权限门放行后才实际调用远端工具
- [x] MCP 工具不再直接注册进 provider 工具表；工具 schema 排序 + 冻结的守护测试
- [x] 权限规则适配（代理调用按真实 server.tool 匹配 allow/ask/deny)
- [x] **验收**：运行中增删 MCP server，连续两轮请求 prefix 命中不掉（schema 恒定由测试锁定；真实命中用 DeepSeek 实测验证过前缀稳定性）

涉及：`bot/extension/mcp/`、`bot/agent/tools.go`、`bot/extension/tool/builtin/catalog/`

## T2. Checkpoint / Rewind（文件级回滚）

**目标**：每个用户回合一个锚点，可回滚代码改动。

- [x] `checkpoint` 包：回合锚点 + 写前快照存储（`~/.nekocode/checkpoints/<session>/<turn>/`)
- [x] 复用现有 writer 工具的 Preview/执行链，在权限与 workspace 校验后的统一执行切点抓写前状态，覆盖新增/修改/删除三态
- [x] bash 副作用不跟踪（权限层兜底，与 Reasonix 一致）
- [x] `/rewind [turn|list]` 命令 + 会话快照里存锚点索引；仅保留最近 10 个有效回合
- [x] **验收**：改 3 个文件后 rewind 到上一回合，文件逐字节恢复；新增文件被删除

涉及：`bot/checkpoint/`、`bot/extension/tool/runtime/runner/`、`bot/command/`、`bot/session/`

## T3. 缓存防退化

**目标**:prefix 稳定性变成可回归的资产。

- [x] system prompt 字节稳定性单测（禁时间戳、禁 map 乱序、禁环境探测 flap)
- [x] 模拟工具循环两轮 `BuildRequest()`：动态 context 以 tagged user 消息追加，上一请求必须是下一请求的完整前缀
- [x] prefix-shape 归因：缓存 miss 时记录 system/tools/history 哪段变了，进 `/context` 展示
- [x] **验收**：人为往前缀插入日期 → golden SHA-256 字节锁测试红

涉及：`bot/prompt/`、`bot/contextmgr/`、`bot/provider/types/`

## T4. 动态 Bash 结构分类（安全）

**目标**：嵌套/间接执行必须人工审批，任何自动机制不可代批。

- [ ] shell 解析（引 `mvdan.cc/sh`，项目已有依赖）分类：命令替换 / 动态命令名 / `eval` / `source` / `bash -c` / heredoc 内联代码
- [ ] 命中"嵌套执行" → 审批卡标注原因，remembered rule 只允许整条 literal，禁止 glob 化
- [ ] "记住授权"落为整条命令 literal（而非 specifier 放宽）
- [ ] **验收**:`bash -c "$(cat x)"` 必须人工批；`git status` 记住后不再问

涉及：`bot/extension/tool/runtime/permission/`、`bot/policy/`

## T5. 待定（看产品方向）

- [ ] 证据账本 + 结束门禁（Delivery 场景）
- [ ] run loop typed-pause（停滞/预算的暂停-恢复语义）
- [ ] 会话树（branch/fork)
- [ ] 工具分档（轻量模型 9 工具 + 惰性连接）
