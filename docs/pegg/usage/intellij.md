---
icon: lucide/braces
---

# IntelliJ IDEs

The Pegg plugin for JetBrains IDEs embeds the same agent engine used by the CLI,
TUI, and VS Code extension. It runs the local `pegg` binary over the
[Agent Client Protocol](acp.md) and exposes chat, sessions, tools, subagents,
plans, permissions, attachments, voice input, and editor actions.

It works in IntelliJ IDEA, PhpStorm, WebStorm, PyCharm, CLion, Android Studio,
and other IntelliJ Platform products.

## Requirements

| | |
|---|---|
| **IntelliJ Platform** | 2025.3.1 or newer |
| **JCEF** | Enabled (the default in JetBrains IDEs) |
| **Project** | Trusted — Pegg can edit files and run commands |
| **Provider** | At least one provider configured with `pegg login` |
| **Binary** | Installed automatically, or provided through settings, `PATH`, or `<project>/bin/pegg` |

The plugin depends only on the platform, language, and JCEF modules, so the same
artifact works across JetBrains products. Android Studio must be on a
2025.3.1+ platform build.

## Install

Download `pegg-intellij-<version>.zip` from the release page, then open
**Settings → Plugins → ⚙ → Install Plugin from Disk…** and restart the IDE.

For local development:

```bash
npm --prefix pkg/chat-ui ci
cd pkg/intellij
npm ci
./gradlew buildPlugin
```

The installable zip is in `pkg/intellij/build/distributions/`.

## First run

1. Open a trusted project.
2. Run **Tools → Pegg → Provider Login**, or configure
   [`~/.pegg/pegg.json`](../config.md).
3. Open the **Pegg** tool window on the right side.
4. Ask a question, type `/` for skills, or type `@` to mention files.

## Tool window

The Pegg tool window has two tabs:

- **Pegg** — the chat, composer, plan, and approvals.
- **Sessions** — session history shared with the CLI and VS Code.

The chat tab is the same shared `pkg/chat-ui` renderer as the VS Code extension, rendered with JCEF. It
supports streaming markdown, syntax highlighting, reasoning, tool cards with
output and diffs, subagent cards with in-place transcripts, plan updates, usage
and cost, permission cards, and ask-user forms.

### Composer

| Action | How |
|---|---|
| Attach files | `+` button, drag and drop, or paste an image |
| Mention a file | Type `@` and pick from the fuzzy file list |
| Run a skill | Type `/` and pick a skill/command |
| Voice input | Mic button |
| Change mode | Mode chip: Auto, Plan, or Build |
| Change model | Model chip |
| Settings | Sliders icon |
| Send | Enter or the send arrow |
| New line | Shift+Enter |
| Stop | Stop button while the agent is running |

## Sessions

The **Sessions** tab lists sessions for the current project and other projects.
It supports:

- Search by title, id, model, or mode.
- Resume with double-click or Enter.
- Rename with F2 or the context menu.
- Delete with Delete or the context menu.
- Refresh from the toolbar.

The current session is marked with a green icon.

## Modes

| Mode | Behavior |
|---|---|
| **Auto** | Pegg decides when to plan and when to execute |
| **Plan** | Read-only planning; writes are limited to `.pegg/plans` |
| **Build** | Execute changes immediately |

## Editor actions

With text selected, right-click and choose **Pegg**:

- Explain Selection
- Edit Selection…
- Ask About Selection…
- Fix Selection
- Find Bugs in Selection
- Review Selection
- Security Review
- Refactor Selection
- Optimize Selection
- Document Selection
- Generate Tests
- Add Selection to Chat

With no selection, the same menu offers file actions: explain, edit, ask,
review, document, generate tests, fix diagnostics, and add file to chat.
Actions are also available in the project view, the editor tab menu, and
**Tools → Pegg**.

Alt+Enter intentions add **Explain with Pegg**, **Fix with Pegg**,
**Review with Pegg**, **Generate tests with Pegg**, **Add to Pegg chat**, and
diagnostic-oriented actions.

!!! note

    IntelliJ does not provide a public API for third-party plugins to inject
    quick fixes into arbitrary existing inspections. Pegg adds its own
    intentions and actions rather than modifying other plugins' lightbulb
    entries.

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

Change them in **Settings → Keymap** by searching for `Pegg`.

## Binary resolution

In order:

1. **Settings → Tools → Pegg → Executable path**
2. `<project>/bin/pegg` when allowed in a trusted project
3. `pegg` on `PATH`
4. Well-known install directories (`/opt/homebrew/bin`, `/usr/local/bin`,
   `~/.local/bin`, `~/bin`, `~/.pegg/bin`) and the `PATH` declared by
   `~/.zshrc`, `~/.zprofile`, `~/.bash_profile`, and `~/.profile`
5. A checksum-verified release downloaded into the IDE config directory

The release tag defaults to the plugin version (`match-extension`). Set it to
`latest` or a specific tag in settings. **Diagnose Binary** lists every probed
location and the reason a candidate was rejected.

## Settings

| Setting | Default | Description |
|---|---|---|
| Executable path | `""` | Explicit path to the `pegg` binary |
| Auto download | on | Download a matching release when no binary is found |
| Allow workspace binary | on | Allow `<project>/bin/pegg` in trusted projects |
| Release tag | `match-extension` | `match-extension`, `latest`, or a specific tag |
| Download base URL | GitHub releases | Release download base URL |
| Preferred provider | `""` | Provider for new sessions |
| Preferred model | `""` | Model for new sessions |
| Default mode | `auto` | Mode for new sessions |
| Include active file | on | Add the active file path to prompts |
| Include selection | on | Add the active selection to prompts |
| Max attachment bytes | 262144 | Maximum text attachment size |
| Send on | `enter` | Send with Enter or Ctrl/Cmd+Enter |
| Notify on complete | on | Notify when a run finishes while the IDE is unfocused |
| Environment variables | `{}` | Extra environment variables for the ACP process |
| Log level | `info` | `error`, `warn`, `info`, or `debug` |

## Permissions and voice

Permission rules live in [`pegg.json`](../config.md) under `permission`. The
plugin bridges `ask`, `semi-ask`, `judge`, and `semi-judge` decisions to inline
cards in the chat, with a modal fallback when the chat tab is not visible.

Voice input uses the Pegg engine, not the browser/IDE microphone. Pegg records
with `sox`, `arecord`, or `ffmpeg` on the machine running the agent, or the
`stt.record_command` override. See [Voice input](../features/voice.md).

## Troubleshooting

### The binary is missing

Run **Tools → Pegg → Install or Update Binary**, or set the executable path.
**Diagnose Binary** shows every candidate and why it failed.

### The binary is old

Update the CLI with **Install or Update Binary**, or point the executable path
at a newer build. Older binaries do not advertise interactive permissions or
rich tool events.

### The server stops

Open **Tools → Pegg → Show Logs**, then **Restart Server**. The current session
is restored on the next run.

### JCEF is disabled

The chat tab shows a fallback message. Enable JCEF with
**Help → Find Action → Choose Boot Java Runtime**, then restart the IDE.

### Android Studio

Use an Android Studio build based on IntelliJ Platform 2025.3.1 or newer.
Older Android Studio releases do not expose the stable JCEF module alias used
by this plugin.

## Related

- [ACP](acp.md) — the protocol used by the plugin.
- [VS Code](vscode.md) — the same chat UI and features in VS Code.
- [Sessions](../features/sessions.md)
- [Permissions](../features/permissions.md)
- [Voice input](../features/voice.md)
