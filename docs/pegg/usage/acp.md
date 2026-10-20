---
icon: lucide/cable
---

# ACP

Pegg implements the **Agent Client Protocol** (ACP): a JSON-RPC 2.0 protocol for
driving a coding agent from an editor or other client. The server exposes the same
orchestrator used by the CLI and TUI.

The official [VS Code extension](vscode.md) is a complete ACP client and uses the
stdio transport. See its guide for editor workflows, permissions, voice, sessions,
and settings.

## Starting the server

=== "stdio"

    ```bash
    pegg serve --acp --stdio
    ```

    Newline-delimited JSON-RPC on stdin/stdout. This is the transport used by editor
    integrations that spawn the agent as a subprocess.

=== "HTTP"

    ```bash
    pegg serve --acp
    # acp server listening on http://127.0.0.1:8080/acp
    ```

    JSON-RPC over HTTP with Server-Sent Events for responses and notifications, plus
    optional WebSocket upgrade. The handler accepts any URL path; `/acp` is the
    convention.

Host and port come from `--host` / `--port` or the `server` config block. The ACP
handler does not apply the HTTP API's auth or CORS middleware.

!!! note

    The background [scheduler](../features/scheduling.md) runs while the ACP server is
    up, so due schedules fire independently of the connected editor session.

## Protocol basics

- Every message is a JSON-RPC 2.0 envelope. `"jsonrpc"` must be exactly `"2.0"`.
- Batch requests are not supported.
- A message with `method` and no `id` is a notification: handlers run, no response is
  written.
- A message with `id` and no `method` is a response to a server-initiated request.

### Error codes

| Code | Meaning |
|---|---|
| `-32700` | Parse error |
| `-32600` | Invalid request |
| `-32601` | Method not found |
| `-32602` | Invalid params |
| `-32603` | Internal error |
| `-32800` | Cancelled |

## Methods

| Method | Params | Result |
|---|---|---|
| `initialize` | `protocolVersion`, `clientCapabilities?`, `clientInfo?` | Capabilities and agent info |
| `session/new` | `cwd` (required), `mcpServers?`, `additionalDirectories?` | `{ "sessionId": "...", "mode": "auto" }` |
| `session/load` | `sessionId`, `cwd`, `mcpServers?`, `additionalDirectories?` | `null` (replays history as notifications) |
| `session/resume` | same as `session/load` | `{}` (no replay) |
| `session/set_mode` | `sessionId`, `modeId` (`build`/`plan`/`auto`) | `{}` |
| `session/set_model` | `sessionId`, `provider`, `model`, `reasoningEffort?` | `{ "provider": "...", "model": "..." }` |
| `session/rename` | `sessionId`, `title` | `{}` |
| `session/prompt` | `sessionId`, `prompt` | `{ "stopReason": "...", "usage?": {...} }` |
| `session/delete` | `sessionId` | `null` |
| `session/close` | `sessionId` | `{}` |
| `session/list` | `cwd?`, `all?`, `limit?`, `cursor?` | `{ "sessions": [...], "nextCursor": "" }` |
| `pegg/models` | `provider?` | `{ "models": [{ "provider": "...", "model": {...} }] }` |
| `stt/transcribe` | `audio` (base64), `mediaType?`, `fileName?` | `{ "text": "...", "provider": "...", "model": "..." }` |
| `stt/record/start` | — | `{ "recording": true }` |
| `stt/record/stop` | — | `{ "text": "...", "provider": "...", "model": "..." }` |
| `session/cancel` | `sessionId` | no response |
| `$/cancelRequest` | — | no response (no-op) |

Unknown methods return `-32601`.

Sessions run in a **mode** (`build`, `plan`, `auto`; default `auto`). The mode is
persisted with the session and restored by `session/load`/`session/resume`. Change
it with `session/set_mode`; the server confirms with a `session/update`
notification whose `update` is `{ "sessionUpdate": "current_mode_update", "mode": "..." }`.

### `initialize`

`protocolVersion` is required and must be at least `1`. The result advertises what
the agent supports:

```json
{
  "protocolVersion": 1,
  "agentCapabilities": {
    "loadSession": true,
    "promptCapabilities": { "image": true, "audio": true, "embeddedContext": true },
    "mcpCapabilities": { "http": true, "sse": false },
    "sessionCapabilities": {
      "delete": {},
      "resume": {},
      "close": {},
      "additionalDirectories": {}
    },
    "pegg": {
      "ask": true,
      "permissions": true,
      "models": true,
      "commands": true,
      "attachments": true,
      "diffs": true,
      "plan": true
    }
  },
  "agentInfo": { "name": "pegg", "title": "Pegg AI Agent", "version": "0.1.9" },
  "authMethods": []
}
```

