<div align="center">

<img src="https://cdn.pegg.dev/logo/infinity.gif" alt="Pegg" width="420" />

# Pegg for JetBrains IDEs

### The same agent. In every JetBrains IDE.

Pegg is a provider-agnostic AI coding agent in a single Go binary. This plugin
brings the CLI, TUI, and VS Code engine into IntelliJ IDEA, PhpStorm, WebStorm,
PyCharm, CLion, Android Studio, and other IntelliJ Platform products.

[Download](https://github.com/peggco/pegg/releases) · [Docs](https://docs.pegg.dev/pegg/usage/intellij/) · [Website](https://pegg.dev) · [Discussions](https://github.com/peggco/pegg/discussions)

[![Latest release](https://img.shields.io/github/v/release/peggco/pegg?style=flat-square&sort=semver)](https://github.com/peggco/pegg/releases)
[![IntelliJ Platform](https://img.shields.io/badge/platform-2025.3.1%2B-black?style=flat-square)](https://docs.pegg.dev/pegg/usage/intellij/)
[![License: FSL-1.1-MIT](https://img.shields.io/badge/license-FSL--1.1--MIT-black?style=flat-square)](https://github.com/peggco/pegg/blob/master/LICENSE.md)

**Install from a release**

Download `pegg-intellij-<version>.zip`, then open
**Settings → Plugins → ⚙ → Install Plugin from Disk…** and restart the IDE.

</div>

```text
Pegg  ·  IntelliJ IDEA  ·  Claude Sonnet

› Add retry handling to the upload client

● orchestrator
  read   src/upload/Client.kt
  edit   src/upload/Client.kt   +31 −4
  bash   ./gradlew test
  ✓      BUILD SUCCESSFUL
```

> The plugin is an ACP client. It runs the real `pegg` binary on your machine,
> keeps provider keys out of the IDE, and never sends code to a hosted Pegg
> service.

---

## Features

- **Chat** — streaming markdown, syntax highlighting, reasoning, tool cards,
  edit diffs, plans, token usage, and USD cost.
- **Subagents** — subagent cards open an in-place transcript with Back.
- **Sessions** — a Sessions tab shared with the CLI and VS Code; resume, rename,
  delete, search, and refresh.
- **Composer** — `@` file mentions, `/` skills and commands, attachments,
  drag-and-drop, pasted images, voice input, and mode/model pickers.
- **Permissions and questions** — inline cards in chat, with modal fallbacks.
- **Editor actions** — explain, edit, ask, review, fix, security review,
  refactor, optimize, document, generate tests, fix diagnostics, and add to
  chat from menus, intentions, and Search Everywhere.
- **History actions** — copy, fork from here, and revert to here.
- **Binary management** — resolve an installed binary or download a
  checksum-verified release into the IDE config directory.
- **Status bar** — mode, model, token usage, and a shortcut to open chat.

## Requirements

| | |
|---|---|
| **IntelliJ Platform** | 2025.3.1 or newer |
| **Java runtime** | JBR 21 (bundled with current IDEs) |
| **JCEF** | Enabled (the default in JetBrains IDEs) |
| **Project** | Trusted — Pegg can edit files and run commands |
| **Provider** | At least one provider configured with `pegg login` |

The plugin depends only on the platform, language, and JCEF modules, so the same
artifact works across JetBrains products. Android Studio must be on a 2025.3.1+
platform build.

## Install

### From a release

Download `pegg-intellij-<version>.zip` from the
[releases page](https://github.com/peggco/pegg/releases), then open
**Settings → Plugins → ⚙ → Install Plugin from Disk…** and restart the IDE.

### Build from source

```bash
npm --prefix pkg/chat-ui ci
cd pkg/intellij
npm ci
./gradlew buildPlugin
```

The installable zip is written to `pkg/intellij/build/distributions/`.

Run a development IDE with:

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

The plugin starts `pegg serve --acp --stdio` lazily and shows a download
progress notification if no binary is available.

## Binary resolution

The plugin probes, in order:

1. **Settings → Tools → Pegg → Executable path**
2. `<project>/bin/pegg` (trusted projects, when allowed)
3. `pegg` on `PATH`
4. Well-known directories (`/opt/homebrew/bin`, `/usr/local/bin`,
   `~/.local/bin`, `~/bin`, `~/.pegg/bin`) and directories declared in
   `~/.zshrc`, `~/.zprofile`, `~/.bash_profile`, and `~/.profile`
5. A checksum-verified release downloaded into the IDE config directory

The release tag defaults to the plugin version (`match-extension`). Set it to
`latest` or a specific tag in settings. Run **Diagnose Binary** to see every
probed location and why a candidate was rejected.

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

All commands are in **Tools → Pegg** and Search Everywhere. Change keybindings
in **Settings → Keymap** by searching for `Pegg`.

## Settings

| Setting | Default | Description |
|---|---|---|
| Executable path | `""` | Explicit path to `pegg` |
| Auto download | on | Download a matching release when needed |
| Allow workspace binary | on | Allow `<project>/bin/pegg` in trusted projects |
| Release tag | `match-extension` | `match-extension`, `latest`, or a tag |
| Download base URL | GitHub releases | Release download base URL |
| Preferred provider / model | `""` | Defaults for new sessions |
| Default mode | `auto` | Auto, Plan, or Build |
| Include active file / selection | on | Editor context |
| Max attachment bytes | `262144` | Maximum text attachment size |
| Send on | `enter` | Enter or Ctrl/Cmd+Enter |
| Notify on complete | on | Notify when a run finishes while unfocused |
| Environment variables | `{}` | Extra variables for the ACP process |
| Log level | `info` | `error`, `warn`, `info`, or `debug` |

Binary and environment changes prompt for a server restart.

## Architecture

```text
Pegg tool window
├── Pegg tab     → JCEF → shared React chat UI (same source as VS Code)
└── Sessions tab → native JBList
        │
        ▼ same JSON message protocol as the VS Code webview
PeggService → ChatController → AcpClient → pegg serve --acp --stdio
```

The webview build consumes `pkg/chat-ui` directly and adds a JCEF bridge in
`webview/bootstrap.ts`. The Kotlin controller/state mirrors the VS Code
controller and produces the same `ChatState` wire format.

## Tests

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
- **Android Studio** must be on a 2025.3.1+ platform build.
- **Diagnostic quick fixes.** IntelliJ does not expose an API for third-party
  plugins to inject quick fixes into arbitrary inspections; Pegg adds its own
  intentions and editor actions instead.
- **Chat participant.** IntelliJ has no equivalent of VS Code's `@pegg` chat
  participant; use the Pegg tool window and editor actions.
- **Remote development (Gateway).** Verify JCEF rendering in JetBrains Client for
  your IDE version.

## Troubleshooting

- **No binary** — run **Tools → Pegg → Install or Update Binary**, or set the
  executable path. **Diagnose Binary** lists every probed location.
- **Binary is old** — update it with **Install or Update Binary**. Older binaries
  do not advertise interactive permissions or rich tool events.
- **Server stops** — open **Show Logs**, then **Restart Server**. The session is
  restored on the next run.
- **JCEF is disabled** — the chat tab shows a fallback message; enable JCEF in
  the IDE runtime and restart.

## License

FSL-1.1-MIT. See [LICENSE.md](https://github.com/peggco/pegg/blob/master/LICENSE.md).
