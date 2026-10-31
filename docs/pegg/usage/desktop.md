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

## Themes

The desktop theme list is generated from the terminal UI at runtime (`internal/tui/styles`). There is no duplicated palette in the frontend:

- Every TUI theme is available, including Catppuccin (4), Tokyo Night (3), Gruvbox (2), Rose Pine (3), Solarized (2), Kanagawa (2), Everforest (2), Nord, One Dark, Dracula, Monokai (3), and the base `dark` / `light`.
- The default is the TUI `dark` theme; `auto` follows the OS light/dark preference and picks the matching sibling theme.
- CSS variables are mapped one-to-one from the TUI `Theme` fields: background, surface, foreground, accent, borders, code colors, and all syntax token colors.
- Legacy `pegg-light` / `pegg-dark` preferences are migrated to the TUI `light` / `dark` themes.
- The interface scale can be adjusted in Settings → Appearance.

## Logo and app icon

Logo assets live in `pkg/desktop/assets`:

- `dark-logo.png` — white wordmark, used in the sidebar for dark themes
- `light-logo.png` — dark wordmark, used in the sidebar for light themes
- `appicon.png` — application icon, embedded into the binary and installed into the icon theme
- `pegg.desktop` — desktop entry

The sidebar swaps the wordmark automatically with the active TUI theme. `make desktop-install` hashes the app icon and writes the hash into the desktop entry (`Icon=pegg-<hash>`), then refreshes the GTK/KDE icon caches, so replacing `appicon.png` and reinstalling updates the titlebar and launcher icon without manually clearing caches.

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
- **Attachments** — use the `+`/paperclip buttons or drag files onto the window.
- **Voice** — the mic button uses the ACP STT configuration from the Pegg settings.
- **Modes** — Auto, Plan, Build.
- **Models** — the model menu is populated from `pegg/models`.
- **Stop** — the send button becomes a stop button while a run is active.

## Settings

The desktop settings page mirrors the TUI settings layout, backed by the same `~/.pegg/pegg.json` and project `.pegg/` files:

- **General** — provider/model, reasoning effort, language, and speech-to-text provider/model.
- **Appearance** — auto/light/dark mode, all TUI themes, and interface scale.
- **Session** — active session details, rename/delete/new chat, and context compaction (strategies, threshold, message and tool-output limits).
- **Permissions** — Auto/Manual/Judge/Recommended presets, per-tool rules, judge provider/model/threshold.
- **Memory** — enable/consolidate, gate, threshold, context budget, max results, entry stats.
- **MCP** — configured servers and a link to the config file.
- **Skills** — global/project skills with reveal actions.
- **Rules** — global/project Markdown rules with a preview.
- **Plugins** — installed plugins with per-plugin enable/disable.
- **System** — desktop/agent versions, binary resolution and updates, runtime paths, and logs.

Theme and model changes apply immediately; section edits have an explicit save action so presets can be reviewed before writing.

The desktop interface supports English (default) and Turkish. Choose the language in Settings → General; the same `language` value is stored in `~/.pegg/pegg.json` and shared with the TUI.

## Tool cards

Tool output follows the terminal UI's hierarchy instead of dumping everything:

- Reads, listings, searches, and web tools render as compact one-line cards; their output is hidden unless the tool failed, in which case it expands from the error action.
- Bash renders as a terminal with the command, output, and a status indicator. Long output is truncated with a show-all control.
- Writes render syntax-highlighted content, truncated for large files with an expand control.
- Edits render a GitHub-style line-numbered diff with hunk headers, additions/deletions, and a show-all control for large diffs.

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
│   ├── themes/              # TUI palette → CSS token DTOs
│   └── workspace/           # workspace validation and file reads
└── frontend/                # React 19 + Vite design system and UI
```

Streaming updates are coalesced in Go (16 ms) and delivered to the UI as one `acp:update` event per frame, which keeps token streaming smooth without React state churn.

## Troubleshooting

- **"Could not load sessions"** — check ~/.config/pegg/desktop.log. Session usage costs are decoded as numbers; older builds that expected an object fail here.
- **No models** — configure a provider with `pegg login` or the Pegg config, then reopen the model menu.
- **Blank window on Linux** — confirm WebKitGTK 4.1 is installed and rebuild with `-tags webkit2_41`.
- **KDE window stays on top** — window rules from an earlier session can persist. Remove the rule for `pegg-desktop` in System Settings → Window Management → Window Rules; the app itself never sets always-on-top.
