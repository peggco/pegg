---
icon: lucide/terminal
---

# TUI hooks

TUI hooks let plugins participate in the terminal interface without owning a page.
They run in the TUI process, next to the built-in in-process hooks; the plugin hook
always runs second.

For painting the interface itself — status bar, header, footer, chat cards, prompt,
menus, key bindings, and settings fields — see [View hooks](view.md).

## `tui.submit`

**Filter** — when the user submits input, before it reaches the agent. Budget:
**250 ms**.

```json
{ "message": "fix the failing test" }
```

Return `message` to rewrite it — expand an alias, prepend context, or reject input by
returning an empty string.

## `tui.key`

**Filter** — for every key event. Budget: **100 ms**.

```json
{
  "key": 27,
  "rune": 0,
  "mod": 0,
  "consumed": false
}
```

`key` and `mod` are the tcell numeric values. Set `consumed: true` to stop the TUI
from handling the key — useful for plugin-defined shortcuts. Key and rune rewrites
are informational; only `consumed` changes host behavior.

## `tui.ready`

**Action** — after the TUI is built and before the event loop starts.

```json
{ "workspace": "" }
```

## `tui.quit`

**Action** — when the TUI is shutting down.

```json
{}
```

## `tui.theme`

**Filter** — after the theme cycle picks the next theme.

```json
{ "name": "dracula", "mode": "dark" }
```

Return a different `name` from the built-in theme registry to override the cycle.
The chosen theme is persisted to the config.

## `tui.notice`

**Filter** — before an error or notice is added to the transcript.

```json
{ "title": "error", "body": "provider rate limited", "level": "error" }
```

Return `body` to rewrite what the user sees.

## `tui.message`

**Filter** — before an assistant message is appended to the transcript.

```json
{
  "agent_id": "a-1",
  "agent_name": "developer",
  "role": "assistant",
  "content": "the answer"
}
```

Return `content` to decorate, redact, or shorten messages for display. The stored
session message is not affected.

## Example: plugin shortcut

```go
func (p *Plugin) ExecuteHook(req peggplugin.HookRequest) (*peggplugin.HookResult, error) {
    if req.Name != "tui.key" {
        return &peggplugin.HookResult{}, nil
    }
    var payload struct {
        Rune     int32 `json:"rune"`
        Mod      int   `json:"mod"`
        Consumed bool  `json:"consumed"`
    }
    if err := json.Unmarshal(req.Value, &payload); err != nil {
        return nil, err
    }
    if payload.Rune == 'k' && payload.Mod == 4 {
        payload.Consumed = true
    }
    out, _ := json.Marshal(payload)
    return &peggplugin.HookResult{Value: out, Handled: true}, nil
}
```

!!! note
    TUI hooks only exist in the terminal application. The desktop app has its own
    [desktop hooks](desktop.md), and headless runs have neither.