### `session/new`

Creates a new ACP session. `cwd` is required. Pegg also creates a persisted
session (titled `ACP Session`) in its session store and links the two.

```json
{ "jsonrpc": "2.0", "id": 1, "method": "session/new", "params": { "cwd": "/home/user/project" } }
```

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "sessionId": "9d1f...",
    "peggSessionId": "3f2b...",
    "mode": "auto",
    "provider": "openai",
    "model": "gpt-4o",
    "permissionMode": "semi-ask",
    "commands": [{ "name": "init", "description": "..." }]
  }
}
```

!!! warning

    The `sessionId` returned by `session/new` is the **ACP session id**, not the
    persisted Pegg session id. `session/load` and `session/resume`, however,
    expect the **Pegg** session id.

`mcpServers` and `additionalDirectories` are accepted but currently ignored.

### `session/prompt`

The first `text` content block is used as the agent input. If the prompt contains
no text but includes an `audio` content block, Pegg transcribes that audio with
the configured STT provider and uses the transcript. `image` blocks become image
attachments; `resource` text blocks are inlined as file context; resource blobs
become file attachments. While the agent runs, the server pushes
`session/update` notifications (see below). The result's `stopReason` is one of:

| Stop reason | Meaning |
|---|---|
| `end_turn` | The agent finished normally |
| `cancelled` | The prompt was cancelled via `session/cancel` |
| `error` | The run failed |

```json
{ "jsonrpc": "2.0", "id": 2, "method": "session/prompt",
  "params": { "sessionId": "9d1f...", "prompt": [{ "type": "text", "text": "Fix the failing test" }] } }
```

```json
{ "jsonrpc": "2.0", "id": 2, "result": { "stopReason": "end_turn" } }
```

### `stt/transcribe`

Transcribes base64 audio directly, without starting an agent run. Audio must be at
most 8 MiB decoded.

```json
{ "jsonrpc": "2.0", "id": 3, "method": "stt/transcribe",
  "params": { "audio": "<base64>", "mediaType": "audio/webm", "fileName": "voice.webm" } }
```

```json
{ "jsonrpc": "2.0", "id": 3,
  "result": { "text": "Fix the failing parser test",
              "provider": "assemblyai", "model": "universal-3-6-pro" } }
