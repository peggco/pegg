---
icon: lucide/kanban
---

# Kanban

A local, file-backed Kanban board with a web UI and TUI. Moving a card into the trigger
column starts a Pegg agent run. By default the agent runs in the repository root; an
integration plugin can provide a different run environment, such as a git worktree, a
container, or a sandbox.

## Install

```bash
pegg plugins install peggco/pegg-kanban
```

Then start the board:

```bash
pegg kanban start
pegg kanban open
```

The board belongs to the current workspace and is stored at
`<workdir>/.pegg/kanban/board.json`.

## Quick start

```bash
pegg kanban
pegg kanban add "Fix the login redirect" --column ready --body "Reproduce and fix it"
pegg kanban list
pegg kanban move <card-id> in_progress
pegg kanban logs <card-id> --tail 100
pegg kanban stop
```

Direct CLI is also available:

```bash
cd /path/to/repo
./bin/pegg-kanban add "Title" --column ready
./bin/pegg-kanban start
./bin/pegg-kanban list
```

## IDE checkout

VS Code (**Pegg: Checkout Card Branch**) and IntelliJ
(**Tools → Pegg → Checkout Card Branch…**) read cards from `board.json` and
check out a card's branch without leaving the editor. The extensions detect
GitHub, Bitbucket, or GitLab from the remote, reuse an existing branch, or
create one from the default branch with the configured template.

## Configuration

```json
{
  "plugins": {
    "options": {
      "kanban": {
        "enabled": true,
        "host": "127.0.0.1",
        "port": 7878,
        "auto_open": true,
        "max_parallel": 2,
        "trigger_column": "in_progress",
        "review_column": "review",
        "auto_move_on_success": true,
        "auto_answer": true,
        "base_branch": "",
        "sync_column": "backlog",
        "sync_minutes": 0,
        "sync_update_existing": true,
        "sync_labels": [],
        "sync_limit": 50,
        "feedback": true,
        "create_change_request": true,
        "change_request_provider": "",
        "change_request_providers": {},
        "hooks_bin": "",
        "hooks_timeout_seconds": 90
      }
    }
  }
}
```

| Option | Default | Meaning |
|---|---|---|
| `host` / `port` | `127.0.0.1:7878` | Web UI bind address. Non-loopback requires `api_token`. |
| `api_token` | empty | Require `X-PegG-Token` or `Authorization: Bearer` on `/api/*`. |
| `max_parallel` | `2` | Concurrent agent runs. |
| `trigger_column` | `in_progress` | Column that starts a run. |
| `review_column` | `review` | Column for successful runs. |
| `auto_move_on_success` | `true` | Move succeeded cards to review. |
| `auto_answer` | `true` | Feed `auto_answer_value` to interactive prompts. |
| `base_branch` | empty | Base branch passed to the run-preparation hook. |
| `sync_column` | `backlog` | Import destination column. |
| `sync_minutes` | `0` | Scheduled sync interval; `0` disables. |
| `sync_update_existing` | `true` | Refresh imported cards outside trigger/terminal columns. |
| `sync_labels` / `sync_limit` / `sync_state` | defaults | Issue import filters. |
| `feedback` | `true` | Let integrations comment after a successful run. |
| `create_change_request` | `true` | Let integrations open a PR/MR after a successful run. |
| `change_request_provider` | empty | Change-request provider for cards with no issue (or no route). |
| `change_request_providers` | `{}` | Route issue provider → change-request provider, e.g. `{"jira":"bitbucket"}`. |
| `transition_on_success` | empty | Status transition passed to integrations. |

Environment variables use the `PEGG_KANBAN_` prefix, for example
`PEGG_KANBAN_HOST`, `PEGG_KANBAN_PORT`, `PEGG_KANBAN_MAX_PARALLEL`,
`PEGG_KANBAN_TOKEN`, `PEGG_KANBAN_CREATE_CHANGE_REQUEST`,
`PEGG_KANBAN_CHANGE_REQUEST_PROVIDER`, and `PEGG_KANBAN_CHANGE_REQUEST_PROVIDERS`.

## Run environments and hooks

Kanban core does not create worktrees. Integrations contribute behavior through these
hooks:

| Hook | Kind | Purpose |
|---|---|---|
| `kanban.providers` | filter | Add an issue sync provider. |
| `kanban.issues.fetch` | filter | Fetch and normalize issues. |
| `kanban.run.prepare` | filter | Change the working directory, branch, environment, or metadata for a run. |
| `kanban.card.succeeded` | action | React to a successful run, such as commenting, transitioning, or opening a PR/MR. |
| `kanban.change_request.routes` | filter | Contribute default issue → change-request routes. |

`kanban.run.prepare` receives:

```json
{
  "card": {"id": "card-...", "title": "Fix login"},
  "repo": "/abs/path/to/repo",
  "base_branch": "main",
  "context": {"kind": "repo", "path": "/abs/path/to/repo", "base": "main", "label": "repository"}
}
```

A handler returns the same value with a modified `context`:

```json
{"context": {"kind": "worktree", "path": "/repo/.pegg-worktrees/card-...", "branch": "pegg/card-...", "base": "main", "label": "card-..."}}
```

`kind` is free-form. `path` becomes `cmd.Dir`, `branch` appears in the prompt, and `env` is
merged into the child process environment. With no handler, the run uses the repository
root. `card.context` stores a sanitized copy for display; `env` is never persisted.

## Change-request routing

The issue source and the change-request target are independent. Kanban resolves a
`change_request_provider` for every card and passes it to `kanban.run.prepare` and
`kanban.card.succeeded`:

