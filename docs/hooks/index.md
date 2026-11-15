---
icon: lucide/webhook
---

# Hooks

Pegg exposes its subsystems through named **filter** and **action** hooks. Plugins
declare the hooks they handle, and the host calls them at the corresponding points
in the agent loop, LLM layer, sessions, tools, TUI, and desktop app. Plugins can
also dispatch hooks themselves with `pkg/plugin/hooks`.

This section is the catalog. The mechanics are documented in
[Plugin Hooks](../plugin/hooks.md).

## Filter vs action

| Kind | Behavior | Result |
|---|---|---|
| `filter` | Handlers run in priority order; each receives the previous value | Rewritten value |
| `action` | Handlers run in priority order; each receives the same value | Zero or more outputs |

Filters are how plugins rewrite requests, messages, titles, key events, and themes.
Actions are how plugins observe usage, errors, lifecycle events, and session changes.

## Handling a hook

```go
type Plugin struct{ peggplugin.Base }

func (p *Plugin) Hooks() ([]peggplugin.HookSpec, error) {
    return []peggplugin.HookSpec{
        {Name: "llm.request", Kind: peggplugin.HookFilter, Priority: 10},
        {Name: "session.message.append", Kind: peggplugin.HookFilter, Priority: 10},
    }, nil
}

func (p *Plugin) ExecuteHook(req peggplugin.HookRequest) (*peggplugin.HookResult, error) {
    switch req.Name {
    case "llm.request":
        var payload struct {
            AgentID string      `json:"agent_id"`
            Request llmPayload  `json:"request"`
        }
        if err := json.Unmarshal(req.Value, &payload); err != nil {
            return nil, err
        }
        payload.Request.Model = "my-model"
        out, _ := json.Marshal(payload)
        return &peggplugin.HookResult{Value: out, Handled: true}, nil
    }
    return &peggplugin.HookResult{}, nil
}
```

The plugin never imports host packages for payloads: every payload is plain JSON.
Use your own structs that mirror the documented fields. Unknown fields are ignored,
and fields you leave out of a filter result keep their zero value, so return the
complete object.

## Dispatching a hook

```go
client := &hooks.Client{Timeout: 10 * time.Second}
_, err := client.FilterInto(ctx, "session.title", map[string]any{
    "session_id": id,
    "title":      "Renamed",
}, &title)
```

The host skips the dispatching plugin's own handlers for that call and caps nested
dispatch at 4 levels per hook, so a plugin cannot re-enter itself.

## Configuration

```json
{
  "hooks": {
    "enabled": true,
    "disabled": ["llm.stream.chunk", "tui.key"],
    "timeout_seconds": 5
  }
}
```

| Key | Default | Meaning |
|---|---|---|
| `enabled` | `true` | Master switch for all hooks |
| `disabled` | `[]` | Hook names to skip |
| `timeout_seconds` | `5` | Default per-hook budget (`1`–`60`) |

High-frequency hooks use shorter fixed budgets: stream chunks and key events 100 ms,
message submission 250 ms. When no plugin handles a hook the bus returns before doing
any JSON work, so unused hooks cost a map lookup.

Hook errors never abort the operation. A failing filter keeps the last good value;
the error is logged by the subsystem and the run continues.

## Catalog

| Hook | Kind | Subsystem | Page |
|---|---|---|---|
| `core.config.loaded` | action | Lifecycle | [Core](core.md) |
| `core.startup` | action | Lifecycle | [Core](core.md) |
| `core.shutdown` | action | Lifecycle | [Core](core.md) |
| `core.command` | filter | CLI | [Core](core.md) |
| `core.notification` | filter | Notifications | [Core](core.md) |
| `llm.request` | filter | LLM | [LLM](llm.md) |
| `llm.response` | filter | LLM | [LLM](llm.md) |
| `llm.stream.chunk` | filter | LLM | [LLM](llm.md) |
| `llm.model.resolved` | action | LLM | [LLM](llm.md) |
| `llm.usage` | action | LLM | [LLM](llm.md) |
| `llm.error` | action | LLM | [LLM](llm.md) |
| `session.create` | filter | Sessions | [Sessions](session.md) |
| `session.open` | action | Sessions | [Sessions](session.md) |
| `session.message.append` | filter | Sessions | [Sessions](session.md) |
| `session.compact` | filter | Sessions | [Sessions](session.md) |
| `session.fork` | action | Sessions | [Sessions](session.md) |
| `session.delete` | action | Sessions | [Sessions](session.md) |
| `session.title` | filter | Sessions | [Sessions](session.md) |
| `tool.before` | filter | Tools | [Tools](tools.md) |
| `tool.after` | filter | Tools | [Tools](tools.md) |
| `tool.permission` | filter | Tools | [Tools](tools.md) |
| `tui.submit` | filter | TUI | [TUI](tui.md) |
| `tui.key` | filter | TUI | [TUI](tui.md) |
| `tui.ready` | action | TUI | [TUI](tui.md) |
| `tui.quit` | action | TUI | [TUI](tui.md) |
| `tui.theme` | filter | TUI | [TUI](tui.md) |
| `tui.notice` | filter | TUI | [TUI](tui.md) |
| `tui.message` | filter | TUI | [TUI](tui.md) |
| `tui.status.left` | filter | TUI view | [View](view.md#status-bar) |
| `tui.status.right` | filter | TUI view | [View](view.md#status-bar) |
| `tui.header` | filter | TUI view | [View](view.md#header-and-footer) |
| `tui.footer` | filter | TUI view | [View](view.md#header-and-footer) |
| `tui.chat.items` | filter | TUI view | [View](view.md#chat-transcript) |
| `tui.chat.empty` | filter | TUI view | [View](view.md#chat-transcript) |
| `tui.message.decorate` | filter | TUI view | [View](view.md#chat-transcript) |
| `tui.prompt` | filter | TUI view | [View](view.md#prompt-and-menus) |
| `tui.menu.items` | filter | TUI view | [View](view.md#prompt-and-menus) |
| `tui.keybindings` | filter | TUI view | [View](view.md#key-bindings) |
| `tui.command` | action | TUI view | [View](view.md#key-bindings) |
| `tui.settings.sections` | filter | TUI view | [View](view.md#settings) |
| `tui.settings.values` | filter | TUI view | [View](view.md#settings) |
| `tui.settings.save` | action | TUI view | [View](view.md#settings) |
| `desktop.bootstrap` | filter | Desktop | [Desktop](desktop.md) |
| `desktop.emit` | filter | Desktop | [Desktop](desktop.md) |
| `desktop.theme` | filter | Desktop | [Desktop](desktop.md) |
| `desktop.notify` | filter | Desktop | [Desktop](desktop.md) |
| `desktop.chat.send` | filter | Desktop | [Desktop](desktop.md) |
| `desktop.session` | action | Desktop | [Desktop](desktop.md) |
| `desktop.view` | filter | Desktop | [Desktop](desktop.md#desktopview) |
| `desktop.command` | action | Desktop | [Desktop](desktop.md#desktopcommand) |
| `desktop.settings.save` | action | Desktop | [Desktop](desktop.md#desktopsettingssave) |
