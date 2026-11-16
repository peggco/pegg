---
icon: lucide/monitor
---

# Desktop hooks

The desktop app is a Go bridge between the React frontend and the ACP agent. Desktop
hooks fire in that bridge, so they cover window events, notifications, themes,
sessions, and the whole desktop view. They are compiled only with the `gui` build tag.

## `desktop.view`

**Filter** — returns the entire view a plugin wants to inject into the desktop UI. One
call per refresh carries the context and the previously rendered view; return the
complete `view` object. Fields you omit reset to empty, so start from the payload you
received.

```json
{
  "platform": "darwin",
  "workspace": "/home/me/project",
  "session_id": "s-1",
  "agent_id": "a-1",
  "agent_name": "developer",
  "mode": "build",
  "theme": "dark",
  "running": true,
  "messages": 12,
  "view": {
    "header": [
      { "spans": [{ "text": "My Plugin", "style": "accent" }] }
    ],
    "footer": [
      { "spans": [{ "text": "Connected", "style": "success" }] }
    ],
    "status_left": [{ "text": "my-plugin", "style": "muted" }],
    "status_right": [{ "text": "3 checks", "style": "success" }],
    "chat_items": [
      {
        "id": "ci",
        "title": "CI",
        "badge": "passing",
        "lines": [{ "spans": [{ "text": "3 checks green", "style": "success" }] }]
      }
    ],
    "empty_items": [
      { "id": "welcome", "title": "My Plugin", "lines": [{ "text": "Press Ctrl+K to start" }] }
    ],
    "placeholder": "Ask My Plugin…",
    "menu_items": [
      { "id": "my-plugin:refresh", "title": "Refresh My Plugin", "badge": "F5" }
    ],
    "key_bindings": [
      { "id": "my-plugin:toggle", "key": "ctrl+k", "label": "My Plugin" }
    ],
    "settings": [
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
}
```

### Where each field renders

| Field | Slot |
|---|---|
| `header` | Rows above the thread |
| `footer` | Rows above the composer hint |
| `status_left` / `status_right` | Thin status line between the thread and the composer |
| `replace_status` | Accepted for parity; the desktop has no built-in status text to replace |
| `chat_items` | Cards in the transcript; ephemeral, never written to the session |
| `empty_items` | Cards under the welcome screen when there are no messages |
| `placeholder` | Composer placeholder |
| `menu_items` | Extra entries in the command palette |
| `key_bindings` | Global shortcuts; selecting one dispatches `desktop.command` |
| `settings` | Settings → plugin pages group, one page per `tab` |

`ViewSpan.style` accepts `default`, `muted`, `hint`, `accent`, `error`, `warning`,
`success`, and `bold`. `ViewFragment` uses `title`, `badge`, `lines`, and `priority`
(lower first).

### Refresh and cost

- The bridge calls the hook with a **150 ms** budget and at most once per **250 ms**,
  plus a 1 s ticker. Refresh is single-flight, so a slow plugin cannot pile up calls.
- Results are cached and painted by the frontend; the bridge emits `desktop:view`
  with the rendered state (the frontend also calls `ViewState()` on mount).
- When no plugin handles `desktop.view` the ticker exits immediately and the view is
  cleared.

### Settings pages

Fields use the existing `TUIField` shape (`ID`, `Label`, `Placeholder`, `Value`,
`Secret`, `Required`, `Options`, `ShowWhen`). Sections are grouped by `tab` into
synthetic pages (`general`, `session`, `mcp`, `skills`, `rules`, `plugins`,
`permissions`, `memory`, `system`; unknown or empty → **Extensions**). Saving one
dispatches `desktop.settings.save` with the page tab and values, then re-runs
`desktop.view`.

## `desktop.command`

**Action** — fired when a plugin key binding or command-palette entry is chosen.

```json
{ "id": "my-plugin:toggle", "key": "ctrl+k" }
```

Key binding grammar: `ctrl+<letter>`, `alt+<key>`, `shift+<key>`, `f1`–`f12`, and named
keys (`tab`, `enter`, `esc`, `space`, `up`, `down`, `left`, `right`, `home`, `end`,
`pgup`, `pgdn`, `backspace`, `delete`, `insert`). `ctrl` also matches Cmd on macOS.
Unmodified single runes are ignored so typing is never swallowed.

## `desktop.settings.save`

**Action** — receives the values of a plugin settings page rendered from
`desktop.view.settings`.

```json
{ "tab": "general", "values": { "token": "abc", "mode": "fast" } }
```

## Other desktop hooks

### `desktop.bootstrap`

**Filter** — before bootstrap data is returned to the frontend.

```json
{
  "platform": "linux",
  "version": "0.2.4",
  "workspace": "/home/me/project",
  "language": "en"
}
```

Return `workspace` or `language` to change the initial view. `language` is empty when the config does not set one explicitly; the desktop then uses its own cached language or English.

### `desktop.emit`

**Filter** — before an event is sent to the webview.

```json
{ "name": "agent:event", "data": { "type": "token", "text": "hi" } }
```

### `desktop.theme`

**Filter** — before a theme change is persisted.

```json
{ "name": "midnight", "mode": "dark" }
```

### `desktop.notify`

**Filter** — before a native desktop notification.

```json
{ "title": "Pegg", "body": "Run finished" }
```

### `desktop.chat.send`

**Filter** — before a prompt is sent to the ACP agent.

```json
{ "session_id": "s-1", "text": "fix the bug" }
```

### `desktop.session`

**Action** — after the bridge creates or loads a session.

```json
{ "action": "create", "session_id": "s-1" }
```

!!! note
    Desktop hooks require the `gui` build tag and are not available in the CLI, TUI, or
    headless server. Declaring them in a plugin is safe: in other modes the host simply
    never dispatches them.
