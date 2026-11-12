---
icon: lucide/cable
---

# ACP

Pegg implements the **Agent Client Protocol** (ACP) v1: a JSON-RPC 2.0 protocol for
driving a coding agent from an editor or other client. The server exposes the same
orchestrator used by the CLI and TUI, and follows the stable ACP v1 specification
exactly. Pegg-specific features are exposed as spec-compliant extensions
(`_`-prefixed methods and `_meta` fields).

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
- `initialize` **must** be the first request. Any other request before it returns
  `-32600`.
- All file paths are absolute; line numbers are 1-based.
- Custom data rides in `_meta`; Pegg uses `com.pegg/` keys.

### Error codes

| Code | Meaning |
|---|---|
| `-32700` | Parse error |
| `-32600` | Invalid request (including a request before `initialize`) |
| `-32601` | Method not found |
| `-32602` | Invalid params |
| `-32603` | Internal error |
| `-32800` | Request cancelled |
| `-32000` | Authentication required |
| `-32002` | Resource not found (unknown session/file/terminal) |

## Methods

| Method | Params | Result |
|---|---|---|
| `initialize` | `protocolVersion`, `clientCapabilities?`, `clientInfo?` | Capabilities and agent info |
| `authenticate` | `methodId` | `{}` |
| `logout` | `{}` | `{}` |
| `session/new` | `cwd` (required), `mcpServers`, `additionalDirectories?`, `systemPrompt?` | `{ "sessionId", "modes", "configOptions", "models", "_meta" }` |
| `session/load` | `sessionId`, `cwd`, `mcpServers`, `additionalDirectories?`, `systemPrompt?` | `{ "sessionId", "modes", "configOptions", "models", "_meta" }` (replays history as notifications) |
| `session/resume` | same as `session/load` | `{ "sessionId", "modes", "configOptions", "models" }` (no replay) |
| `session/prompt` | `sessionId`, `prompt` | `{ "stopReason" }` |
| `session/set_mode` | `sessionId`, `modeId` (`build`/`plan`/`auto`) | `{}` |
| `session/set_model` | `sessionId`, `modelId` (or `provider` + `model`) | `{ "provider", "model" }` |
| `session/set_config_option` | `sessionId`, `configId`, `type?`, `value` | `{ "configOptions" }` |
| `session/delete` | `sessionId` | `{}` |
| `session/close` | `sessionId` | `{}` |
| `session/list` | `cwd?`, `cursor?` | `{ "sessions": [...], "nextCursor?": "..." }` |
| `session/cancel` | `sessionId` | no response (cancels a local or remote run) |
| `$/cancel_request` | `requestId` | no response (cancels the identified request) |

Unknown methods return `-32601`. Unknown sessions return `-32002` with
`data.sessionId`.

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
      "delete": {}, "resume": {}, "close": {}, "list": {},
      "additionalDirectories": {},
      "systemPrompt": { "mode": { "override": {} } }
    },
    "auth": { "logout": {} },
    "_meta": {
      "com.pegg/capabilities": {
        "ask": true, "permissions": true, "models": true, "commands": true,
        "attachments": true, "diffs": true, "plan": true, "history": true,
        "terminals": true, "elicitation": true
      }
    }
  },
  "agentInfo": { "name": "pegg", "title": "Pegg AI Agent", "version": "0.1.9" },
  "authMethods": []
}
```

### `session/new`

Creates a new ACP session. `cwd` is required. Pegg also creates a persisted session
(titled `ACP Session`) in its session store and links the two; the persisted id is
returned in `_meta["com.pegg/sessionId"]`.

```json
{ "jsonrpc": "2.0", "id": 1, "method": "session/new", "params": { "cwd": "/home/user/project", "mcpServers": [] } }
```

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "sessionId": "9d1f...",
    "modes": { "currentModeId": "auto", "availableModes": [ { "id": "build", "name": "Build" } ] },
    "configOptions": [
      { "id": "mode", "name": "Session Mode", "category": "mode", "type": "select", "currentValue": "auto", "options": [] },
      { "id": "model", "name": "Model", "category": "model", "type": "select", "currentValue": "gpt-4o", "options": [] }
    ],
    "models": {
      "currentModelId": "gpt-4o",
      "availableModels": [ { "modelId": "gpt-4o", "name": "gpt-4o" } ]
    },
    "_meta": { "com.pegg/sessionId": "3f2b...", "com.pegg/permissionMode": "semi-ask" }
  }
}
```

The `models` field is the legacy model selector state read directly by clients
such as T3Code; `configOptions` carries the same information as a `model`
category select. Both are populated even before the LLM manager finishes its
initial provider sync (falling back to the on-disk model cache).

