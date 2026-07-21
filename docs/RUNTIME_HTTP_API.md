# Runtime HTTP API

本文档描述当前 `runtime/httpapi` adapter 的稳定契约。HTTP 层只代理 Runtime Control Layer：输入、查询、连接器控制、approval/question 回复和事件订阅都转发到同一套 runtime。

## 通用规则

- JSON 响应 `Content-Type` 为 `application/json`，SSE 响应为 `text/event-stream`。
- POST 请求体必须是单个 JSON 文档；未知字段和尾随第二个 JSON 值都会返回 `400`。
- 错误响应统一为：

```json
{"error":"message"}
```

- 如果启用 `WithBearerAuth(handler, token)`，除 `GET /health` 外都需要认证。支持：
  - `Authorization: Bearer <token>`
  - `X-Nekocode-Token: <token>`

## Endpoints

| Method | Path | Description |
| --- | --- | --- |
| GET | `/health` | 健康检查 |
| POST | `/input` | 提交用户输入；idle 时创建 run，running 时由 runtime 转为 steer |
| GET | `/runs?limit=50` | 查询最近 run read model，`limit` 最大 200 |
| GET | `/runs/current` | 查询当前 run |
| GET | `/runs/{runID}` | 查询指定 run |
| GET | `/runs/{runID}/artifacts` | 查询指定 run 的 artifacts |
| POST | `/runs/{runID}/abort` | 中止指定 run |
| GET | `/events?run_id=&replay=1` | 订阅 SSE 事件；`replay=1` 先回放内存历史 |
| POST | `/approvals/{approvalID}` | 回复工具确认 |
| POST | `/questions/{questionID}` | 回复用户问题 |
| GET | `/connect` | 查询 connector 状态 |
| POST | `/connect/{name}` | 执行 connector connect/command |
| POST | `/disconnect/{name}` | 断开 connector |
| GET | `/model` | 查询当前 provider/model |
| GET | `/commands` | 查询可用 command 名称 |
| GET | `/stats` | 查询 token/运行统计 |
| GET | `/context` | 查询 context window 快照 |
| GET | `/memory?scope=project` | 查询 memory 只读视图 |
| GET | `/sessions` | 查询 session 列表 |
| GET | `/sessions/current/messages` | 查询当前 session 消息 |

## Requests

### `POST /input`

```json
{
  "text": "hello",
  "kind": "message",
  "source": {"kind": "http", "id": "client-1"},
  "sender": {"id": "user-1", "username": "alice", "display": "Alice"},
  "session_id": "optional",
  "reply_to": "optional"
}
```

`text` 必填且不能为空。`kind` 为空时按 `message` 处理。`source.kind` 为空时 HTTP adapter 会填为 `http`。

成功响应：

```json
{"run_id":"run_1"}
```

### `POST /connect/{name}`

请求体可为空。非空时：

```json
{"args":["status"]}
```

成功响应：

```json
{"message":"connector response"}
```

### `POST /disconnect/{name}`

成功响应：

```json
{"message":"connector response"}
```

### `POST /runs/{runID}/abort`

成功响应：

```json
{"ok":true}
```

### `POST /approvals/{approvalID}`

```json
{
  "allowed": true,
  "remember": false,
  "allow_with_permission": false
}
```

`allow_with_permission` 只在 runtime broker 标记该 approval 可升级权限时生效；否则会被 runtime 强制清为 `false`。

成功响应：

```json
{"ok":true}
```

### `POST /questions/{questionID}`

```json
{
  "answers": [["choice-a"], ["free-form answer"]],
  "rejected": false
}
```

`answers` 是二维数组，每个内层数组对应一个问题。拒答时传 `{"rejected":true}`。

成功响应：

```json
{"ok":true}
```

## Query Responses

### Run View

`GET /runs/current`、`GET /runs/{runID}` 返回：

