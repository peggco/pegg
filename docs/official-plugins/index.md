---
icon: lucide/blocks
---

# Official Plugins

Official Pegg plugins are standalone binaries maintained by the Pegg team. They extend the
agent with browsers, desktop control, Kanban boards, issue-tracker integrations, and token
analytics.

Every plugin uses the same plugin protocol as third-party plugins. Pegg loads it from
`~/.pegg/plugins`, exposes its tools and skills to the agent, and runs it in a separate
process.

## Install

```bash
pegg plugins install peggco/pegg-browser-use
pegg plugins install peggco/pegg-computer-use
pegg plugins install peggco/pegg-kanban
pegg plugins install peggco/pegg-kanban-github
pegg plugins install peggco/pegg-kanban-gitlab
pegg plugins install peggco/pegg-kanban-jira
pegg plugins install peggco/pegg-kanban-bitbucket
pegg plugins install peggco/pegg-usage
pegg plugins install peggco/pegg-logs
```

Verify the installed plugins:

```bash
pegg plugins list
pegg plugins list --runtime
```

Update them with:

```bash
pegg plugins update --all
pegg plugins update --all --check
```

## Official plugins

| Plugin | Package | What it does |
|---|---|---|
| [Browser Use](browser-use.md) | `peggco/pegg-browser-use` | Real Chromium browser control with snapshots, forms, uploads, downloads, tabs, and screenshots. |
| [Computer Use](computer-use.md) | `peggco/pegg-computer-use` | Desktop screenshots, mouse, keyboard, windows, clipboard, and a decision-model task loop. |
| [Kanban](kanban.md) | `peggco/pegg-kanban` | Local board that runs agent cards, exposes a web UI, and accepts integration hooks. |
| [Usage](usage.md) | `peggco/pegg-usage` | Local token and cost analytics from the Pegg session database. |
| [Logs](logs.md) | `peggco/pegg-logs` | Read-only search and dashboard for the Pegg system log database. |

### Kanban integrations

Kanban integration plugins are hooks-only. Install `pegg-kanban` alongside them; they
contribute no agent tools.

| Integration | Package | Provides |
|---|---|---|
| [GitHub](kanban-github.md) | `peggco/pegg-kanban-github` | GitHub issue sync, worktrees, feedback comments, and pull requests. |
| [GitLab](kanban-gitlab.md) | `peggco/pegg-kanban-gitlab` | GitLab issue sync, worktrees, feedback notes, and merge requests. |
| [Jira](kanban-jira.md) | `peggco/pegg-kanban-jira` | Jira issue sync, comments, and status transitions. |
| [Bitbucket](kanban-bitbucket.md) | `peggco/pegg-kanban-bitbucket` | Worktrees and pull requests, paired with Jira, GitHub, or GitLab issues. |

## Configuration

Plugin options live under `plugins.options.<plugin-name>` in `~/.pegg/pegg.json`:

```json
{
  "plugins": {
    "enabled": true,
    "options": {
      "browser-use": {
        "headless": true
      },
      "kanban-github": {
        "repo": "acme/widgets"
      }
    }
  }
}
```

Environment variables override the file for most plugins and use the plugin name as a
prefix, for example `PEGG_BROWSER_HEADLESS`, `PEGG_GITHUB_TOKEN`, or `PEGG_USAGE_DB`.
Each plugin page lists its own options and environment variables.

## Version compatibility

Catalog entries can declare the Pegg version and plugin protocol they support:

```json
"requires": {
  "pegg": ">=0.2.4 <0.3.0",
  "protocol": 2
}
```

`pegg plugins info`, `plugins install`, and `plugins update` print a warning when the
current Pegg version or protocol does not match. A warning does not block installation.

To pin a specific release, pass a tag:

```bash
pegg plugins install peggco/pegg-kanban --version v0.1.2
```

Pegg does not automatically select an older compatible release when the latest one
requires a newer Pegg. Pin the version explicitly if you are staying on an older Pegg
release.

## Security

Plugins run as separate processes with the permissions of your user. Review a plugin
before installing it. Official plugins never print tokens, cookies, or credentials in
tool output or errors.