!!! warning

    The `sessionId` returned by `session/new` is the **ACP session id**. `session/load`
    and `session/resume` expect the **Pegg** session id (the value in
    `_meta["com.pegg/sessionId"]`).

`mcpServers`, `additionalDirectories`, and `systemPrompt` are accepted. `systemPrompt`
supports `append` and `override` modes.

### `session/prompt`

The first `text` content block is used as the agent input. If the prompt contains no
text but includes attachments, the agent receives the attachments alone. If `audio`
blocks arrive without data, Pegg transcribes them with the configured STT provider and
uses the transcript. Every attachment — `image`, `audio`, `resource` (text or blob),
and `resource_link` targets that exist on disk — is copied into the attachment store
(`os.TempDir()/pegg`) exactly like a file dropped in the TUI, and referenced by path in
an `<attachments>` block appended to the message. Only media the active model accepts
is also sent inline. While the agent runs, the server pushes `session/update`
notifications (see below). The result's `stopReason` is one of:

| Stop reason | Meaning |
|---|---|
| `end_turn` | The agent finished normally |
| `max_tokens` | The token limit was reached |
| `max_turn_requests` | The per-turn request limit was reached |
| `refusal` | The agent refused to continue |
| `cancelled` | The prompt was cancelled via `session/cancel` |

```json
{ "jsonrpc": "2.0", "id": 2, "method": "session/prompt",
  "params": { "sessionId": "9d1f...", "prompt": [{ "type": "text", "text": "Fix the failing test" }] } }
```

```json
{ "jsonrpc": "2.0", "id": 2, "result": { "stopReason": "end_turn" } }
```

### `session/set_config_option`

Sets a session configuration option. Pegg exposes `mode`, `model`, and
`reasoning_effort`. The result always contains the complete configuration state.

```json
{ "jsonrpc": "2.0", "id": 3, "method": "session/set_config_option",
  "params": { "sessionId": "9d1f...", "configId": "model", "value": "gpt-4o" } }
```

### `session/list`

Lists persisted sessions with opaque cursor pagination. `SessionInfo` uses
`sessionId`, `cwd`, `title`, `updatedAt`, and `_meta` (Pegg extras such as
`com.pegg/provider`, `com.pegg/model`, `com.pegg/mode`, `com.pegg/usage`,
`com.pegg/createdAt`).

Sessions with no messages are excluded from the listing. A session created by
`session/new` stays hidden until the first prompt records a message, so clients
that open a session eagerly (for example while probing the agent) do not leave
empty `ACP Session` entries in the list.

```json
{
  "sessions": [
    { "sessionId": "3f2b...", "cwd": "/home/user/project", "title": "Fix parser bug",
      "updatedAt": "2026-09-10T09:42:11Z" }
  ],
  "nextCursor": ""
}
```

## Server notifications

### `session/update`

The server pushes spec-compliant `session/update` notifications:

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
| `user_message_chunk` | Replaying a stored user message or mirroring a run started elsewhere |
| `agent_message_chunk` | Streaming assistant text |
| `agent_thought_chunk` | Streaming model reasoning |
| `tool_call` | A tool call starts (`toolCallId`, `title`, `name`, `kind`, `status`, `rawInput`, `locations`) |
| `tool_call_update` | A tool call updates: `in_progress` with partial `rawOutput` while bash runs, then `completed`/`failed` with the final output (`status`, `rawOutput`, optional `content` diff) |
| `plan` | A `todowrite` call updates the plan (`entries`) |
| `available_commands_update` | Skills/commands available for the session (`availableCommands`) |
| `current_mode_update` | Mode changed (`currentModeId`) |
| `config_option_update` | Model/mode/config changed (`configOptions`) |
| `session_info_update` | Session title/metadata changed (`title`, `updatedAt`) |
| `usage_update` | After a run with token usage (`used`, `size`, `cost`) |
| `notice` | A user-facing notice (`severity`, `title`, `description`) |
| `compaction_update` | Context compaction (`compactionId`, `status`, `summary`) |
| `compaction_summary_chunk` | Streamed compaction summary (`compactionId`, `content`) |

Pegg extras on standard updates live in `_meta` with `com.pegg/` keys:
`com.pegg/agentId`, `com.pegg/agentName`, `com.pegg/seq`, `com.pegg/baseSeq`,
`com.pegg/historyPage`, `com.pegg/attachments` (an array of
`{ name, mediaType, type, size, path }` descriptors), and `com.pegg/error`.
Task/subagent updates also carry `com.pegg/taskName` (the unique `task` name) and
`com.pegg/durationMs`; finished subagents add `com.pegg/usage` (`{ used, size, cost }`).
The `session/prompt` result includes `_meta["com.pegg/runId"]` so clients can ignore
a stale prompt completion when a background continuation run is active.