```json
{
  "id": "run_1",
  "status": "running",
  "source": {"kind": "http"},
  "sender": {"id": "user-1"},
  "input": "hello",
  "phase": "Thinking",
  "started_at": "2026-07-19T00:00:00Z",
  "updated_at": "2026-07-19T00:00:01Z",
  "tools": [],
  "approvals": [],
  "questions": [],
  "output": "",
  "error": "",
  "event_count": 3
}
```

Run status 值：`idle`、`running`、`waiting_approval`、`waiting_question`、`done`、`failed`、`aborted`。

### Artifact View

`GET /runs/{runID}/artifacts` 返回：

```json
{
  "run_id": "run_1",
  "diffs": [],
  "patches": [],
  "reviews": [],
  "results": [],
  "errors": [],
  "events": 10
}
```

每个 artifact item：

```json
{
  "kind": "diff",
  "tool_name": "edit",
  "title": "edit preview",
  "content": "--- a/file\n+++ b/file\n",
  "created_at": "2026-07-19T00:00:00Z"
}
```

### Approval View

Run view 中的 `approvals` item：

```json
{
  "id": "apr_1",
  "tool_name": "shell",
  "args": {"command":"go test ./..."},
  "args_hash": "sha256-hex",
  "tool_call_hash": "sha256-hex",
  "kind": "permission",
  "can_escalate_permission": true,
  "status": "pending",
  "created_at": "2026-07-19T00:00:00Z",
  "source": {"kind":"runtime"}
}
```

Approval status 值：`pending`、`approved`、`rejected`、`expired`。

### Question View

Run view 中的 `questions` item：

```json
{
  "id": "q_1",
  "questions": [
    {
      "header": "Choice",
      "question": "Pick one",
      "options": [{"label":"A","description":"Use A"}],
      "multiple": false,
      "custom": true
    }
  ],
  "status": "pending",
  "created_at": "2026-07-19T00:00:00Z",
  "source": {"kind":"runtime"}
}
```

Question status 值：`pending`、`answered`、`rejected`。

### Connect View

`GET /connect` 返回：

```json
{
  "connectors": [
    {
      "name": "telegram",
      "registered": true,
      "initialized": true,
      "configured": true,
      "running": true,
      "status": "running",
      "message": "",
      "devices": [{"id":"1","username":"alice","display":"Alice"}]
    }
  ]
}
```

### Other Queries

- `GET /model`:

```json
{"provider":"openai","model":"gpt-test"}
```

- `GET /commands`:

```json
{"commands":["help","model","connect"]}
```

- `GET /sessions`:

```json
{"sessions":[{"id":"session_1","cwd":"/repo","createdAt":0,"updatedAt":0,"msgCount":2}]}
```

- `GET /sessions/current/messages`:

```json
{"messages":[{"role":"user","content":"hello"}]}
```

- `GET /stats`:

```json
{
  "promptTokens": 10,
  "completionTokens": 5,
  "turnPrompt": 2,
  "turnCompletion": 1,
  "contextTokens": 100,
  "compactCount": 0,
  "duration": "1.2s"
}
```

- `GET /context` returns `runtime.ContextSnapshot` as JSON.
- `GET /memory` returns `runtime.MemoryView` as JSON. Current stable scope is `project`.

## SSE Events

`GET /events` streams Server-Sent Events:

```text
id: evt_1
event: run_done
data: {"id":"evt_1","run_id":"run_1","type":"run_done","time":"2026-07-19T00:00:00Z","payload":{"output":"done"}}
```

If `id` is empty, the `id:` line is omitted. `event:` is always the runtime event type. `data:` is the full runtime event JSON.

Common event types:

- `input_accepted`
- `system_message`
- `assistant_delta`
- `reasoning_delta`
- `phase_changed`
- `tool_started`
- `tool_preview`
- `tool_completed`
- `tool_blocked`
- `approval_requested`
- `approval_resolved`
- `question_requested`
- `question_resolved`
- `run_started`
- `run_done`
- `run_failed`
- `run_aborted`
- `session_resumed`
- `connector_status`
