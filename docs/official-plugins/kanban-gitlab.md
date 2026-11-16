---
icon: lucide/git-branch
---

# Kanban GitLab

GitLab issues, worktrees, run notes, and merge requests for the
[Kanban](kanban.md) board.

This plugin is hooks-only and contributes no agent tools. Install it alongside
`pegg-kanban`.

## Install

```bash
pegg plugins install peggco/pegg-kanban
pegg plugins install peggco/pegg-kanban-gitlab
```

## Hooks

Provider id: `gitlab`.

| Hook | Kind | Priority | Purpose |
|---|---|---|---|
| `kanban.providers` | filter | 10 | Advertise GitLab as an issue provider. |
| `kanban.issues.fetch` | filter | 10 | Fetch GitLab issues for Kanban sync. |
| `kanban.run.prepare` | filter | 10 | Create a git worktree and branch for a card run. |
| `kanban.card.succeeded` | action | 10 | Comment the summary and open a merge request. |

## Configuration

```json
{
  "plugins": {
    "options": {
      "kanban-gitlab": {
        "enabled": true,
        "token": "glpat-...",
        "base_url": "https://gitlab.com/api/v4",
        "project": "group/project",
        "default_target_branch": "main",
        "worktree_root": ".pegg-worktrees",
        "sync_state": "opened",
        "sync_labels": ["kanban"],
        "sync_limit": 50,
        "request_timeout_seconds": 30
      }
    }
  }
}
```

The option key `pegg-kanban-gitlab` is also accepted. Legacy keys `gitlab` and
`pegg-gitlab` remain as fallbacks.

| Setting | Meaning |
|---|---|
| `token` | GitLab access token. |
| `base_url` | GitLab API base. Use your instance URL for self-managed GitLab. |
| `project` | Project path or numeric ID. |
| `default_target_branch` | Target branch for merge requests and worktrees. |
| `worktree_root` | Worktree directory. Default `.pegg-worktrees`. |
| `sync_state` / `sync_labels` / `sync_limit` | Issue import filters. |

Environment overrides:

| Variable | Purpose |
|---|---|
| `PEGG_GITLAB_TOKEN` | Preferred token. |
| `GITLAB_TOKEN` | Token fallback. |
| `CI_JOB_TOKEN` | Read-only token fallback. |
| `PEGG_GITLAB_URL` | API base URL. |
| `PEGG_GITLAB_PROJECT` | Project path or numeric ID. |
| `PEGG_GITLAB_WORKTREE_ROOT` | Worktree directory. |
| `PEGG_GITLAB_TIMEOUT_SECONDS` | API timeout. |

## Behavior

- `kanban.issues.fetch` maps GitLab issues to cards with `source=gitlab`, the IID, URL,
  labels, assignees, and state.
- `kanban.run.prepare` creates a worktree under `worktree_root` on a `pegg/<slug>`
  branch and returns a `context` with `kind: "worktree"`.
- `kanban.card.succeeded` comments the run summary when `feedback` is enabled and
  creates a merge request from `context.branch` into `context.base` or
  `default_target_branch` when `create_change_request` is enabled.
- Existing merge requests produce an actionable error instead of a duplicate.

### Change-request routing

Kanban resolves a `change_request_provider` per card and passes it in the
`kanban.run.prepare` and `kanban.card.succeeded` payloads. This plugin handles both hooks
only when that value is `gitlab` (or, for older Kanban versions that omit the field, when
the issue provider is `gitlab`). The issue note is only posted when the issue itself came
from GitLab, so a Jira issue routed to GitLab opens a merge request without a GitLab note.

To route another issue source to GitLab, set the route in the Kanban options:

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

The project is taken from the issue repo when the issue came from GitLab, and from
`project` otherwise.

## Token scopes

Use a project or group access token with `api` scope (or `read_api` for sync-only usage).
For self-managed GitLab, set `base_url` to `https://gitlab.example.com/api/v4`.

## Notes

- Worktrees are created by this plugin, not by Kanban core.
- The target branch must exist in the repository.
- Tokens are never printed or returned; errors are redacted.