1. If `change_request_providers` maps the card's issue provider, use that value (an empty
   value disables the change request for that provider).
2. Otherwise use the issue provider itself.
3. For cards with no issue, use `change_request_provider`.

Default routes come from the `kanban.change_request.routes` filter, so Kanban stays
provider-neutral. `kanban-bitbucket` contributes `{"jira": "bitbucket"}` by default, so
Jira issues open Bitbucket pull requests out of the box. User config in
`change_request_providers` always wins over plugin defaults.

Each git-host plugin handles `kanban.run.prepare` and `kanban.card.succeeded` only when it
is the resolved `change_request_provider`. Issue comments and transitions stay gated on the
issue's own provider, so a Jira issue routed to Bitbucket is still commented and
transitioned by `kanban-jira`.

With `create_change_request` enabled (the default), a GitLab issue opens a GitLab MR, a
GitHub issue opens a GitHub PR, and a Jira issue opens a Bitbucket PR — no extra routing
config needed. Override the target per issue provider:

```json
{
  "plugins": {
    "options": {
      "kanban": {
        "change_request_providers": {"jira": "gitlab"}
      }
    }
  }
}
```

## Integration plugins

Install one or more hooks-only integrations alongside Kanban:

| Plugin | Provides |
|---|---|
| [Kanban GitHub](kanban-github.md) | GitHub issues, worktrees, comments, pull requests |
| [Kanban GitLab](kanban-gitlab.md) | GitLab issues, worktrees, notes, merge requests |
| [Kanban Jira](kanban-jira.md) | Jira issues, comments, transitions |
| [Kanban Bitbucket](kanban-bitbucket.md) | Worktrees and pull requests (paired with Jira, GitHub, or GitLab issues) |

Without an integration plugin, manual cards, drag and drop, agent runs, logs, SSE, and
CLI/TUI actions all work offline.

## Tools

| Tool | Type | Purpose |
|---|---|---|
| `kanban_status` | read-only | Server, board, sync, and provider status |
| `kanban_list` | read-only | List cards by column or run status |
| `kanban_add` | write | Create a card |
| `kanban_update` | write | Edit title, body, priority, labels, assignee, or column |
| `kanban_move` | write | Move a card; entering the trigger column starts a run |
| `kanban_run` | write | Start a card explicitly |
| `kanban_cancel` | write | Cancel a queued or running card |
| `kanban_log` | read-only | Tail a card log |
| `kanban_sync` | write | Import issues from hook providers |
| `kanban_config` | read-only | Sanitized configuration |

## Commands

```text
pegg kanban                  Start detached, open the browser, print the URL
pegg kanban start            Start detached
pegg kanban stop             Stop a detached server
pegg kanban status           Health and board summary
pegg kanban add <title>      --column --body --priority --labels --assignee
pegg kanban list             --column
pegg kanban move <id> <col>  Move a card
pegg kanban sync             --provider --target --query --labels --state --limit
pegg kanban logs <id>        --tail
pegg kanban open             Start if needed and open the browser
```

TUI and desktop settings start an embedded server owned by that window; closing the
window stops it. If a healthy server for the same board already exists, it is reused.

## Web UI and HTTP API

`http://127.0.0.1:7878/` serves the embedded board UI. The API includes `/api/board`,
`/api/cards`, `/api/cards/{id}/move`, `/api/cards/{id}/run`, `/api/cards/{id}/log`,
`/api/sync`, `/api/events` (SSE), `/api/config`, and `/api/health`.

Request bodies are capped at 1 MiB. The server binds loopback by default, and errors are
redacted for tokens and authorization headers.

## Files on disk

```text
<workdir>/.pegg/kanban/
├── board.json          # board, columns, cards, run state, per-provider sync state
├── board.json.lock     # write lock
├── events.jsonl        # append-only events
├── logs/<card-id>.log  # agent stdout/stderr
├── server.json         # server metadata
└── server.log          # detached server log
```

## Limitations

- Process-group cancel is POSIX-specific.
- Scheduled sync runs only while the server process is alive.
- The web UI is plain HTML/CSS/JS with no build step.

## Kanban integrations

Kanban core is provider-neutral. Install one or more hooks-only integrations alongside
`pegg-kanban`:

| Integration | Package | Provides |
|---|---|---|
| [GitHub](kanban-github.md) | `peggco/pegg-kanban-github` | GitHub issue sync, worktrees, feedback comments, and pull requests. |
| [GitLab](kanban-gitlab.md) | `peggco/pegg-kanban-gitlab` | GitLab issue sync, worktrees, feedback notes, and merge requests. |
| [Jira](kanban-jira.md) | `peggco/pegg-kanban-jira` | Jira issue sync, comments, and status transitions. |
| [Bitbucket](kanban-bitbucket.md) | `peggco/pegg-kanban-bitbucket` | Worktrees and pull requests, paired with Jira, GitHub, or GitLab issues. |

```bash
pegg plugins install peggco/pegg-kanban
pegg plugins install peggco/pegg-kanban-github
pegg plugins install peggco/pegg-kanban-gitlab
pegg plugins install peggco/pegg-kanban-jira
pegg plugins install peggco/pegg-kanban-bitbucket
```

Each integration registers `kanban.providers`, `kanban.issues.fetch`,
`kanban.run.prepare`, `kanban.card.succeeded`, and/or `kanban.change_request.routes`
handlers. Without an integration, manual cards, drag and drop, agent runs, logs, SSE, and
CLI/TUI actions all work offline.