```

The STT provider and model are configured on the agent machine under
**Settings → General**. A missing configuration returns `-32603`; invalid audio
returns `-32602`.

### `session/list`

Lists persisted sessions (page 1, size 50, newest first). `nextCursor` is currently
always empty.

```json
{
  "sessions": [
    { "id": "3f2b...", "title": "Fix parser bug",
      "createdAt": "2026-09-10T09:15:00Z", "updatedAt": "2026-09-10T09:42:11Z" }
  ],
  "nextCursor": ""
}
```

## Server notifications

The server pushes `session/update` notifications:

```json
{
  "jsonrpc": "2.0",
  "method": "session/update",
  "params": {
    "sessionId": "9d1f...",
    "update": { "sessionUpdate": "agent_message_chunk", "messageId": "…",
                "content": { "type": "text", "text": "Working on it" } }
  }
}
```

| `sessionUpdate` | Emitted when |
|---|---|
| `user_message_chunk` | Replaying a stored user message (`session/load`) |
| `agent_message_chunk` | Streaming assistant text |
| `agent_thought_chunk` | Streaming model reasoning |
| `tool_call` | A tool call starts (`toolCallId`, `title`, `kind`, `status`, `rawInput`, `locations`) |
| `tool_call_update` | A tool call completes or fails (`status`, `rawOutput`, `error`, optional `contents` diff) |
| `plan` | A `todowrite` call updates the plan (`entries`) |
| `usage_update` | After a run with token usage (`used`, `size`, `cost`) |
| `current_mode_update` | Mode changed (`mode`) |
| `current_model_update` | Model changed (`provider`, `model`) |
| `available_commands_update` | Skills/commands available for the session |
| `error` | The run failed (`error`) |

Updates also include `agentId` and `agentName` when the event came from a
subagent. Tool calls may include a `contents` array with a `diff` object
(`path`, `oldText`, `newText`) for `write`/`edit` calls.

When the context is compacted mid-run, an `agent_message_chunk` notification is
emitted with the text `[Context compacted (<strategy>) — <messages> messages,
<tokens> tokens]`.

The `usage_update` payload uses a fixed `size` of `200000` and a USD cost object.

## Transports

=== "stdio"

    Write one JSON-RPC message per line to stdin; read newline-delimited JSON from
    stdout. Empty lines are ignored. Closing stdin ends the server cleanly.

    ```bash
    echo '{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":1}}' \
      | pegg serve --acp --stdio
    ```

=== "HTTP + SSE"

    The flow is:

    1. `POST` an `initialize` request **without** a connection header. The response
       is `200` with an `Acp-Connection-Id` header and the initialize result.
    2. `GET` the same URL with `Accept: text/event-stream` and the
       `Acp-Connection-Id` header to open a long-lived event stream.
    3. `POST` further JSON-RPC requests with `Acp-Connection-Id`. The server returns
       `202 Accepted` immediately; the JSON-RPC response arrives on the SSE stream.
    4. `DELETE` with `Acp-Connection-Id` to close the connection.

    | Header | Purpose |
    |---|---|
    | `Acp-Connection-Id` | Connection identifier (returned by `initialize`) |
    | `Acp-Session-Id` | Routes responses/notifications to a per-session SSE stream |

    Status codes: `415` for a missing JSON content type, `400` for an invalid or
    missing connection, `404` for an unknown connection, `406` when the stream
    request lacks `Accept: text/event-stream`, `405` for unsupported methods.

    ```bash
    # 1. initialize
    curl -i http://127.0.0.1:8080/acp \
      -H 'Content-Type: application/json' \
      -d '{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":1,"clientInfo":{"name":"demo","version":"1.0"}}}'

    # 2. open the event stream (long-lived)
    curl -N http://127.0.0.1:8080/acp \
      -H 'Accept: text/event-stream' \
      -H 'Acp-Connection-Id: <uuid>'
    ```

=== "WebSocket"

    A `GET` with `Upgrade: websocket` establishes a WebSocket connection; JSON-RPC
    responses are sent as text frames on the same socket.

    !!! note

        In pure WebSocket mode, `session/update` notifications are only delivered if
        a matching SSE stream is also open. Use HTTP+SSE for full notification
        support.

## Complete example (stdio)

```
→ {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":1}}
← {"jsonrpc":"2.0","id":0,"result":{"protocolVersion":1,"agentCapabilities":{...},"agentInfo":{...}}}
→ {"jsonrpc":"2.0","id":1,"method":"session/new","params":{"cwd":"/home/user/project"}}
← {"jsonrpc":"2.0","id":1,"result":{"sessionId":"9d1f..."}}
→ {"jsonrpc":"2.0","id":2,"method":"session/prompt","params":{"sessionId":"9d1f...","prompt":[{"type":"text","text":"List the Go files"}]}}
← {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"9d1f...","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Here are"}}}}
← {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"9d1f...","update":{"sessionUpdate":"tool_call","toolCallId":"…","title":"glob","kind":"other","status":"pending"}}}
← {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"9d1f...","update":{"sessionUpdate":"tool_call_update","toolCallId":"…","status":"completed"}}}
← {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"9d1f...","update":{"sessionUpdate":"usage_update","used":1540,"size":200000,"cost":{"amount":0.004,"currency":"USD"}}}}
← {"jsonrpc":"2.0","id":2,"result":{"stopReason":"end_turn"}}
```

## Cancellation

Send `session/cancel` with the session id to abort a running prompt. The running
agent's context is cancelled and the in-flight `session/prompt` resolves with
`stopReason: "cancelled"`. No response is written for the cancel message itself.
`$/cancelRequest` is accepted but does nothing.

## Outbound requests

The server calls the client when the agent needs user input:

- `session/request_permission` — the permission middleware asks for a decision.
  Clients should return `{ "outcome": { "outcome": "selected", "optionId": "allow_once" } }`
  (or another advertised option id).
- `pegg/ask` — Pegg-specific questions from `askuserquestion`. Params contain
  `sessionId`, `agentId`, `agentName`, and `questions` (text, boolean, select,
  multiselect). Return `{ "answers": { "<question id>": "<answer>" } }`.

Permission and question requests are sent while `session/prompt` is running. A
client must continue reading JSON-RPC frames while a prompt is active so it can
answer them. If the client cannot answer, return a rejection/empty answers rather
than leaving the run blocked.

`fs/read_text_file` and `fs/write_text_file` are also available; current built-in
tools use the server-side sandbox and do not require them.
