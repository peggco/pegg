# Pegg for JetBrains IDEs

Pegg brings the same agent engine used by the CLI, TUI, and VS Code extension into
IntelliJ IDEA, PhpStorm, WebStorm, PyCharm, CLion, Android Studio, and other
IntelliJ Platform IDEs.

The plugin runs the real `pegg` binary over the
[Agent Client Protocol](../../docs/pegg/usage/acp.md) (`pegg serve --acp --stdio`).
It is not a hosted service and it does not proxy your provider keys.

## Features

- **Chat** — streaming markdown, syntax highlighting, reasoning, tool cards,
  edit diffs, plans, token usage, and USD cost.
- **Subagents** — subagent cards open an in-place transcript with a Back action.
- **Sessions** — a separate **Sessions** tab lists sessions shared with the CLI
  and VS Code. Resume, rename, delete, search, and refresh.
- **Composer** — `@` file mentions, `/` skills and commands, attachments,
  drag-and-drop, pasted images, voice input, mode and model pickers.
- **Permissions and questions** — inline cards in the chat; modal fallbacks when
  the chat tab is not visible.
- **Editor actions** — explain, edit, ask, review, fix, security review, refactor,
  optimize, document, generate tests, fix diagnostics, and add to chat. Actions
  appear in the editor and project-view context menus, the Tools menu, and
  Alt+Enter intentions.
- **Session history actions** — copy, fork from here, and revert to here from a
  user message.
- **Binary management** — resolve an installed binary or download a
  checksum-verified release into the IDE config directory.
- **Status bar** — mode, model, token usage, and a shortcut to open chat.

## Requirements

| | |
|---|---|
| IntelliJ Platform | 2025.3.1 or newer |
| Java runtime | JBR 21 (bundled with current IDEs) |
| JCEF | Enabled (default in JetBrains IDEs) |
| Project | Trusted — Pegg can edit files and run commands |
| Provider | At least one provider configured with `pegg login` |

The plugin depends only on `com.intellij.modules.platform`,
`com.intellij.modules.lang`, and `com.intellij.modules.jcef`, so the same
artifact works across JetBrains products. Android Studio must be running a
2025.3.1+ platform build.

## Install

### From a release

Download `pegg-intellij-<version>.zip` from the release page, then:

**Settings → Plugins → ⚙ → Install Plugin from Disk…** and restart the IDE.

### Build from source

```bash
npm --prefix pkg/chat-ui ci
cd pkg/intellij
npm ci
./gradlew buildPlugin
```

The installable zip is written to `pkg/intellij/build/distributions/`.

### Run a development IDE

```bash
cd pkg/intellij
./gradlew runIde
```

## First run

1. Open a trusted project.
2. Run **Tools → Pegg → Provider Login**, or add a provider to
   `~/.pegg/pegg.json`.
3. Open the **Pegg** tool window on the right.
4. Ask a question, type `/` for skills, or type `@` to mention a file.

The plugin starts `pegg serve --acp --stdio` lazily and shows a binary download
progress notification if no binary is available.

## Binary resolution

In order:

1. **Settings → Tools → Pegg → Executable path**
2. `<project>/bin/pegg` (trusted projects, when allowed)
3. `pegg` on `PATH`
4. Well-known directories (`/opt/homebrew/bin`, `/usr/local/bin`,
   `~/.local/bin`, `~/bin`, `~/.pegg/bin`) and directories declared in
   `~/.zshrc`, `~/.zprofile`, `~/.bash_profile`, and `~/.profile`
5. A downloaded release in the IDE config directory, verified against
   `checksums.txt`

The release tag defaults to the plugin version (`match-extension`). Set it to
`latest` or a specific tag in settings.

## Settings

| Setting | Default | Description |
|---|---|---|
| Executable path | `""` | Explicit path to the `pegg` binary |
| Auto download | on | Download a matching release when no binary is found |
| Allow workspace binary | on | Allow `<project>/bin/pegg` in trusted projects |
| Release tag | `match-extension` | `match-extension`, `latest`, or a tag such as `v0.2.0` |
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

Binary and environment changes prompt for a server restart.

## Commands and keybindings

| Action | Shortcut |
|---|---|
| Open Pegg Chat | `Ctrl/Cmd+Alt+P` |
| New Session | `Ctrl/Cmd+Alt+N` |
| Select Mode | `Ctrl/Cmd+Alt+M` |
| AI Actions… | `Ctrl/Cmd+Alt+A` |
| Explain Selection | `Ctrl/Cmd+Alt+E` |
| Ask About Selection… | `Ctrl/Cmd+Alt+Q` |
| Edit Selection… | `Ctrl/Cmd+Alt+X` |

All commands are in **Tools → Pegg** and the Search Everywhere action list.
Keybindings can be changed in **Settings → Keymap** by searching for `Pegg`.

## Architecture

```
Pegg tool window
├── Pegg tab    → JCEF → shared React chat UI (same source as the VS Code extension)
└── Sessions tab → native JBList
        │
        ▼ same JSON message protocol as the VS Code webview
PeggService → ChatController → AcpClient → pegg serve --acp --stdio
```

The webview build consumes `pkg/chat-ui` directly and adds a JCEF bridge in
`webview/bootstrap.ts`; the chat UI is therefore a single shared source of truth.
The Kotlin controller/state is a direct port of the VS Code extension's
controller/state and produces the same `ChatState` wire format.

### Tests

```bash
cd pkg/intellij
./gradlew test
```

The ACP integration test uses `../../bin/pegg` or `PEGG_TEST_BINARY`.

## Known limitations

- **JCEF is required.** If the IDE runtime has JCEF disabled, the chat tab shows
  an actionable message. Enable JCEF from **Help → Find Action → Choose Boot
  Java Runtime** and restart.
- **2025.3.1+ only.** JCEF became a separate plugin dependency in 2026.2, and
  older platforms do not expose the stable `com.intellij.modules.jcef` alias.
  Supporting older IDEs requires a legacy build.
- **Android Studio** must be on a 2025.3.1+ platform build.
- **Diagnostic quick fixes.** IntelliJ does not expose an API for third-party
  plugins to inject quick fixes into arbitrary existing inspections. Pegg adds
  its own intentions and editor actions instead.
- **Chat participant.** IntelliJ has no equivalent of VS Code's `@pegg` chat
  participant; use the Pegg tool window and editor actions.
- **Remote development (Gateway).** JCEF rendering in JetBrains Client should be
  verified for your IDE version.

## Troubleshooting

### No binary is found

Run **Tools → Pegg → Install or Update Binary**, or set the executable path in
settings. **Diagnose Binary** lists every probed location and the reason a
candidate was rejected.

### Binary is old

Update the CLI with **Install or Update Binary**, or point the executable path
at a newer build. Older binaries do not advertise interactive permissions or
rich tool events.

### The server stops

Open **Tools → Pegg → Show Logs**, then **Restart Server**. The current session
is restored on the next run.

### JCEF is disabled

The chat tab shows a fallback message. Enable JCEF in the IDE runtime and
restart the IDE.
