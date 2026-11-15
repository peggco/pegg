---
icon: lucide/play
---

# Core hooks

Lifecycle, CLI, configuration, and notification hooks. They fire once per process
or at the transport edges, so they are cheap and safe to enable.

## `core.config.loaded`

**Action** — after configuration is loaded and plugins are ready.

```json
{
  "version": "0.2.4",
  "theme": "dark",
  "language": "en",
  "providers": ["openai", "anthropic"]
}
```

Secrets are never included. Use it to adapt a plugin to the host's provider list,
theme, or language without reading `~/.pegg/config.json` yourself.

## `core.startup`

**Action** — once the modules are wired, immediately before the CLI command runs.

```json
{
  "version": "0.2.4",
  "workspace": "/home/me/project"
}
```

## `core.shutdown`

**Action** — when the host exits, including on error paths.

```json
{ "reason": "exit" }
```

Use it to flush plugin-owned state. The hook has the normal 5 second budget; keep
shutdown handlers fast.

## `core.command`

**Filter** — rewrites the argv before the CLI executes it.

```json
{ "args": ["run", "--model", "gpt-5"] }
```

Return `{"args": [...]}` with the arguments you want. This runs after plugin
loading, so a plugin can change the command line of the current process. Prefer
`llm.request` for model selection; reserve this hook for genuinely CLI-level
behavior.

## `core.notification`

**Filter** — rewrites OS notifications before they are delivered.

```json
{
  "title": "Input required: developer",
  "body": "Allow bash?",
  "level": "info"
}
```

Return the same shape. The hook fires only when the host is unfocused and the
notification driver is enabled.

## Config

All core hooks honor [`hooks.enabled`, `hooks.disabled`, and
`hooks.timeout_seconds`](../pegg/config.md#hooks).
