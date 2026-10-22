# Pegg for VS Code

Pegg is a provider-agnostic AI coding agent. This extension brings the same engine
used by the Pegg CLI and TUI into VS Code: chat, tools, sessions, modes, subagents,
plans, permissions, attachments, voice input, and editor integrations.

## Requirements

- VS Code 1.90 or newer.
- A trusted workspace. Pegg can edit files and run commands.
- One or more LLM providers configured for Pegg.

## Getting started

1. Install the extension.
2. Open the Pegg view in the activity bar, or run **Pegg: Open Chat**.
3. If no Pegg binary is found, the extension downloads the matching release into
   VS Code's global storage and verifies it against the release checksums.
4. Run **Pegg: Provider Login** to configure a provider, or edit
   `~/.pegg/pegg.json` with **Pegg: Open Global Config**.
5. Ask Pegg a question or use `@pegg` in the Chat view and in compatible editors.

The extension uses the real `pegg` binary over ACP stdio:

```bash
pegg serve --acp --stdio
```

## Binary resolution

In order:

1. `pegg.executablePath`
2. `<workspace>/bin/pegg` when `pegg.binary.allowWorkspaceBinary` is enabled
3. `pegg` on `PATH`
4. `pegg` in the well known install directories (`/opt/homebrew/bin`,
   `/usr/local/bin`, `~/.local/bin`, `~/bin`, `~/.pegg/bin`) and in the `PATH`
   declared by the shell profiles (`~/.zshrc`, `~/.zprofile`, `~/.bash_profile`,
   `~/.profile`). This keeps the extension working when VS Code is started
   from the Dock or the Finder, where the `PATH` is minimal.
5. A downloaded release binary in global storage

Downloads are verified with `checksums.txt` and cached by release tag. Set
`pegg.binary.version` to `match-extension` (default), a tag such as `v0.1.9`, or
`latest`. Set `pegg.binary.autoDownload` to `false` to disable downloads.

Run `Pegg: Diagnose Binary` to see every location that was probed, the version
that was detected, and the exact reason a candidate was rejected.

## Features

- Streaming chat with markdown, syntax highlighting, reasoning, tool calls,
  diffs, plan/todo updates, usage and cost.
- Modes: **Auto**, **Plan**, and **Build**.
- Model and provider picker.
- Session list, resume, rename, delete, and replay.
- Click a user message for **Copy**, **Fork from here** and **Revert to here**
  (the same actions as the TUI). Forking opens a new session with the messages
  before that point; reverting removes the message and everything after it and
  puts the text back into the composer.
- Interactive permission approvals and `askuserquestion` forms.
- Slash commands and skills (`/init`, `/batch`, `/review`, `/rule`, custom skills).
- File, image, selection, `@` mention, and voice attachments.
- `@pegg` chat participant with `/explain`, `/fix`, `/review`, `/tests`, `/init`,
  `/rule`, and `/batch`.
- Editor context menu/submenu, editor title button, quick-pick AI actions,
  code actions/lightbulb actions, diagnostics fixes, custom Ask/Edit prompts,
  SCM review, and a status bar.
- Config, login, doctor, update, and log commands.

## Commands

Open the command palette and type `Pegg`. Highlights:

| Command | Description |
|---|---|
| `Pegg: Open Chat` | Focus the Pegg chat view |
| `Pegg: New Session` | Start a fresh session |
| `Pegg: Select Mode` | Auto, Plan, or Build |
| `Pegg: Select Model` | Pick a configured provider/model |
| `Pegg: Resume Session` | Load a previous session |
| `Pegg: AI Actions…` | Quick pick with selection or file actions |
| `Pegg: Explain Selection` | Explain the selection or current line |
| `Pegg: Edit Selection…` | Describe a change and apply it to the selection |
| `Pegg: Ask About Selection…` | Ask a free-form question about the selection |
| `Pegg: Find Bugs in Selection` | Focused correctness review |
| `Pegg: Security Review Selection` | Security-focused review |
| `Pegg: Optimize Selection` | Improve performance and clarity |
| `Pegg: Document Selection` | Add or improve documentation comments |
| `Pegg: Fix All Diagnostics in File` | Fix errors and warnings in the active file |
| `Pegg: Review Changes` | Review the working tree diff |
| `Pegg: Provider Login` | Run `pegg login` in a terminal |
| `Pegg: Connect Team` | Run `pegg team` to connect a Pegg team |
| `Pegg: Install or Update Binary` | Force a fresh binary download |

