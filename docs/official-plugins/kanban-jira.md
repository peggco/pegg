---
icon: lucide/square-kanban
---

# Kanban Jira

Jira issue sync, run comments, and status transitions for the
[Kanban](kanban.md) board.

This plugin is hooks-only and contributes no agent tools. Install it alongside
`pegg-kanban`.

## Install

```bash
pegg plugins install peggco/pegg-kanban
pegg plugins install peggco/pegg-kanban-jira
```

## Setup

1. Create a Jira Cloud API token at
   [id.atlassian.com](https://id.atlassian.com/manage-profile/security/api-tokens).
2. Add the plugin options to `~/.pegg/pegg.json`:

```json
{
  "plugins": {
    "options": {
      "kanban-jira": {
        "enabled": true,
        "site": "acme",
        "email": "you@example.com",
        "token": "ATATT...",
        "project": "ABC",
        "default_jql": "project = ABC AND status != Done ORDER BY updated DESC",
        "request_timeout_seconds": 30
      }
    }
  }
}
```

`site` accepts `acme`, `acme.atlassian.net`, or a full URL. The derived base URL is
`https://<site>.atlassian.net` unless a full URL is supplied.

The preferred option keys are `kanban-jira` and `pegg-kanban-jira`. Legacy
`jira` and `pegg-jira` keys are read as fallbacks.

## Environment variables

| Variable | Meaning |
|---|---|
| `PEGG_JIRA_OPTIONS` | JSON options object |
| `PEGG_JIRA_SITE` | Jira site or full URL |
| `PEGG_JIRA_EMAIL` | Atlassian account email |
| `PEGG_JIRA_TOKEN` | API token |
| `PEGG_JIRA_PROJECT` | Default project key |
| `PEGG_JIRA_JQL` | Default JQL when a fetch request has no query |
| `PEGG_JIRA_SYNC_STATE` | Default state filter |
| `PEGG_JIRA_SYNC_LABELS` | JSON array or comma-separated labels |
| `PEGG_JIRA_SYNC_LIMIT` | Default fetch limit (default 30, max 100) |
| `PEGG_JIRA_DEFAULT_TRANSITION` | Fallback success transition |
| `PEGG_JIRA_TIMEOUT_SECONDS` | Request timeout |
| `PEGG_JIRA_DISABLED` | `1` or `true` disables the hooks |

## Hooks

| Hook | Kind | Priority | Purpose |
|---|---|---|---|
| `kanban.providers` | filter | 10 | Advertise Jira as an issue provider. |
| `kanban.issues.fetch` | filter | 10 | Fetch Jira issues for Kanban sync. |
| `kanban.card.succeeded` | action | 10 | Comment the summary and transition the issue. |

A disabled plugin, an unknown hook, or a request for another provider returns
`handled: false` without an error.

### Issue sync

`kanban.issues.fetch` accepts `{provider,target,labels,state,limit,query,issues}`.
Only `provider: "jira"` is handled. `query` overrides JQL; otherwise `default_jql` is
used, or a query is built from the project plus `sync_state` and `sync_labels`.

Issues are appended with `key`, `title`, `body`, `url`, `labels`, `assignees`, `state`,
`priority`, and timestamps. Descriptions are converted from Atlassian Document Format to
Markdown. The fetch limit is clamped to 100.

### Card success

`kanban.card.succeeded` handles cards whose `card.issue.provider` is `jira`:

- With `feedback` enabled and a non-empty `summary`, the summary is posted as a comment.
- With `transition_on_success` set, the issue is transitioned to that status. A
  configured `default_transition` is used when the payload has none.

## Notes

- Jira Cloud API tokens are used with HTTP basic auth (`email` + `token`).
- The project key is required for sync.
- Tokens are never printed or returned; errors are redacted.
