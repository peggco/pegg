<div align="center">

<img src="https://cdn.pegg.dev/logo/infinity.gif" alt="Pegg" width="420" />

# Pegg for VS Code

### The same agent. Inside your editor.

Pegg is a provider-agnostic AI coding agent in a single Go binary. This extension
brings the CLI and TUI engine into VS Code over ACP: streaming chat, tools,
sessions, subagents, plans, permissions, attachments, voice input, and editor
actions.

[Marketplace](https://marketplace.visualstudio.com/items?itemName=peggco.pegg-vscode) · [Open VSX](https://open-vsx.org/extension/peggco/pegg-vscode) · [Docs](https://docs.pegg.dev/pegg/usage/vscode/) · [Website](https://pegg.dev) · [Discussions](https://github.com/peggco/pegg/discussions)

[![VS Code Marketplace](https://img.shields.io/visual-studio-marketplace/v/peggco.pegg-vscode?style=flat-square&label=marketplace)](https://marketplace.visualstudio.com/items?itemName=peggco.pegg-vscode)
[![Installs](https://img.shields.io/visual-studio-marketplace/i/peggco.pegg-vscode?style=flat-square)](https://marketplace.visualstudio.com/items?itemName=peggco.pegg-vscode)
[![License: FSL-1.1-MIT](https://img.shields.io/badge/license-FSL--1.1--MIT-black?style=flat-square)](https://github.com/peggco/pegg/blob/master/LICENSE.md)

**Install**

```bash
code --install-extension peggco.pegg-vscode
```

</div>

```text
Pegg  ·  @pegg  ·  Claude Sonnet

› Refactor the auth middleware and add tests

● orchestrator
  read   internal/auth/middleware.go
  edit   internal/auth/middleware.go   +24 −9
  bash   go test ./internal/auth
  ✓      41 passed
```

> The extension is a thin ACP client. It does not proxy your keys and it does
> not send code to a hosted service. The real `pegg` binary runs on the machine
> that owns the workspace.

---

## Features

- **Chat** — streaming markdown, syntax highlighting, reasoning, tool cards,
  edit diffs, plans, token usage, and USD cost.
- **Subagents** — each subagent gets its own card and an in-place transcript.
- **Sessions** — list, resume, rename, delete, replay, fork, and revert.
- **Modes** — Auto, Plan, and Build.
- **Models** — provider/model picker with the providers you already configured.
- **Permissions** — inline `ask` and `judge` prompts with Allow, Allow All,
  Reject, and Reject Always.
- **Attachments** — files, images, selections, `@` mentions, and voice.
- **Skills and commands** — `/init`, `/batch`, `/review`, `/rule`, and custom
  skills.
- **`@pegg` participant** — `/explain`, `/fix`, `/review`, `/tests`, `/init`,
  `/rule`, and `/batch` in the Chat view.
- **Editor actions** — explain, edit, ask, review, fix, security review,
  refactor, optimize, document, generate tests, and fix diagnostics from the
  context menu, editor title, lightbulb, and command palette.
- **Working-tree review** — review the diff before you keep it.

## Requirements

| | |
|---|---|
| **VS Code** | 1.90 or newer |
| **Workspace** | Trusted — Pegg can edit files and run commands |
| **Provider** | At least one provider configured with `pegg login` or **Pegg: Provider Login** |
| **Binary** | Downloaded automatically, or provided through `pegg.executablePath`, `PATH`, or `<workspace>/bin/pegg` |

The extension is a **workspace extension**, so it also works in remote and WSL
windows and downloads the binary for the remote platform.

## First run

1. Install the extension.
2. Open the Pegg view in the activity bar, or run **Pegg: Open Chat**.
3. Run **Pegg: Provider Login** if no provider is configured.
4. Ask a question, type `/` for skills, or type `@` to mention files.

The extension runs the real binary over ACP stdio:

```bash
pegg serve --acp --stdio
```

## Binary resolution

The extension probes, in order:

1. `pegg.executablePath`
2. `<workspace>/bin/pegg` when `pegg.binary.allowWorkspaceBinary` is enabled
3. `pegg` on `PATH`
4. Well-known install directories (`/opt/homebrew/bin`, `/usr/local/bin`,
   `~/.local/bin`, `~/bin`, `~/.pegg/bin`) and the `PATH` declared by
   `~/.zshrc`, `~/.zprofile`, `~/.bash_profile`, or `~/.profile`
5. A checksum-verified release binary downloaded into VS Code global storage

Set `pegg.binary.version` to `match-extension` (default), a tag such as
`v0.2.4`, or `latest`. Set `pegg.binary.autoDownload` to `false` to disable
downloads.

Run **Pegg: Diagnose Binary** to see every location that was probed, the version
that was detected, and why a candidate was rejected.

## Commands

| Command | Description |
|---|---|
| `Pegg: Open Chat` | Focus the Pegg chat view |
| `Pegg: New Session` | Start a fresh session |
| `Pegg: Select Mode` | Auto, Plan, or Build |
| `Pegg: Select Model` | Pick a configured provider/model |
| `Pegg: Resume Session` | Load a previous session |
| `Pegg: AI Actions…` | Quick pick with selection or file actions |
| `Pegg: Explain Selection` | Explain the selection or current line |
| `Pegg: Edit Selection…` | Describe a change and apply it |
| `Pegg: Ask About Selection…` | Ask a free-form question |
| `Pegg: Review Changes` | Review the working-tree diff |
| `Pegg: Provider Login` | Run `pegg login` in a terminal |
| `Pegg: Diagnose Binary` | Show binary discovery details |
| `Pegg: Install or Update Binary` | Force a fresh download |

## Settings

| Setting | Default | Description |
|---|---|---|
| `pegg.executablePath` | `""` | Explicit binary path |
| `pegg.binary.autoDownload` | `true` | Download a binary when needed |
| `pegg.binary.allowWorkspaceBinary` | `true` | Allow `<workspace>/bin/pegg` in trusted workspaces |
| `pegg.binary.version` | `match-extension` | Release tag to download |
| `pegg.downloadBaseUrl` | GitHub releases | Download base URL |
| `pegg.provider` / `pegg.model` | `""` | Preferred provider and model |
| `pegg.defaultMode` | `auto` | New session mode |
| `pegg.context.includeActiveFile` / `includeSelection` | `true` | Editor context |
| `pegg.context.maxFileBytes` | `262144` | Max inlined file size |
| `pegg.chat.sendOnEnter` | `enter` | Send with Enter or Ctrl/Cmd+Enter |
| `pegg.notifications.onComplete` | `true` | Notify when unfocused |
| `pegg.env` | `{}` | Extra environment variables |
| `pegg.logLevel` | `info` | Extension log level |

## Security and privacy

- No telemetry from the extension.
- Downloads are checksum-verified.
- Workspace trust is required because the agent can run commands and edit files.
- API keys stay in Pegg's own config; the extension never logs them.
- The webview uses a strict CSP, a per-load nonce, and sanitized markdown.

## Troubleshooting

- **Binary not found** — run `Pegg: Diagnose Binary`, then
  `Pegg: Install or Update Binary`, or set `pegg.executablePath`.
- **Started from the Dock / Finder** — GUI launches have a minimal `PATH`; the
  extension also checks the usual install directories and shell profiles.
- **No models configured** — run `Pegg: Provider Login`.
- **Server stopped** — run `Pegg: Restart Server` and inspect `Pegg: Show Logs`.
- **Permissions hang** — update the Pegg binary so it advertises interactive ACP
  permissions.
- **Remote / WSL** — the extension runs as a workspace extension and downloads
  the binary for the remote platform.

## Development

```bash
npm --prefix ../chat-ui ci
npm ci
npm run check          # typecheck + lint + unit tests + build
npm run package        # build a .vsix
```

Integration tests spawn the real `pegg` binary over ACP and are skipped unless
`PEGG_TEST_BINARY` is set:

```bash
npm run test:integration
npm run test:download
```

## License

FSL-1.1-MIT. See [LICENSE.md](https://github.com/peggco/pegg/blob/master/LICENSE.md).
