---
icon: lucide/webhook
---

# Plugin hooks

Hooks let plugins cooperate without depending on each other: an integration plugin
handles a named hook, and another plugin dispatches it. Pegg is only the broker.

There are two directions:

| Direction | Who calls whom | API |
|---|---|---|
| Handle | Pegg host calls the plugin | `Hooks()` + `ExecuteHook()` |
| Dispatch | Plugin calls the host | `pkg/plugin/hooks` client |

The host also calls plugins at fixed points in the agent loop, LLM layer, sessions,
tools, TUI, and desktop app. See the [hook catalog](../hooks/index.md) for every
name, payload, and call site.

## Handling hooks

Declare the hooks you handle and implement `ExecuteHook`:

```go
import peggplugin "github.com/peggco/pegg/pkg/plugin"

func (p *Plugin) Hooks() ([]peggplugin.HookSpec, error) {
    return []peggplugin.HookSpec{{
        Name:        "kanban.providers",
        Kind:        peggplugin.HookFilter,
        Priority:    10,
        Description: "report GitHub providers",
    }}, nil
}

func (p *Plugin) ExecuteHook(req peggplugin.HookRequest) (*peggplugin.HookResult, error) {
    if req.Name != "kanban.providers" {
        return &peggplugin.HookResult{}, nil
    }
    value, _ := json.Marshal(providers)
    return &peggplugin.HookResult{Value: value, Handled: true}, nil
}
```

`HookRequest` carries `Name`, `Kind`, `Value` (JSON), `TimeoutMS`, and `Origin` (the
plugin that dispatched the hook, empty for host-originated calls).

`HookResult` fields:

| Field | Meaning |
|---|---|
| `Value` | Replacement value for a filter |
| `Handled` | Set to `false` to leave a filter value unchanged |
| `Outputs` | Action outputs, one per handler |
| `Errors` | Non-fatal handler errors |

Filters run in priority order and chain values. Actions run in priority order and
collect outputs; one failing handler does not stop the others.

## Dispatching hooks

Use the host dispatcher through `pkg/plugin/hooks`:

```go
import "github.com/peggco/pegg/pkg/plugin/hooks"

client := &hooks.Client{Timeout: 90 * time.Second}

var providers []ProviderInfo
_, err := client.FilterInto(ctx, "kanban.providers", []ProviderInfo{}, &providers)
if errors.Is(err, hooks.ErrNoHandler) {
    return nil // no integration plugin registered
}

outcome, err := client.Action(ctx, "kanban.card.succeeded", payload)
if err != nil {
    return err
}
for _, output := range outcome.Outputs {
    // decode each handler's output
}
```

Calls travel over the RPC connection Pegg already serves to the plugin, so no extra
`pegg` process is started and no hooks binary needs to be configured.

### Rules

- **Origin skipping.** When a plugin dispatches a hook, the host skips that plugin's own
  handlers for the call. This prevents a plugin from re-entering itself.
- **Recursion limit.** Nested dispatch of the same hook from the same origin is capped at
  4 levels; beyond that the call fails with a recursion error.
- **Timeouts.** `Client.Timeout` (default 30s) is sent as `TimeoutMS`; the host enforces
  it across all handlers. Context cancellation also stops waiting locally.
- **No handler.** `ErrNoHandler` means no plugin (other than the origin) handles the hook.
- **No host.** `ErrNoHostDispatcher` means the plugin is not running inside a Pegg host
  that serves hook dispatch.

## CLI

From a shell, inspect and apply hooks directly:

```bash
pegg hooks list [--json]
pegg hooks apply <name> --kind filter|action --value '{"..."}' [--json]
```

These run in the host process; they do not use a separate dispatcher daemon.
