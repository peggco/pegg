---
icon: lucide/scroll-text
---

# Logs

Search and inspect the Pegg system log database. Logs provides read-only search, level and
time filters, counts, CSV/JSON export, a local dashboard, and a TUI/desktop settings page.

## Install

```bash
pegg plugins install peggco/pegg-logs
```

Then load the `logs` skill before the first tool call:

```text
/logs
```

## Data source

- Database: `~/.pegg/logs.db` (override with `db_path` or `PEGG_LOGS_DB`).
- Table: `logs(id, timestamp, level, message)`.
- The database is opened read-only with `mode=ro` and `query_only(1)`.
- Levels: `DEBUG`, `INFO`, `WARN`, `ERROR`.

## Configuration

```json
{
  "plugins": {
    "options": {
      "logs": {
        "enabled": true,
        "db_path": "~/.pegg/logs.db",
        "default_range_days": 7,
        "default_limit": 100,
        "request_timeout_seconds": 30
      }
    }
  }
}
```

Environment variables:

| Variable | Meaning |
|---|---|
| `PEGG_LOGS_DB` | Database path. |
| `PEGG_LOGS_RANGE_DAYS` | Default range in days. |
| `PEGG_LOGS_LIMIT` | Default page size. |
| `PEGG_LOGS_TIMEOUT_SECONDS` | Query timeout. |
| `PEGG_LOGS_DISABLED` | Disable the plugin. |
| `PEGG_LOGS_DASHBOARD_TOKEN` | Override the dashboard token. |

## Tools

| Tool | Permission | Description |
|---|---|---|
| `logs_search` | allow | Search by `levels`, `query`, `from`, `to`, `days`, `limit`, `page`, and `sort`. |
| `logs_stats` | allow | Count logs by level and day for a filtered range. |
| `logs_export` | semi-judge | Export filtered logs to CSV or JSON. |

`logs_export` defaults to `.pegg/logs/logs-<timestamp>.<format>` under the workspace.
Relative paths resolve against the workspace; absolute paths are honored.

## Dashboard

```bash
pegg logview open
pegg logview start
pegg logview stop
```

The dashboard binds `127.0.0.1` on a random port with a random token. Runtime state lives
at `~/.pegg/logs/server.json` with mode `0600`. The UI includes level filters, text search,
date presets and custom ranges, page size, auto-refresh, summary cards, pagination, and CSV/JSON export.

## Commands

```text
pegg logview status                 Database, row count, latest log, dashboard state
pegg logview open                   Start the dashboard and open it
pegg logview start                  Start the dashboard without a browser
pegg logview stop                   Stop the dashboard
pegg logview search --level ERROR   Search logs
pegg logview export --format csv    Export filtered logs
pegg logview config                 Resolved settings
```

## TUI and desktop

The plugin contributes a **Logs** settings page with total/error/warn/info/debug counts,
recent logs, dashboard controls, configuration fields, and CSV export. Desktop settings
render the same page automatically.

## Privacy

- The database is opened read-only and never modified.
- The dashboard binds loopback and requires a random token.
- Tokens are never included in tool output or errors.
