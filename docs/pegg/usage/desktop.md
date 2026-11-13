---
icon: lucide/monitor
---

# Pegg Desktop

A cross-platform desktop client for Pegg built with Wails v2 (Go + the system webview) and a React 19 + TypeScript frontend. It talks to the agent over the same **ACP stdio** protocol used by the VS Code and IntelliJ extensions (`pegg serve --acp --stdio`), so sessions, permissions, tools, models, skills, subagents, and STT are shared with the CLI.

The desktop app is intentionally separate from the agent binary. It discovers an installed `pegg`, or downloads a checksum-verified release into `~/.pegg/bin`.

## Requirements

- Go 1.26+
- Node.js 20+
- The [Wails v2 CLI](https://wails.io): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Linux: GTK 3 and WebKitGTK 4.1 development headers
  - Arch/CachyOS: `gtk3 webkit2gtk-4.1`
  - Debian/Ubuntu: `libgtk-3-dev libwebkit2gtk-4.1-dev`
- macOS: Xcode command line tools
- Windows: WebView2 runtime (installed by default on current Windows)

## Build and run

```bash
make desktop-deps       # npm install in pkg/desktop/frontend
make desktop-dev        # Wails dev server + hot reload
make desktop-build      # production build
make desktop-test       # frontend typecheck/tests + Go tests with the gui tag
```

The production binary is written to `pkg/desktop/build/bin/pegg-desktop`.

On Linux the build uses `-tags "gui webkit2_41"` because Wails v2 defaults to WebKitGTK 4.0. The `gui` tag keeps the Wails entrypoint out of the normal `go test ./...` / `go vet ./...` runs. macOS and Windows use `-tags gui`.

## Appearance

The desktop ships exactly two palettes: **Light** and **Dark**.

- Switch light/dark from the sidebar footer (sun/moon row). This is the only quick switcher.
- Settings → Appearance → **Color mode** keeps Auto / Light / Dark; Auto follows the OS light/dark preference.
- Removed palette names and legacy `pegg-light` / `pegg-dark` values are normalized to Light or Dark at load time.
- Both palettes map the TUI `Theme` fields (background, surface, foreground, accent, borders, code colors, and syntax tokens) to CSS variables.
- The interface scale is in Settings → Appearance.

The terminal UI keeps its full theme list; only the desktop is limited to Light/Dark.

## Logo and app icon

Logo assets live in `pkg/desktop/assets`:

- `dark-logo.png` — white wordmark, used in the sidebar for dark themes
- `light-logo.png` — dark wordmark, used in the sidebar for light themes
- `appicon.png` — application icon, embedded into the binary and installed into the icon theme
- `pegg.desktop` — desktop entry

The sidebar swaps the wordmark automatically with the active TUI theme. `make desktop-install` hashes the app icon and writes the hash into the desktop entry (`Icon=pegg-<hash>`), then refreshes the GTK/KDE icon caches, so replacing `appicon.png` and reinstalling updates the titlebar and launcher icon without manually clearing caches.

## Sidebar

Drag the sidebar's right edge to resize it between 200 and 520 px; dragging it below about 140 px collapses it. Double-click the handle to reset the width to 272 px. The width is saved in the desktop preferences and restored on the next launch. When the sidebar is collapsed, a slim rail on the far left reopens it at the saved width; the titlebar toggle and the **Toggle sidebar** command palette action also work.

Settings → Appearance also controls the shell:

- **Sidebar** — **Sidebar** (flat, full height), **Inset** (padded card), or **Floating** (padded card with a shadow).
- **Layout** — **Default**, **Compact** (tighter sidebar and chat spacing), or **Full layout** (wider chat content and composer).
- **Direction** — **Left to Right** or **Right to Left**. The shell mirrors, while code blocks and terminal output stay left-to-right.

### Chat history

- **Compact** (default) groups consecutive reasoning and tool calls behind a single activity summary such as `Ran 9 commands · read 3 files`, with a timestamp. Expanding the summary shows one-line reasoning/tool rows; expanding a row opens its full tool card (terminal output, diff, written code, or command output).
- **Detailed** renders every reasoning block and tool card inline, matching the previous behavior.

Assistant replies and user messages stay visible in both modes. Task and subagent cards remain standalone and full. Switching modes applies immediately and the choice is stored with the desktop preferences.

Opening a session restores the last 200 messages. Scrolling to the top loads older messages in batches of 200. The viewport stays anchored on the message you are reading — loading a page never jumps to the bottom, and live replies only auto-follow while you are already at the bottom.

## Agent binary

Resolution order:

1. `pegg.binaryPath` saved from Settings → Agent
2. `<workspace>/bin/pegg`
3. `$PATH`
4. `~/.pegg/bin/pegg`
5. Homebrew / `/usr/local/bin` / `~/.local/bin`

If nothing is found, Settings → Agent and the first-run screen offer **Download Pegg**. Downloads go to `~/.pegg/bin` and are verified against the release `checksums.txt` with SHA-256.

## Composer

- **`@` file mentions** — type `@` and a filename to search the workspace. Selecting a file inserts its path and attaches it to the message so the agent receives the content.
- **`/` commands** — type `/` to open the command menu. Server-provided commands are merged with built-in commands (`init`, `review`, `test`, `explain`, `fix`, `plan`, `commit`).
- **Attachments** — use the `+`/paperclip buttons or drag files onto the window. Files of
  any type are accepted: they are copied into Pegg's attachment store and the agent
  opens them by path. Images, audio, and video are also sent inline when the active
  model supports them. Attachments show as chips under your message and reappear when
  you reopen a session.
- **Voice** — the mic button uses the ACP STT configuration from the Pegg settings.
- **Modes** — Auto, Plan, Build.
- **Models** — the model menu is populated from `_pegg/models`.
- **Stop** — the send button becomes a stop button while a run is active.

## Agent questions

When the agent calls `askuserquestion`, the composer is replaced by a question
panel so you can answer without leaving the chat:

- **One question at a time** with a tab chip per question. A chip shows a check
  once that question is answered; click a chip (or press `Alt`+`←`/`→`) to switch.
- **Text** questions use an inline textarea (`Enter` continues, `Shift`+`Enter`
  adds a newline). **Select**, **multiselect**, and **boolean** questions render
  option buttons.
- **Custom…** adds a free-form answer to every select/multiselect/boolean
  question. Selecting it opens an inline input; for multiselect the custom value
  is appended to the selected options and can be removed from the editor.
- Multi-question asks finish with a **Review** step that lists every answer;
  edit any row or submit them all at once. Required answers are validated
  client-side and the panel jumps to the first missing one.
- Only one ask is shown at a time; queued asks appear in order as you submit.
  `Esc` (or the close button) cancels, and the composer returns to its normal
  state after the answer is sent.

Answers are submitted as a single `answers` map keyed by question ID, matching
the terminal UI and the ACP `_pegg/ask` protocol.

## Settings

The desktop settings page mirrors the TUI settings layout, backed by the same `~/.pegg/pegg.json` and project `.pegg/` files:

- **General** — provider/model, reasoning effort, language, and speech-to-text provider/model.
- **Appearance** — auto/light/dark color mode, sidebar mode, layout density, direction, chat history mode, and interface scale.
- **Session** — active session details, rename/delete/new chat, and context compaction (strategies, threshold, message and tool-output limits).
- **Permissions** — Auto/Manual/Judge/Recommended presets, per-tool rules, judge provider/model/threshold.
- **Memory** — enable/consolidate, gate, threshold, context budget, max results, entry stats.
- **MCP** — configured stdio, SSE, and Streamable HTTP servers with their transport labels, plus a link to the config file.
- **Skills** — global, project, and plugin-provided skills with reveal actions; the same list feeds the composer `/` skill menu. Plugin skills appear after the agent has loaded the plugin, so restart the desktop once after installing a plugin.
- **Rules** — global/project Markdown rules with a preview.
- **Plugins** — installed plugins with per-plugin enable/disable.
- **System** — desktop/agent versions, binary resolution and updates, runtime paths, and logs.

Theme and model changes apply immediately; section edits have an explicit save action so presets can be reviewed before writing.

The desktop interface supports English (default) and Turkish. Choose the language in Settings → General; the same `language` value is stored in `~/.pegg/pegg.json` and shared with the TUI.

## Tool cards

Tool output follows the terminal UI's hierarchy instead of dumping everything:

- Reads, listings, searches, and web tools render as compact one-line cards; their output is hidden unless the tool failed, in which case it expands from the error action.
- Bash renders as a terminal with the command, live output while the command
  runs, and a status indicator. Long output is truncated with a show-all control.
- Writes render syntax-highlighted content, truncated for large files with an expand control.
- Edits render a GitHub-style line-numbered diff with hunk headers, additions/deletions, and a show-all control for large diffs.
- `task` calls render as a single **TASK** card: name, subagent type, prompt, `background` badge, live activity, response preview, usage/duration, and an **Open history** action while running. The raw `task` tool row and the duplicate subagent card are no longer shown for task-launched subagents; fork subagents keep the standalone subagent card. Background continuations stream into the same thread when the subagent finishes.
- Todo/plan writes render as an inline plan card at the latest `todowrite` in each message, replacing the tool row. Older updates in the same message collapse into that last card; a new todo list in a later message appears as its own card. The plan no longer sticks to the bottom of the chat or follows you into new messages.

## Desktop updates

Pegg Desktop checks for a newer release on startup and covers both binaries:

- the desktop app (`pegg-desktop`), verified against
  `pegg-desktop_checksums.txt`
- the agent (`pegg`), verified against the release `checksums.txt`

When both have a newer version, they are downloaded and staged together; a
single **Restart to update** banner lists both versions and applies them in one
go. If only one has an update, only that one is offered. The staged update is
applied only after you confirm and the app relaunches.

- **Linux/Windows** — automatic for user-writable installs (e.g.
  `~/.local/bin`, a tarball/zip extracted somewhere you own). If the install
  directory is not writable (deb/rpm/AUR/Homebrew, `/usr/bin`,
  `/Applications`), the app does not overwrite it; update through the package
  manager or the download page.
- **Agent binary** — updated together with the desktop when it is the
  desktop-managed/writable binary. A system-installed agent is reported in the
  same banner but must be updated through its package manager.
- **macOS** — desktop check/notify only in this release; use the download page
  until signed/notarized bundle updates are available. The agent can still be
  updated automatically if it is writable.
- **Staged updates** — closing and reopening the app keeps the staged update
  and shows the Restart banner again; it is never applied silently.
- **Manual** — Settings → System has **Check for updates**, download progress,
  current/latest versions for both binaries, and **Restart to update**.

## Releases

`.github/workflows/release.yml` builds the desktop app on native runners for:

- macOS Apple Silicon and Intel
- Windows x64 and ARM64
- Linux x64 (`.tar.gz`, `.deb`, `.rpm`)

Artifacts are attached to the GitHub release. Optional publishing steps are enabled by repository secrets:

- `HOMEBREW_TAP_TOKEN` — pushes the generated cask to `peggco/homebrew-tap`
- `AUR_SSH_PRIVATE_KEY` — publishes `packaging/aur/PKGBUILD` to the AUR
- `CLOUDSMITH_API_KEY` — uploads `.deb`/`.rpm` packages to Cloudsmith for apt/dnf consumption

The desktop binary receives the release version through `-X github.com/peggco/pegg/internal/core/config.AppVersion=...`, so the GUI and agent versions stay in sync.

## Window behavior

- On Linux the app uses native window decorations by default. KDE/KWin and several GTK decoration modules re-add server-side decorations even for frameless GtkWindows, so native chrome is the reliable option.
- Set `PEGG_DESKTOP_FRAMELESS=1` to opt into a custom frameless chrome (the app then renders its own titlebar).
- On macOS the app uses a hidden-inset title bar with traffic lights.
- On Windows the app uses a frameless window with custom controls.
- The window is never set always-on-top (`AlwaysOnTop` is explicitly `false`).

## Architecture

```
pkg/desktop/
├── main.go                  # wails.Run, embedded frontend + icon, per-OS options
├── app.go                   # bindings: bootstrap, binary, workspace, themes, window
├── sessions.go              # ACP requests: sessions, prompts, models, permissions
├── files.go                 # @ mention workspace file index
├── platform_{linux,darwin,windows}.go
├── internal/
│   ├── acpclient/           # stdio JSON-RPC client, batched updates
│   ├── binary/              # discovery, download, checksum verification
│   ├── prefs/               # ~/.config/pegg/desktop.json
│   ├── themes/              # Light/Dark palette → CSS token DTOs
│   └── workspace/           # workspace validation and file reads
└── frontend/                # React 19 + Vite design system and UI
```

Streaming updates are coalesced in Go (16 ms) and delivered to the UI as one `acp:update` event per frame, which keeps token streaming smooth without React state churn.

## Troubleshooting

- **"Could not load sessions"** — check ~/.config/pegg/desktop.log. Session usage costs are decoded as numbers; older builds that expected an object fail here.
- **No models** — configure a provider with `pegg login` or the Pegg config, then reopen the model menu.
- **Blank window on Linux** — confirm WebKitGTK 4.1 is installed and rebuild with `-tags webkit2_41`.
- **KDE window stays on top** — window rules from an earlier session can persist. Remove the rule for `pegg-desktop` in System Settings → Window Management → Window Rules; the app itself never sets always-on-top.
