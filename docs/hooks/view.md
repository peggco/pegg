---
icon: lucide/layout-dashboard
---

# View hooks

View hooks let a plugin paint into the terminal interface itself: the status bar,
header, footer hint, chat transcript, prompt, menus, key bindings, and settings.
Payloads are plain JSON so a plugin never imports TUI packages.

The TUI collects fragments asynchronously — at most once every 500 ms, with a 150 ms
budget per hook — and caches them. Drawing only paints the cache, so a slow plugin
cannot freeze the interface.

## Building blocks

```go
type ViewSpan struct {
    Text  string `json:"text"`
    Style string `json:"style,omitempty"` // default|muted|hint|accent|error|warning|success|bold
}

type ViewLine struct {
    Text  string     `json:"text,omitempty"`
    Spans []ViewSpan `json:"spans,omitempty"`
}

type ViewFragment struct {
    ID       string     `json:"id,omitempty"`
    Title    string     `json:"title,omitempty"`
    Badge    string     `json:"badge,omitempty"`
    Style    string     `json:"style,omitempty"`
    Lines    []ViewLine `json:"lines,omitempty"`
    Priority int        `json:"priority,omitempty"`
}
```

A line either uses `text` or a list of colored `spans`. Fragments are sorted by
`priority` (lower first) where several are shown.

## Status bar

### `tui.status.left` / `tui.status.right`

**Filter** — re-render one side of the bottom status bar.

Payload carries the current state (`mode`, `running`, `provider`, `model`,
`reasoning_effort`, `cost`, `total_tokens`, `max_tokens`) and `current`, the built-in
spans for that side.

```json
{
  "mode": "build",
  "running": true,
  "provider": "openai",
  "model": "gpt-5",
  "current": [{ "text": "build" }, { "text": "gpt-5" }],
  "spans": [{ "text": "my-plugin", "style": "accent" }],
  "replace": false
}
```

Return `spans` to append them, or `replace: true` to replace the built-in content
entirely. `tui.status.right` sits between the built-in right segment and the edge.

## Header and footer

### `tui.header`

**Filter** — adds rows above the chat/logo area. `title` becomes an accent line,
`lines` follow it. The chat area shrinks by the number of rows.

```json
{
  "mode": "auto",
  "session_id": "s-1",
  "title": "My Plugin",
  "lines": [{ "spans": [{ "text": "connected", "style": "success" }] }]
}
```

### `tui.footer`

**Filter** — replaces or extends the centered hint above the input.

```json
{ "hint": "Press F2 to open My Plugin", "lines": [] }
```

`hint` replaces the built-in random tip when the input is empty; `lines` render just
above it.

## Chat transcript

### `tui.chat.items`

**Filter** — appends plugin cards to the transcript. Cards are notice items with a
stable ID (`plugin:<id>`), replaced whenever the hook result changes.

```json
{
  "items": [
    {
      "id": "ci-status",
      "title": "CI",
      "badge": "passing",
      "priority": 10,
      "lines": [{ "spans": [{ "text": "3 checks green", "style": "success" }] }]
    }
  ]
}
```

### `tui.chat.empty`

**Filter** — cards shown under the logo while the transcript is empty. Same payload
as `tui.chat.items`.

### `tui.message.decorate`

**Filter** — decorates an assistant message as it is rendered.

```json
{
  "agent_id": "a-1",
  "agent_name": "developer",
  "role": "assistant",
  "content": "the answer",
  "prefix": "[dev] ",
  "suffix": ""
}
```

`prefix`/`suffix` are prepended/appended to the rendered text; the stored session
message is unchanged. This hook runs per assistant message, not per token.

## Prompt and menus

### `tui.prompt`

**Filter** — sets the input placeholder.

```json
{ "placeholder": "Ask My Plugin…", "prompt": "" }
```

### `tui.menu.items`

**Filter** — appends entries to a context menu.

```json
{
  "menu": "",
  "items": [
    { "id": "my-plugin:refresh", "title": "Refresh My Plugin", "badge": "" }
  ]
}
```

Selecting an item dispatches `tui.command` with the item `id`.

## Key bindings

### `tui.keybindings`

**Filter** — registers shortcuts. Modifier-based bindings are required; single runes
without a modifier are ignored so typing is never swallowed.

```json
{
  "bindings": [
    { "id": "my-plugin:toggle", "key": "ctrl+k", "label": "My Plugin" },
    { "id": "my-plugin:palette", "key": "alt+enter", "label": "Palette" }
  ]
}
```

Supported forms: `ctrl+<letter>`, `alt+<key>`, `shift+<key>`, `f1`–`f12`, and named
keys (`tab`, `enter`, `esc`, `space`, `up`, `down`, `left`, `right`, `home`, `end`,
`pgup`, `pgdn`, `backspace`, `delete`, `insert`). Labels appear in the footer hint.

### `tui.command`

**Action** — fired when a plugin binding or menu item is chosen.

```json
{ "id": "my-plugin:toggle", "key": "ctrl+k" }
```

## Settings

### `tui.settings.sections`

**Filter** — adds editable fields to the settings dialog, grouped by the built-in tab
named in `tab`: `general`, `session`, `mcp`, `skills`, `rules`, `plugins`,
`permissions`, `memory`, `system`. Unknown or empty tabs go to an **Extensions** tab.

```json
{
  "tab": "general",
  "sections": [
    {
      "tab": "general",
      "title": "My Plugin",
      "description": "Connection settings",
      "fields": [
        { "ID": "token", "Label": "API token", "Value": "", "Secret": true },
        { "ID": "mode", "Label": "Mode", "Value": "fast", "Options": ["fast", "safe"] }
      ]
    }
  ]
}
```

Fields use the existing `TUIField` shape (`ID`, `Label`, `Placeholder`, `Value`,
`Secret`, `Required`, `Options`, `ShowWhen`). Each plugin tab is inserted right after
its target tab in the tab bar and renders with the standard plugin-page editor.

### `tui.settings.values`

**Filter** — validates or rewrites the values just before saving.

```json
{ "tab": "general", "values": { "token": "abc", "mode": "fast" } }
```

### `tui.settings.save`

**Action** — receives the final values when the user saves a plugin settings tab.

```json
{ "tab": "general", "values": { "token": "abc", "mode": "fast" } }
```

After the action runs, the TUI re-runs `tui.settings.sections` so the displayed values
reflect what the plugin persisted.

## Desktop

The desktop app uses a single aggregate hook instead of the TUI's per-area hooks:
[`desktop.view`](desktop.md#desktopview) returns the header, footer, status spans,
chat cards, empty-state cards, composer placeholder, palette entries, key bindings,
and settings pages in one payload. The bridge caches it and the React frontend paints
fixed slots. `desktop.command` and `desktop.settings.save` are the companion actions.

!!! note
    TUI view hooks are only dispatched by the TUI. The desktop app uses `desktop.view`
    instead, and headless runs call neither. Use `hooks.disabled` to silence individual
    hooks.
