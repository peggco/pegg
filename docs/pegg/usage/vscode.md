---
icon: lucide/code-xml
---

# VS Code

The Pegg VS Code extension embeds the same agent engine used by the CLI and TUI
inside the editor. It runs the local `pegg` binary over the
[Agent Client Protocol](acp.md) and exposes chat, sessions, tools, permissions,
subagents, plans, attachments, voice input, and editor actions without a hosted
service.

## Requirements

| | |
|---|---|
| **VS Code** | 1.90 or newer |
| **Workspace** | Trusted — Pegg can edit files and run commands |
| **Provider** | At least one provider configured with `pegg login` or **Pegg: Provider Login** |
| **Binary** | Installed automatically, or provided through `pegg.executablePath`, `PATH`, or `<workspace>/bin/pegg` |

The extension is a **workspace extension**, so it runs in remote and WSL windows
and downloads the correct platform binary for the remote machine.

## Install

=== "VS Code Marketplace"

    Search for **Pegg** in the Extensions view, or run:

    ```bash
    code --install-extension peggco.pegg-vscode
    ```

=== "Open VSX"

    Download the VSIX from
    [open-vsx.org](https://open-vsx.org/extension/peggco/pegg-vscode), then run
    `code --install-extension <downloaded>.vsix`. Editors that use Open VSX can
    also install it from their Extensions view.

=== "From a release"

    Download `pegg-vscode-<version>.vsix` from the
    [GitHub releases page](https://github.com/peggco/pegg/releases), then run:

    ```bash
    code --install-extension pegg-vscode-<version>.vsix
    ```

=== "Build from source"

    ```bash
    cd pkg/vscode
    npm ci
    npm run package
    code --install-extension pegg-vscode-*.vsix
    ```

Reload the window after installing.

## First run

1. Open a trusted workspace.
2. Run **Pegg: Provider Login** and add a provider API key, or configure
   [`~/.pegg/pegg.json`](../config.md) directly.
3. Open the Pegg icon in the activity bar, or run **Pegg: Open Chat**.
4. Ask a question, or type `/` for skills and `@` for files.

A new chat starts a new session. Use the **Sessions** view or **Pegg: Resume
Session** to continue an earlier chat. The current session is marked with a green
icon in the sessions tree.

## Chat

The sidebar chat supports:

- Streaming markdown with syntax highlighting.
- Collapsible reasoning, merged into a single block while streaming.
- Tool cards with command output, expandable input, copy, file locations, and
  edit diffs.
- Plan/todo updates, subagent activity, token usage, and USD cost.
- Permission and `askuserquestion` prompts rendered inline.
- Auto-scroll that follows the run only while you are at the bottom.

### Composer

| Action | How |
|---|---|
| Attach files | `+` button, drag and drop, or paste an image |
| Mention a file | Type `@` and pick from the fuzzy file list |
| Run a skill | Type `/` and pick a skill/command |
| Voice input | Mic button |
| Change mode | Mode chip: Auto, Plan, or Build |
| Change model | Model chip |
| Settings | Sliders icon left of the mic |
| Send | Enter or the send arrow |
| New line | Shift+Enter |
| Stop | Stop button while the agent is running |

`@` mentions and `/` skills are inline, with keyboard navigation:

- Up/Down — move the selection.
- Enter/Tab — accept.
- Esc — dismiss.

## Modes

| Mode | Behavior |
|---|---|
| **Auto** | Pegg decides when to plan and when to execute |
| **Plan** | Read-only planning; writes are limited to `.pegg/plans` |
| **Build** | Execute changes immediately |

## Sessions

Use the **Sessions** view or the command palette:

- **Pegg: Resume Session**
- **Pegg: New Session**
- **Pegg: Rename Session**
- **Pegg: Delete Session**
- **Pegg: Refresh Sessions**

Sessions are shared with the CLI and TUI. Resuming a session replays its history
and restores the stored provider, model, and mode.

## Editor integrations

Pegg adds actions to the editor, explorer, and source control views.

### Right-click menu

With text selected, right-click and choose **Pegg** or **AI Actions…**:

- Explain Selection
- Edit Selection…
- Ask About Selection…
- Fix Selection
- Find Bugs in Selection
- Review Selection
- Security Review Selection
- Refactor Selection
- Optimize Selection
- Document Selection
- Generate Tests
- Add Selection to Chat

With no selection, the menu offers file actions: explain, edit, ask, review,
document, generate tests, fix diagnostics, and add file to chat.

### Selection and file actions

Run **Pegg: AI Actions…** to open a quick pick for the current selection, or for
the active file when nothing is selected.

Selection actions fall back to the current line when no text is selected.

### Code actions

The lightbulb and Quick Fix menus include Pegg actions for selections and
diagnostics, including **Fix all diagnostics in this file**.

### Keybindings

| Key | Action |
|---|---|
| `Ctrl/Cmd+Alt+P` | Open Pegg chat |
| `Ctrl/Cmd+Alt+N` | New session |
| `Ctrl/Cmd+Alt+M` | Select mode |
| `Ctrl/Cmd+Alt+A` | Pegg AI Actions |
| `Ctrl/Cmd+Alt+E` | Explain selection |
| `Ctrl/Cmd+Alt+Q` | Ask about selection |
| `Ctrl/Cmd+Alt+X` | Edit selection |

### Chat participant

Use `@pegg` in the Chat view with:

- `/explain`
- `/fix`
- `/review`
- `/tests`
- `/init`
- `/rule`
- `/batch`

### Source control

**Pegg: Review Changes** reviews the working-tree diff. Use the SCM title action
or run **Pegg: Review Current File Changes** for the active file.

## Commands

| Command | Description |
|---|---|
| **Pegg: Open Chat** | Focus the Pegg chat view |
| **Pegg: New Session** | Start a fresh session |
| **Pegg: Stop Agent** | Cancel the running agent and subagents |
| **Pegg: Select Mode** | Auto, Plan, or Build |
| **Pegg: Select Model** | Choose a configured provider/model |
| **Pegg: Resume Session** | Load a previous session |
| **Pegg: Rename Session** | Rename a session |
| **Pegg: Delete Session** | Delete a session |
| **Pegg: Refresh Sessions** | Refresh the sessions tree |
| **Pegg: AI Actions…** | Selection or file quick pick |
| **Pegg: Explain Selection** | Explain the selection or current line |
| **Pegg: Edit Selection…** | Apply a custom instruction to the selection |
| **Pegg: Ask About Selection…** | Ask a free-form question |
| **Pegg: Find Bugs in Selection** | Focused correctness review |
| **Pegg: Security Review Selection** | Security-focused review |
| **Pegg: Optimize Selection** | Improve performance and clarity |
| **Pegg: Document Selection** | Add or improve doc comments |
| **Pegg: Review Changes** | Review the working-tree diff |
| **Pegg: Fix All Diagnostics in File** | Fix errors and warnings |
| **Pegg: Provider Login** | Run `pegg login` in a terminal |
| **Pegg: Connect Team** | Run `pegg team` in a terminal |
| **Pegg: Run Doctor** | Run `pegg doctor` |
| **Pegg: Check for Updates** | Run `pegg update` |
| **Pegg: Open Global Config** | Open `~/.pegg/pegg.json` |
| **Pegg: Show Logs** | Open the extension output channel |
| **Pegg: Restart Server** | Restart the ACP process |
| **Pegg: Install or Update Binary** | Force a verified binary download |
| **Pegg: Diagnose Binary** | List the probed binary locations and failure reasons |

## Binary resolution

In order:

1. `pegg.executablePath`
2. `<workspace>/bin/pegg` when `pegg.binary.allowWorkspaceBinary` is enabled
3. `pegg` on `PATH`
4. `pegg` in the well known install directories (`/opt/homebrew/bin`,
   `/usr/local/bin`, `~/.local/bin`, `~/bin`, `~/.pegg/bin`) and in the `PATH`
   declared by the shell profiles (`~/.zshrc`, `~/.zprofile`, `~/.bash_profile`,
   `~/.profile`)
5. A checksum-verified release binary downloaded into VS Code global storage

The downloaded version defaults to the extension version. Set
`pegg.binary.version` to `latest` or a specific tag to override it.

Step 4 exists because a GUI launch (Dock, Finder) starts VS Code with a
minimal `PATH`. Run `Pegg: Diagnose Binary` to see every location that was
probed, the detected version and why a candidate was rejected.

## Settings

| Setting | Default | Description |
|---|---|---|
| `pegg.executablePath` | `""` | Explicit path to the `pegg` binary |
| `pegg.binary.autoDownload` | `true` | Download a binary when none is found |
| `pegg.binary.allowWorkspaceBinary` | `true` | Allow `<workspace>/bin/pegg` in trusted workspaces |
| `pegg.binary.version` | `match-extension` | Release tag, `match-extension`, or `latest` |
| `pegg.downloadBaseUrl` | GitHub releases | Download base URL |
| `pegg.provider` | `""` | Preferred provider for new sessions |
| `pegg.model` | `""` | Preferred model for new sessions |
| `pegg.defaultMode` | `auto` | Mode for new sessions |
| `pegg.context.includeActiveFile` | `true` | Include the active file path |
| `pegg.context.includeSelection` | `true` | Include the active selection |
| `pegg.context.maxFileBytes` | `262144` | Maximum text attachment size |
| `pegg.chat.sendOnEnter` | `enter` | Send with Enter or Ctrl/Cmd+Enter |
| `pegg.notifications.onComplete` | `true` | Notify when a run finishes unfocused |
| `pegg.env` | `{}` | Extra environment variables for the Pegg process |
| `pegg.logLevel` | `info` | Extension log level |

## Voice input

Click the mic in the composer. Recording happens in the Pegg engine, not in the
VS Code webview, so no browser microphone permission is requested. Pegg uses
`sox`, `arecord`, or `ffmpeg` on the machine running the agent, or the
`stt.record_command` override in `pegg.json`.

Configure an STT provider and model under `stt` in [`pegg.json`](../config.md).
See [Voice input](../features/voice.md) for provider setup.

## Permissions

Permission rules live in `pegg.json` under `permission`. The extension bridges
`ask`, `semi-ask`, `judge`, and `semi-judge` decisions to VS Code:

- **Allow** — run this call.
- **Allow All** — run and remember this call.
- **Reject** — deny this call; Pegg asks for an optional reason.
- **Reject Always** — deny this call.

See [Permissions](../features/permissions.md).

## Remote, WSL, and multi-root workspaces

- The extension runs as a workspace extension. In remote and WSL windows the
  binary is resolved on the remote machine.
- A new chat uses the first workspace folder, or the folder of the active editor.
- Use one Pegg session per workspace to keep context focused.

## Security

- Workspace trust is required because the agent can run commands and edit files.
- Release binaries are verified against `checksums.txt` before installation.
- No telemetry is collected by the extension.
- API keys remain in Pegg's config and are redacted from extension logs.
- Markdown is sanitized, and the webview uses a strict CSP with a per-load nonce.

## Troubleshooting

### The binary is missing

Run **Pegg: Install or Update Binary**, or set `pegg.executablePath`. To use a
workspace build, run `make build` and keep
`pegg.binary.allowWorkspaceBinary` enabled.

### No models are configured

Run **Pegg: Provider Login**, or open **Pegg: Open Global Config** and add a
provider.

### The extension says the binary is old

Update the CLI with **Pegg: Install or Update Binary**, or point
`pegg.executablePath` at a newer binary. Older binaries do not advertise
interactive permissions or rich tool events.

### The server stops or a request fails

Open **Pegg: Show Logs**, run **Pegg: Restart Server**, and retry. Sessions are
restored on the next message.

### Voice recording fails

Install `sox`, `arecord`, or `ffmpeg` on the machine running Pegg, verify the
microphone works there, and configure `stt.provider`/`stt.model`.

### Remote microphone issues

Recording uses the remote machine's audio devices. If the remote host has no
microphone, use the CLI on the local machine or configure a custom
`stt.record_command`.

## Related

- [ACP](acp.md) — the protocol used by the extension.
- [Sessions](../features/sessions.md)
- [Permissions](../features/permissions.md)
- [Voice input](../features/voice.md)
- [Team AI](../features/team-ai.md)
- [Adding Context](../features/adding-context.md)
