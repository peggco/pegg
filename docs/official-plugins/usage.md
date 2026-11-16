---
icon: lucide/chart-no-axes-column-increasing
---

# Usage

Know where your tokens go. Usage reads the local Pegg session database and reports token
and cost analytics: summaries, breakdowns, session lists, model totals, charts, and CSV
or JSON export.

## Install

```bash
pegg plugins install peggco/pegg-usage
```

Then load the `usage` skill before the first analytics tool call:

```text
/usage
```

## Data source

- Database: `~/.pegg/sessions.db` (override with `db_path` or `PEGG_USAGE_DB`).
- Tables: `sessions` and `session_messages`.
- Usage keys: `prompt_tokens`, `completion_tokens`, `total_tokens`, `cost`, and optional
  `cached_tokens` / `reasoning_tokens`.
- The database is opened read-only with `mode=ro` and `query_only(1)`. The plugin never
  writes to the session database.
- `cost_coverage_pct` reports how many sessions carried a cost value.

## Filters

All tools, the dashboard, and `summary`/`export` share the same filters:

| Field | Meaning |
|---|---|
| `from`, `to` | RFC3339 or `YYYY-MM-DD`. Date-only `to` includes the whole day. |
| `days` | Lookback when `from`/`to` are absent. Default `default_range_days` (30). |
| `project` | Project directory path or name substring. |
| `provider`, `model`, `mode` | Case-insensitive exact filters. |
| `include_subagents` | Include subagent sessions. |
| `min_tokens` | Minimum total tokens per session. |

## Configuration

```json
{
  "plugins": {
    "options": {
      "usage": {
        "enabled": true,
        "db_path": "~/.pegg/sessions.db",
        "default_range_days": 30,
        "include_subagents": false,
        "request_timeout_seconds": 30
      }
    }
  }
}
```

Environment variables:

| Variable | Meaning |
|---|---|
| `PEGG_USAGE_DB` | Database path. |
| `PEGG_USAGE_RANGE_DAYS` | Default range in days. |
| `PEGG_USAGE_INCLUDE_SUBAGENTS` | Include subagent sessions. |
| `PEGG_USAGE_TIMEOUT_SECONDS` | Query timeout. |
| `PEGG_USAGE_DISABLED` | Disable the tools. |

## Tools

| Tool | Permission | Description |
|---|---|---|
| `usage_summary` | allow | Sessions, messages, tokens, cost, coverage, averages, and range. |
| `usage_breakdown` | allow | Tokens/cost/sessions by `day`, `week`, `month`, `model`, `provider`, `project`, or `mode`. |
| `usage_sessions` | allow | Session rows with message/token/cost counts; sort by `cost`, `tokens`, `updated`, or `messages`. |
| `usage_models` | allow | Per-model and per-provider totals and averages. |
| `usage_export` | semi-judge | Write the filtered breakdown to CSV or JSON. |

`usage_export` defaults to `.pegg/usage/usage-<timestamp>.csv` under the workspace.
Relative paths resolve against the workspace; absolute paths are honored.

## Dashboard

```bash
pegg usage open
pegg usage stop
```

The dashboard starts a detached server on `127.0.0.1` with a random port and token. The
token is required for every API request. Runtime state lives at
`~/.pegg/usage/server.json` with mode `0600`.

The UI includes date presets (7/30/90/all), custom ranges, project/provider/model/mode
filters, a subagent toggle, summary cards, SVG charts, sortable session lists, model and
project tables, and CSV export.

## Commands

```text
pegg usage status                 Database, row counts, dashboard state, defaults
pegg usage open                   Start the dashboard and open it
pegg usage stop                   Stop the dashboard
pegg usage summary --days 7       Text summary
pegg usage export --format csv    Write an export
pegg usage config                 Resolved settings
```

Direct CLI:

```bash
./bin/pegg-usage summary --days 7
./bin/pegg-usage export --format json --days 30 --path /tmp/usage.json
./bin/pegg-usage config
```

## Privacy

- The plugin reads only session and message-count metadata. Message content is never
  selected.
- The database is opened read-only.
- Tokens are never included in tool output or errors.
- The dashboard binds loopback and requires a random token.

## TUI

The Usage plugin contributes a TUI page with the same range filters, summary cards,
breakdowns, model totals, and a session list. Use it from Settings → Plugins, or run
`pegg usage open` for the web dashboard.
