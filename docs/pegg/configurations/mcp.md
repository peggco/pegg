---
icon: lucide/package-plus
---

# MCP

Pegg is an MCP (Model Context Protocol) **client**. It connects to MCP servers at
startup, discovers their tools, and exposes them to agents alongside the built-in
tools.

## Configuration

=== "Global"

    Add servers to the `mcp_servers` map in `~/.peggco/pegg.json`:

    ```json
    {
      "mcp_servers": {
        "db": {
          "command": "npx",
          "args": ["-y", "@modelcontextprotocol/server-postgres"],
          "env": { "DEBUG": "true" }
        },
        "remote": {
          "type": "streamable-http",
          "url": "https://example.com/mcp",
          "headers": { "Authorization": "Bearer token" }
        },
        "legacy": {
          "type": "sse",
          "url": "https://example.com/sse"
        }
      }
    }
    ```

=== "Project"

    Create `.mcp.json` in the project root:

    ```json
    {
      "mcpServers": {
        "db": {
          "command": "npx",
          "args": ["-y", "@modelcontextprotocol/server-postgres",
                   "postgresql://localhost:5432/db"]
        }
      }
    }
    ```

Project entries override global entries with the same name.

### Server options

| Key | Type | Description |
|---|---|---|
| `type` | string | Transport: `stdio`, `sse`, or `streamable-http`. When omitted, `url` selects legacy SSE and `command` selects stdio |
| `command` | string | Executable to spawn (stdio transport) |
| `args` | string[] | Command arguments |
| `env` | object | Extra environment variables, merged over the inherited environment |
| `url` | string | Remote endpoint (SSE or Streamable HTTP) |
| `headers` | object | HTTP headers sent with SSE and Streamable HTTP requests |

A stdio server must define `command`; a remote server must define `url`. The
`type` aliases `local` (stdio), `http` (legacy SSE), and `streamable`
(streamable-http) are accepted for compatibility, but new configurations should
use the canonical names.

## Transports

### stdio

Pegg spawns `command args...` as a child process:

- The child inherits Pegg's full environment; `env` entries override individual
  variables.
- The child's stderr is forwarded to Pegg's stderr.
- Messages are newline-delimited JSON (with optional `Content-Length` framing), max
  16 MiB per frame.
- The child is terminated with SIGTERM when Pegg shuts down.

### SSE (legacy)

Pegg opens a `GET` request with `Accept: text/event-stream` and the configured
headers. The server must first emit an `endpoint` event telling Pegg where to
POST messages. Subsequent requests include the `Mcp-Session-Id` header when the
server provides one. Responses arrive as `message` events.

Set `"type": "sse"` explicitly, or omit `type` on a server that only has a `url`.
The `http` CLI/type alias also selects this legacy transport.

### Streamable HTTP

Set `"type": "streamable-http"` for MCP servers implementing the
[Streamable HTTP](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)
transport. Pegg uses protocol version `2025-06-18` for this transport and:

- `POST`s each JSON-RPC message to `url` with
  `Accept: application/json, text/event-stream`. A response may be a single JSON
  object, a batch array, an SSE stream, or empty (`202`/`204`) for notifications.
- Captures `Mcp-Session-Id` from responses and sends it on every subsequent
  request and on stream reconnects.
- Sends `MCP-Protocol-Version` after initialization.
- Opens an optional `GET` SSE stream for server-initiated notifications, resuming
  with `Last-Event-ID` after a reconnect. Servers that reject the stream with
  `405` are supported; POST remains the request path.
- Re-initializes once and retries when the server reports an expired session
  (`404`).
- Sends a best-effort `DELETE` on shutdown to end the session.

## Lifecycle

1. At startup, one goroutine per configured server connects with a 15 second
   timeout: transport connect → `initialize` → `tools/list`.
2. Every discovered tool is registered in the global tool registry as
   `<server>__<tool>` (double underscore). Names are guaranteed not to collide
   between servers.
3. A server that fails to connect is logged as a warning and skipped; the rest of
   Pegg continues normally.
4. On shutdown, every MCP tool is unregistered and every connection is closed.

Streamable HTTP servers use MCP protocol version `2025-06-18`; stdio and legacy
SSE servers use `2024-11-05`. Pegg accepts the version returned by the server
during `initialize`.

## Managing servers

```bash
pegg mcp list                       # all servers, global and project
pegg mcp list --project             # project only
pegg mcp tools                      # live tools from connected servers
pegg mcp tools --server db          # filter by server
```

### Adding a server

Interactive:

```bash
pegg mcp add
```

Fully specified:

```bash
# Local stdio server
pegg mcp add --project --name db --transport stdio \
  --command npx --arg -y --arg @modelcontextprotocol/server-postgres \
  --env DEBUG=true

# Remote Streamable HTTP server
pegg mcp add --global --name remote --transport streamable-http \
  --url https://example.com/mcp \
  --header Authorization="Bearer token"

# Legacy remote SSE server
pegg mcp add --global --name legacy --transport sse \
  --url https://example.com/sse \
  --header Authorization="Bearer token"
```

| Flag | Description |
|---|---|
| `--global` / `--project` | Where to write the server |
| `--name` | Server name |
| `--transport` | `stdio`, `sse` (legacy), or `streamable-http` (aliases: `local`, `http`) |
| `--command` | Executable for stdio servers |
| `--arg` | Argument (repeatable) |
| `--env` | `KEY=VALUE` environment variable (repeatable) |
| `--url` | Endpoint for SSE or Streamable HTTP servers |
| `--header` | `KEY=VALUE` request header (repeatable) |

The interactive flow offers common header presets: `Authorization`, `X-API-Key`,
`Cookie`, and `User-Agent`.

### Removing a server

```bash
pegg mcp remove db
```

If the server exists in both scopes, Pegg asks which one to remove. If it exists
in only one, that scope is used automatically.

## Using MCP tools

MCP tools behave like built-in tools. The model sees them under their
`server__tool` names, and they are subject to the permission middleware's
`permission.default` mode unless overridden by a rule.

Tool results are returned as text; image content is summarized as `[image data]`.
A tool result flagged `isError` is surfaced as a tool error.

## TUI

Open Settings with ++ctrl+p++ and switch to the **MCP** tab. Each server shows its
tool count and connection state (`connected` / `not connected`). Selecting a server
opens a sub-list of its tools.

## Troubleshooting

```bash
pegg logs --search mcp
pegg mcp tools --server db
```

Common issues:

- **Command not found** — the executable must be on `PATH` for the Pegg process.
- **SSE server never becomes ready** — the server must emit the `endpoint` event.
- **Tools missing** — the server connected but `tools/list` returned nothing, or the
  connection failed during `initialize`. Check the logs for a warning line.
