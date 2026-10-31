---
icon: lucide/book-open
---

# Pegg Documentation

Pegg is a provider-agnostic AI coding agent written in Go. It runs as an interactive
terminal application, a headless CLI, an HTTP API server, an
[Agent Client Protocol](pegg/usage/acp.md) server, a
[VS Code extension](pegg/usage/vscode.md), or as an embeddable
[Go SDK](sdk/index.md) — all inside a sandboxed workspace.

## Quick start

```bash
# macOS / Linux
curl -fsSL https://pegg.dev/install | bash
```

```powershell
# Windows (PowerShell)
irm https://pegg.dev/install.ps1 | iex
```

```bash
pegg login   # add a provider API key
pegg         # launch the TUI
```

Prefer VS Code?

```bash
code --install-extension peggco.pegg-vscode
```

Open a trusted workspace, run **Pegg: Provider Login**, and open the Pegg chat. See
the [VS Code guide](pegg/usage/vscode.md).

See [Installation](pegg/installation.md) for npm, prebuilt binaries,
`go install`, and build-from-source options.

<div class="grid cards" markdown>

- [:lucide-rocket:{ .lg } **Pegg**](pegg/overview.md)

    The coding agent itself: [installation](pegg/installation.md),
    [providers & models](pegg/providers-and-models.md), [config](pegg/config.md),
    and [usage](pegg/usage/cli.md) across CLI, TUI, VS Code, HTTP, and ACP.

- [:lucide-box:{ .lg } **SDK**](sdk/index.md)

    Embed the full agent engine in your own Go program: chats, streaming,
    sessions, sandboxed files, and custom tools.

- [:lucide-bot:{ .lg } **Peggbot**](peggbot/index.md)

    Run Pegg as a GitHub bot: workflows, prompts, and GitHub tooling for
    issues and pull requests.

- [:lucide-puzzle:{ .lg } **Plugin**](plugin/index.md)

    Extend Pegg with plugins: quickstart, interface reference, and
    development guide.

</div>

## Explore

| Section | Where to go |
|---|---|
| First steps | [Installation](pegg/installation.md), [Providers & Models](pegg/providers-and-models.md), [Config](pegg/config.md) |
| Everyday use | [TUI](pegg/usage/tui.md), [VS Code](pegg/usage/vscode.md), [CLI](pegg/usage/cli.md), [Desktop](pegg/usage/desktop.md), [Adding Context](pegg/features/adding-context.md), [Sessions](pegg/features/sessions.md) |
| Configuration | [Tools](pegg/configurations/tools.md), [Skills](pegg/configurations/skills.md), [Rules](pegg/configurations/rules.md), [MCP](pegg/configurations/mcp.md), [LSP](pegg/configurations/lsp.md), [Plugins](pegg/configurations/plugins.md) |
| Going deeper | [Subagents](pegg/features/subagents.md), [Permissions](pegg/features/permissions.md), [Memory](pegg/features/memory.md), [SDK](sdk/index.md) |
| Getting unstuck | [Troubleshooting](pegg/troubleshooting.md) |

## Resources

- **Source code** — [github.com/peggco/pegg](https://github.com/peggco/pegg)
- **Releases** — [GitHub releases](https://github.com/peggco/pegg/releases)
- **Website** — [pegg.dev](https://pegg.dev)
