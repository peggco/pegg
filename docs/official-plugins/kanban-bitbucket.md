---
icon: lucide/git-branch
---

# Kanban Bitbucket

Worktrees and pull requests for the [Kanban](kanban.md) board, with issues provided by
Jira, GitHub, or GitLab.

This plugin is hooks-only. It contributes no agent tools and does not fetch issues.

## What it does

- `kanban.run.prepare` creates a git worktree and a `pegg/<slug>` branch for a card run.
- `kanban.card.succeeded` opens a pull request from that branch into the card's base
  branch.

Bitbucket Cloud Issues was removed, and Bitbucket Data Center has no built-in issue
tracker, so this plugin does not sync issues. Pair it with
[Kanban Jira](kanban-jira.md), [Kanban GitHub](kanban-github.md), or
[Kanban GitLab](kanban-gitlab.md).

## Install

```bash
pegg plugins install peggco/pegg-kanban
pegg plugins install peggco/pegg-kanban-bitbucket
```

## Jira → Bitbucket flow

1. `kanban-jira` imports issues into the board.
2. A card run starts. `kanban-bitbucket` creates `.pegg-worktrees/<slug>` on branch
   `pegg/<slug>`.
3. The agent commits to that branch.
4. `kanban.card.succeeded` opens the pull request in Bitbucket.

The card's issue provider can be Jira, GitHub, or GitLab. The plugin does not require
`card.issue.provider == "bitbucket"`.

## Hooks

| Hook | Kind | Priority | Purpose |
|---|---|---|---|
| `kanban.run.prepare` | filter | 10 | Create a worktree with a `pegg/<slug>` branch. |
| `kanban.card.succeeded` | action | 10 | Open a Bitbucket pull request. |

Both hooks are no-ops when the plugin is disabled.

`kanban.run.prepare` reads the repository from `repo`, creates a worktree under
`worktree_root` (default `.pegg-worktrees`), and returns a `context`:

```json
{
  "kind": "worktree",
  "path": "/repo/.pegg-worktrees/fix-login-abc12345",
  "branch": "pegg/fix-login-abc12345",
  "base": "main",
  "label": "fix-login-abc12345"
}
```

`kanban.card.succeeded` reads the branch from `context.branch` and the destination from
`context.base`, then `base_branch`, then `default_base`, then the repository default
branch.

## Configuration

### Bitbucket Cloud

```json
{
  "plugins": {
    "options": {
      "kanban-bitbucket": {
        "enabled": true,
        "kind": "cloud",
        "workspace": "acme",
        "repo": "widgets",
        "email": "bot@example.com",
        "api_token": "ATBB...",
        "default_base": "main",
        "worktree_root": ".pegg-worktrees",
        "request_timeout_seconds": 30
      }
    }
  }
}
```

### Bitbucket Data Center

```json
{
  "plugins": {
    "options": {
      "kanban-bitbucket": {
        "enabled": true,
        "kind": "datacenter",
        "base_url": "https://bitbucket.example.com",
        "project": "PROJ",
        "repo": "widgets",
        "token": "http-access-token",
        "default_base": "main"
      }
    }
  }
}
```

`kind` also accepts `server` and `dc`. The option keys `pegg-kanban-bitbucket`,
`bitbucket`, and `pegg-bitbucket` are fallbacks.

### Environment

| Variable | Purpose |
|---|---|
| `PEGG_BITBUCKET_ENABLED` | Set `false` to disable. |
| `PEGG_BITBUCKET_KIND` | `cloud` or `datacenter`. |
| `PEGG_BITBUCKET_URL` | API base URL. |
| `PEGG_BITBUCKET_WORKSPACE` | Cloud workspace. |
| `PEGG_BITBUCKET_PROJECT` | Data Center project key. |
| `PEGG_BITBUCKET_REPO` | Repository slug. |
| `PEGG_BITBUCKET_EMAIL` | Cloud email. |
| `PEGG_BITBUCKET_API_TOKEN` | Cloud API token. |
| `PEGG_BITBUCKET_ACCESS_TOKEN` | Cloud bearer token. |
| `PEGG_BITBUCKET_USERNAME` / `PEGG_BITBUCKET_PASSWORD` | Data Center basic auth. |
| `PEGG_BITBUCKET_TOKEN` | Data Center HTTP access token. |
| `PEGG_BITBUCKET_DEFAULT_BASE` | Default destination branch. |
| `PEGG_BITBUCKET_WORKTREE_ROOT` | Worktree directory. |
| `PEGG_BITBUCKET_TIMEOUT_SECONDS` | API timeout. |

## Authentication

Bitbucket Cloud uses an API token with your Atlassian email, sent as HTTP Basic, or an
`access_token` sent as Bearer. App passwords are deprecated.

Bitbucket Data Center uses an HTTP access token (`token`, Bearer) or
`username` + `password` basic auth.

Missing credentials produce an actionable error naming the setting and environment
variable. Tokens and passwords are redacted from hook errors.

## Notes

- Worktrees are created by this plugin, not by Kanban core.
- Duplicate pull requests produce an error instead of a second PR.
- Bitbucket remotes in the document worktree are used to infer the workspace/project and
  repository when not configured.
