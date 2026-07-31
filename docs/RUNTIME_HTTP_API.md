# Runtime HTTP API

`runtime/httpapi` exposes the runtime interaction protocol over JSON and
Server-Sent Events. It does not expose bot implementations directly.

## Rules

- JSON requests reject unknown fields and multiple JSON values.
- Runtime protocol errors include a stable `code`.
- SSE event IDs are monotonic cursors. Clients reconnect with `after` or
  `Last-Event-ID`.
- `replay=1` replays retained history before live events.
- `WithBearerAuth` protects every endpoint except `/health`.
- Optional management endpoints return `501 Not Implemented` when their
  capability is absent. `/capabilities` is the authoritative discovery
  response.

## Endpoints

| Method | Path | Description |
| --- | --- | --- |
| GET | `/health` | Process health and protocol version |
| GET | `/status` | Runtime lifecycle and active run |
| GET | `/capabilities` | Optional capability manifest |
| POST | `/input` | Start a run |
| GET | `/events?run_id=&after=&replay=1` | Subscribe to runtime events |
| GET | `/runs?limit=50` | List recent run snapshots |
| GET | `/runs/current` | Read the latest run snapshot |
| GET | `/runs/{runID}` | Read one run snapshot |
| POST | `/runs/{runID}/abort` | Cancel the active run |
| POST | `/approvals/{approvalID}` | Resolve a pending approval |
| POST | `/questions/{questionID}` | Answer or reject a pending question |
| GET | `/connect` | Read connector state |
| POST | `/connect/{name}` | Configure or start a connector |
| POST | `/disconnect/{name}` | Stop a connector |
| GET | `/model` | Read the active provider and model |
| GET | `/commands` | List available commands |
| GET | `/metrics` | Read the latest operational metrics |
| GET | `/context` | Read the context snapshot |
| GET | `/memory?scope=project` | Read project memory |
| GET | `/sessions` | List sessions |
| GET | `/sessions/current/messages` | Read current session messages |

## Input

`POST /input` accepts:

```json
{
  "text": "hello",
  "source": {"kind": "http", "id": "client-1"},
  "sender": {"id": "user-1"}
}
```

The response is `202 Accepted` with `{"run_id":"run_1"}`. A blank
`source.kind` defaults to `http`.

## Interaction

Approval:

```json
{"allowed":true,"remember":false,"allow_with_permission":false}
```

Question:

```json
{"answers":[["choice-a"]],"rejected":false}
```

Connector commands accept an optional body:

```json
{"args":["status"]}
```

## Events

Each SSE item contains the stable event type and the complete versioned event:

```text
id: evt_12
event: run_done
data: {"version":"2.0","id":"evt_12","sequence":12,"run_id":"run_1","type":"run_done","payload":{"output":"done"}}
```

The main lifecycle events are `run_started`, `run_done`, `run_failed`, and
`run_aborted`. Streaming, tool, approval, question, session, connector, and
metrics changes use their corresponding typed runtime events.