### `_pegg/update`

Pegg-only session events are delivered on the `_pegg/update` extension notification
(same `{ sessionId, update }` shape) so the standard `session/update` surface stays
spec-pure. Variants: `subagent`, `run_state_update`, `run_ask_resolved`,
`agent_warning_clear`.

### `_pegg/sessions_changed`

Emitted when the persisted session list changes. No params.

## Client methods

The server calls the client when the agent needs user input or client-side execution:

- `session/request_permission` — the permission middleware asks for a decision.
  Clients return `{ "outcome": { "outcome": "selected", "optionId": "allow_once" } }`
  (or another advertised option id), or `{ "outcome": { "outcome": "cancelled" } }`.
- `fs/read_text_file` / `fs/write_text_file` — available when the client advertises
  the `fs` capability. Current built-in tools use the server-side sandbox.
- `terminal/create`, `terminal/output`, `terminal/wait_for_exit`, `terminal/kill`,
  `terminal/release` — available when the client advertises `terminal: true`.
- `elicitation/create` — structured user input (form or URL mode). Pegg uses it for
  `askuserquestion` when the client advertises the `elicitation` capability, and
  falls back to the `_pegg/ask` extension otherwise.
- `elicitation/complete` — notification that a URL elicitation finished.

Permission and question requests are sent while `session/prompt` is running. A client
must continue reading JSON-RPC frames while a prompt is active so it can answer them.

## Pegg extension methods

Pegg-specific methods are prefixed with `_` per the ACP extensibility rules:

| Method | Purpose |
|---|---|
| `_pegg/models` | List available models |
| `_pegg/skills/reload` | Reload skills and re-advertise commands |
| `_pegg/skills/search` | Search the skill hub |
| `_pegg/skills/install` | Install a skill from the hub |
| `_pegg/stt/transcribe` | Transcribe base64 audio |
| `_pegg/stt/record/start` / `_pegg/stt/record/stop` | Record and transcribe voice |
| `_pegg/session/history` | Paginated session history |
| `_pegg/session/rename` | Rename a session |
| `_pegg/session/set_model` | Set provider/model/reasoning effort (alias of `session/set_model`) |
| `_pegg/session/fork` | Fork a session before a message |
| `_pegg/session/revert` | Revert a session to a message |
| `_pegg/ask` | Ask the user structured questions (fallback for elicitation) |

### Session history pagination

`_pegg/session/history` loads one page of older messages for a session that is
already loaded. Params: `{ sessionId, cwd?, beforeSeq?, limit? }`. `beforeSeq` is
an exclusive upper bound (`seq < beforeSeq`); `limit` defaults to no cap at the
protocol level, and the bundled clients request 100–200 per page. The messages are not returned in
the result — they are streamed as `session/update` notifications, each carrying
`_meta["com.pegg/seq"]` and `_meta["com.pegg/historyPage"] = true`. The JSON-RPC
result carries `{ oldestSeq, hasMore }`, where `hasMore` reports whether more
messages exist before `oldestSeq`.

Clients should treat a history-page update as a prepend, never as a live append,
and can keep the reading position stable by anchoring the first visible message
while the page is inserted. `session/load` uses the same pagination through
`historyLimit`/`historyBefore` (a non-positive `historyLimit` loads the whole
session for backwards compatibility).

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
→ {"jsonrpc":"2.0","id":1,"method":"session/new","params":{"cwd":"/home/user/project","mcpServers":[]}}
← {"jsonrpc":"2.0","id":1,"result":{"sessionId":"9d1f...","modes":{...},"configOptions":[...]}}
→ {"jsonrpc":"2.0","id":2,"method":"session/prompt","params":{"sessionId":"9d1f...","prompt":[{"type":"text","text":"List the Go files"}]}}
← {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"9d1f...","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Here are"}}}}
← {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"9d1f...","update":{"sessionUpdate":"tool_call","toolCallId":"…","title":"glob","name":"glob","kind":"search","status":"pending"}}}
← {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"9d1f...","update":{"sessionUpdate":"tool_call_update","toolCallId":"…","status":"completed"}}}
← {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"9d1f...","update":{"sessionUpdate":"usage_update","used":1540,"size":200000,"cost":{"amount":0.004,"currency":"USD"}}}}
← {"jsonrpc":"2.0","id":2,"result":{"stopReason":"end_turn"}}
```

## Cancellation

Send `session/cancel` with the session id to abort a running prompt. The running
agent's context is cancelled and the in-flight `session/prompt` resolves with
`stopReason: "cancelled"`. No response is written for the cancel message itself.

`$/cancel_request` with a `requestId` cancels the identified in-flight request (for
example a `session/prompt`), following the protocol-level cancellation mechanism.
