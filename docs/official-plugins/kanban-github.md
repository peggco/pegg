---
icon: lucide/git-branch
---

# Kanban GitHub

GitHub issues, worktrees, run feedback, and pull requests for the
[Kanban](kanban.md) board.

This plugin is hooks-only. It contributes no agent tools. Install it alongside
`pegg-kanban`.

## Install

```bash
pegg plugins install peggco/pegg-kanban
pegg plugins install peggco/pegg-kanban-github
```

Verify with:

```bash
pegg plugins list --runtime
```

## Hooks

| Hook | Kind | Purpose |
|---|---|---|
| `kanban.providers` | filter | Advertise GitHub as an issue provider (`id: github`). |
| `kanban.issues.fetch` | filter | Import open GitHub issues as cards. |
| `kanban.run.prepare` | filter | Create a git worktree and `pegg/<slug>` branch for a card run. |
| `kanban.card.succeeded` | action | Comment the run summary and open a pull request. |

Provider IDs stay `github`, so existing boards and sync state keep working.

## Configuration

Options live under `plugins.options["kanban-github"]` in `~/.pegg/pegg.json`. The
legacy keys `github` and `pegg-github` are read as fallbacks.

```json
{
  "plugins": {
    "options": {
      "kanban-github": {
        "enabled": true,
        "token": "",
        "owner": "",
        "repo": "",
        "api_url": "https://api.github.com",
        "default_base": "",
        "worktree_root": ".pegg-worktrees",
        "request_timeout_seconds": 30,
        "sync_state": "open",
        "sync_labels": [],
        "sync_limit": 50
      }
    }
  }
}
```

| Setting | Meaning |
|---|---|
| `enabled` | Enable the integration. |
| `token` | GitHub token. Falls back to `PEGG_GITHUB_TOKEN`, `GITHUB_TOKEN`, `GH_TOKEN`, then `gh auth token`. |
| `owner` / `repo` | Default repository. `PEGG_GITHUB_REPO=owner/name` overrides both. |
| `api_url` | API base for GitHub Enterprise. |
| `default_base` | Base branch for worktrees and pull requests. |
| `worktree_root` | Worktree directory. Default `.pegg-worktrees`. |
| `sync_state` / `sync_labels` / `sync_limit` | Issue import filters. |

Environment: `PEGG_GITHUB_TOKEN`, `PEGG_GITHUB_REPO`, `PEGG_GITHUB_OWNER`,
`PEGG_GITHUB_REPO_NAME`, `PEGG_GITHUB_API_URL`, `PEGG_GITHUB_WORKTREE_ROOT`,
`PEGG_GITHUB_DEFAULT_BASE`, `PEGG_GITHUB_TIMEOUT_SECONDS`.

## Behavior

- `kanban.issues.fetch` maps GitHub issues to cards with `source=github`, the issue number,
  URL, labels, and state.
- `kanban.run.prepare` creates a worktree at `<worktree_root>/<slug>` on branch
  `pegg/<slug>`, based on the card's base branch or `default_base`. It returns a
  `context` with `kind: "worktree"`.
- `kanban.card.succeeded` posts the run summary as an issue comment when `feedback` is
  enabled and opens a pull request from `context.branch` when
  `create_change_request` is enabled. The PR targets `context.base`, then
  `base_branch`, then `default_base`, then the repository default branch.
- Tokens are never printed, logged, or returned. Errors are redacted.

## Example

```bash
pegg kanban start
pegg kanban sync --provider github --target acme/widgets
pegg kanban add "Fix login redirect" --column ready
pegg kanban move <card-id> in_progress
```

When the run succeeds, the plugin comments on the issue and, if enabled, opens a pull
request from the card branch.

## Notes

- Worktrees are created by this plugin, not by Kanban core.
- Auto-PR needs a configured remote and a pushed branch.
- Use a token with `repo` scope for private repositories.
