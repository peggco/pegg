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

When Pegg Desktop starts, it imports the login shell's `PATH` and the git-host
credential variables above, so `gh auth token` and `GITHUB_TOKEN` resolve the same way
they do in the TUI. If `gh` is installed somewhere the login shell does not expose, set
`token` in `plugins.options["kanban-github"]` or export `PEGG_GITHUB_TOKEN` for the
desktop session. Settings → Plugins shows the plugin's boot result and error, so a
failing credential lookup is no longer silent.

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

### Change-request routing

Kanban resolves a `change_request_provider` per card and passes it in the
`kanban.run.prepare` and `kanban.card.succeeded` payloads. This plugin handles both hooks
only when that value is `github` (or, for older Kanban versions that omit the field, when
the issue provider is `github`). The issue comment is only posted when the issue itself
came from GitHub, so a Jira issue routed to GitHub opens a pull request without a GitHub
comment.

To route another issue source to GitHub, set the route in the Kanban options:

```json
{
  "plugins": {
    "options": {
      "kanban": {
        "change_request_providers": {"jira": "github"}
      }
    }
  }
}
```

The repository is taken from the issue repo when the issue came from GitHub, and from
`owner`/`repo` (or the git remote) otherwise.

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