## Settings

| Setting | Default | Description |
|---|---|---|
| `pegg.executablePath` | `""` | Explicit binary path |
| `pegg.binary.autoDownload` | `true` | Download a binary when needed |
| `pegg.binary.allowWorkspaceBinary` | `true` | Allow executing `<workspace>/bin/pegg` in trusted workspaces |
| `pegg.binary.version` | `match-extension` | Release tag to download |
| `pegg.downloadBaseUrl` | GitHub releases | Download base URL |
| `pegg.provider` | `""` | Preferred provider |
| `pegg.model` | `""` | Preferred model |
| `pegg.defaultMode` | `auto` | New session mode |
| `pegg.context.includeActiveFile` | `true` | Include active file context |
| `pegg.context.includeSelection` | `true` | Include active selection |
| `pegg.context.maxFileBytes` | `262144` | Max inlined file size |
| `pegg.chat.sendOnEnter` | `enter` | Send with Enter or Ctrl/Cmd+Enter |
| `pegg.notifications.onComplete` | `true` | Notify when unfocused |
| `pegg.env` | `{}` | Extra environment variables |
| `pegg.logLevel` | `info` | Extension log level |

## Permissions

Pegg's permission rules live in `~/.pegg/pegg.json` under `permission`. The
extension bridges `ask` and `judge` prompts to VS Code. `Allow`, `Allow All`,
`Reject`, and `Reject Always` work as they do in the CLI.

## Voice input

Voice is recorded by the Pegg engine on the machine running the agent, so it
works in local, remote, and WSL windows without browser microphone permissions.
Pegg uses `sox`, `arecord`, or `ffmpeg` (whichever is installed), or the
`stt.record_command` override in `pegg.json`. Configure the STT provider/model
under `stt` in `pegg.json`.

## Security and privacy

- No telemetry is collected by this extension.
- Downloads are checksum-verified.
- Workspace trust is required because the agent can execute commands and edit
  files.
- API keys stay in Pegg's own config; the extension does not log them.
- The webview uses a strict CSP, a per-load nonce, and sanitized markdown.

## Troubleshooting

- **Binary not found**: run `Pegg: Diagnose Binary` to see what was probed and
  why it failed, then `Pegg: Install or Update Binary`, or set
  `pegg.executablePath`.
- **"Downloaded Pegg binary failed its version check"**: the message now
  includes the reason reported by the binary. A common cause is a working
  directory that cannot be written to; `pegg version` no longer depends on one.
- **Started from the Dock / Finder**: the `PATH` of a GUI launch is minimal, so
  the extension also searches the usual install directories and the `PATH` from
  your shell profiles.
- **No models configured**: run `Pegg: Provider Login`.
- **Server stopped**: run `Pegg: Restart Server` and inspect `Pegg: Show Logs`.
- **Permissions hang**: update the Pegg binary so it advertises interactive ACP
  permissions.
- **Remote / WSL**: the extension runs as a workspace extension and downloads the
  binary for the remote platform.

## License

FSL-1.1-MIT. See [LICENSE.md](LICENSE.md).

## Development

```bash
npm ci
npm run check          # typecheck + lint + unit tests + build
npm run package        # build a .vsix

# Requires a configured Pegg install in the current environment:
npm run test:integration
npm run test:download   # verifies GitHub download + checksum + extraction
```

The integration tests spawn the real `pegg` binary over ACP and are skipped when
`PEGG_TEST_BINARY` is not set.
