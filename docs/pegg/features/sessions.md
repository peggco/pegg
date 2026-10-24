---
icon: lucide/history
---

# Sessions

A session is a persistent conversation. Every turn — user input, assistant messages,
tool calls and results, usage — is recorded and can be resumed later, in any
interface.

## Storage

| Driver | Location | Notes |
|---|---|---|
| `sqlite` (default) | `~/.pegg/sessions.db` | Recommended; one table with full message history and snapshots |
| `json` | `~/.pegg/sessions/` | One JSON file per session |

Configure the driver with `session.driver` in [Config](../config.md#session). Sessions
record the `project_dir` at creation time, which is why listings default to the
current project.

## How sessions are created

- A session is created automatically the first time a run starts and none is active.
- The **recorder** subscribes to the agent event stream and persists messages as they
  happen — streamed tokens are committed in small batches, tool results and usage
  immediately.
- A **title generator** runs in the background on the first message: a dedicated
  model call (temperature 0.3) produces a title of at most 50 characters, in the
  same language as the message, with no tool names. Until then the title is
  `New Session <timestamp>`.
- Sessions track cumulative token usage and cost across all their runs.

## Resuming a session

=== "CLI"

    ```bash
    pegg run --session 3f2b1c8e-... "Continue the refactor"
    pegg run --select-session              # pick from a paginated list
    ```

    The stored history is replayed into the model context and shown as
    `--- session history ---`. Without an explicit provider/model, the session's
    stored provider and model are reused.

=== "TUI"

    Open Settings (++ctrl+p++) → **Session** → **Load session**. Sessions from the
    current directory are listed newest first. Loading restores the last 50
    messages; scrolling to the top loads older messages in batches of 50. A
    submitted message with an active session resumes it automatically.

=== "VS Code"

    Open the **Sessions** view, or run **Pegg: Resume Session**. The current
    session is marked with a green icon and a `current` label. Use **Pegg: New
    Session** to start a fresh chat; opening a new chat does not auto-resume the
    last session.

=== "HTTP"

    ```bash
    curl -X POST http://127.0.0.1:8080/api/run \
      -H 'Content-Type: application/json' \
      -d '{"message":"continue","session_id":"3f2b1c8e-..."}'
    ```

=== "ACP"

    `session/resume` (no history replay) or `session/load` (replays history as
    `session/update` notifications). See [ACP](../usage/acp.md).

Resuming publishes a `session.resume` event so the recorder re-attaches the run to
the same session rather than creating a new one.

## Live runs across clients

A run is owned by exactly one process (TUI, CLI, HTTP server, ACP/IDE, Connect,
or the SDK engine). Its stream is journaled in the shared session store, so every
other client that opens the same session attaches to the live run instead of
showing a stale transcript.

- Opening a session that is already running replays the journal and continues
  streaming tokens, reasoning, tool calls, and compaction events live. The
  subscription stays open after the run finishes, so the next message sent from
  any client is streamed too.
- A single process-wide watcher tails the shared journal (250 ms when idle, faster
  while a run is active); individual clients do not poll independently.
- While a run is active, sending a new prompt from another client is rejected;
  the client stays in read-only attach mode until the run finishes.
- Canceling the run (TUI double-Esc, `pegg sessions cancel`, HTTP
  `POST /api/runs/{id}/cancel`, ACP `session/cancel`, or the Connect stop button)
  is delivered to the owner process and propagates to every attached client.
- If the owning process crashes, its heartbeat stops and other clients mark the
  run as failed after ~15 seconds.
- Permission/ask questions are journaled too, so any attached client can answer
  them; the first answer wins and the others dismiss the prompt.

This coordination requires the `sqlite` session driver (the default) so
processes can share the run registry. The `json` driver keeps working in-process
but does not synchronize separate processes.

## Listing and inspecting

=== "CLI"

    ```bash
    pegg sessions list                    # current project only
    pegg sessions list --all --search fix # everywhere, filtered by title
    pegg sessions show 3f2b1c8e-...       # metadata + messages
    ```

=== "HTTP"

    ```bash
    curl 'http://127.0.0.1:8080/api/sessions?page=1&size=20'
    curl http://127.0.0.1:8080/api/sessions/3f2b1c8e-...
    curl http://127.0.0.1:8080/api/sessions/3f2b1c8e-.../messages
    ```

=== "ACP"

    `session/list` returns id, title, and timestamps for the most recent sessions.

Listings only show **original** sessions: compacted follow-up sessions (see below)
are internal views of the same conversation and never appear in session lists.

## Compaction and session chains

When [context compaction](../config.md#compaction) triggers, the compacted messages
are stored in a **new session linked to the current one**, forming a chain. The
original session keeps its full history; the newest session in the chain holds the
compacted view plus everything said afterwards.

- Resuming an original session opens the **newest session in its chain** — you see
  the latest compacted conversation first.
- In the TUI, scrolling to the top walks back through the chain: an
  `─ context compacted ─` divider marks each earlier conversation, ending with the
  original one.
- In the CLI, HTTP (SSE), and ACP, a compaction mid-run is surfaced as a
  `compaction` stream event / `↻ Context compacted (...)` line.
- Resuming re-applies compaction to the loaded history (sliding-window and
  tool-clearing), so resumed conversations stay within the context budget.
- Deleting an original session deletes its whole compaction chain.

## Managing sessions

| Action | CLI | TUI | VS Code | HTTP | ACP |
|---|---|---|---|---|---|
| List | `sessions list` | Settings → Session → Load | Sessions view | `GET /api/sessions` | `session/list` |
| Show details | `sessions show <id>` | transcript | chat transcript | `GET /api/sessions/{id}` | — |
| Rename | — | Settings → Session → Change title | **Pegg: Rename Session** | — | `session/rename` |
| Delete | — | Settings → Session → Delete | **Pegg: Delete Session** | `DELETE /api/sessions/{id}` | `session/delete` |
| Resume | `run --session` | Load session | `POST /api/run` | `session/resume` / `session/load` |

Deleting a session permanently removes its messages and usage from the store.

## Forking and reverting

The session layer also supports **forking** (copy a session up to a message into a
new session, recording a `parent_id`) and **reverting** (truncate after a message,
with the removed messages stored as a snapshot for later restore). These APIs are
exposed in the [Go SDK](../../sdk/sessions.md) but are not wired into the CLI, TUI, or
HTTP API yet.

## Tips

- Sessions are scoped to a project directory; use `--all` (CLI) to see everything.
- Run `pegg doctor` or check the status bar to see cumulative cost per session.
- ACP sessions created with `session/new` appear in listings titled `ACP Session`.