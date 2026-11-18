---
icon: lucide/bell-ring
---

# Events & Errors

## Events

The engine exposes the internal event bus. Subscribe with a topic and a handler
whose single parameter matches the payload type (dispatched by reflection), then
`defer` the returned unsubscribe function.

```go
unsub, err := eng.OnAgentFinished(func(ev sdk.AgentFinished) {
	fmt.Println("finished:", ev.AgentName, ev.Output)
})
if err != nil {
	return err
}
defer unsub()
```

### Typed helpers

| Method | Payload |
|---|---|
| `OnAgentToken(fn func(AgentToken))` | Streamed token (`Content`, `Reasoning`) |
| `OnAgentToolCall(fn func(AgentToolCall))` | A tool call about to run (`Call`) |
| `OnAgentToolResult(fn func(AgentToolResult))` | A tool result (`Output`, `Err`) |
| `OnAgentFinished(fn func(AgentFinished))` | Successful run completion |
| `OnAgentError(fn func(AgentError))` | Any run failure |

### Generic subscription

```go
unsub, err := eng.Subscribe(sdk.TopicAgentToken, func(ev sdk.AgentToken) { ... })
unsub, err = eng.SubscribeOnce(sdk.TopicSessionCreated, func(ev sdk.SessionCreated) { ... })
```

Available topics:

- **Agent**: `TopicAgentStarted`, `TopicAgentInput`, `TopicAgentMessage`,
  `TopicAgentToken`, `TopicAgentToolCall`, `TopicAgentToolResult`,
  `TopicAgentUsage`, `TopicAgentFinished`, `TopicAgentError`, `TopicAgentAsk`,
  `TopicAgentAskAnswer`.
- **Session**: `TopicSessionCreated`, `TopicSessionDeleted`, `TopicSessionUpdated`,
  `TopicSessionMessageAdded`, `TopicSessionForked`, `TopicSessionReverted`,
  `TopicSessionRestored`, `TopicSessionCurrentChanged`, `TopicSessionResume`,
  `TopicSessionAttached`.
- **Change journal**: `TopicChangeJournal` fires for every revert and undo with
  a `ChangeJournalResult`; `TopicChangeJournalReverted` and
  `TopicChangeJournalRestored` are the action-specific variants. The result
  lists per-file `restored`, `deleted`, `conflict`, and `not_tracked` statuses.
- **Engine**: `app.mounted` is published at `Open` with the resolved config.

`ChatStream` already maps the same events into `ChatEvent`s, so prefer it when you
only need run telemetry. Use the bus for cross-cutting concerns (logging, metrics,
custom middlewares).

## Errors

### Engine errors

| Sentinel | When |
|---|---|
| `sdk.ErrAlreadyOpen` | A second `Open` call in the same process |
| `sdk.ErrClosed` | Any method after `Close` |
| `sdk.ErrNotConfigured` | `Chat`/`Complete` with no configured providers |
| `sdk.ErrNoModels` | Provider has no models for the requested model |
| `sdk.ErrNoSession` | Session not found (`GetSession`, `Chat` with a bad `SessionID`) |
| `sdk.ErrModelTimeout` | Timed out selecting a model |

### Aliased errors

These are the same values as the internal sentinels, so `errors.Is` works:

| Sentinel | Value |
|---|---|
| `sdk.ErrAgentEmptyInput` | `agent: input must not be empty` |
| `sdk.ErrAgentMaxIterations` | `agent: max iterations reached` |
| `sdk.ErrAgentNoProvider` | `agent: provider is not set` |
| `sdk.ErrAgentToolExecFailed` | `agent: tool execution failed` |
| `sdk.ErrToolNil` / `ErrToolEmpty` / `ErrToolDuplicate` / `ErrToolNotFound` | tool registry errors |
| `sdk.ErrMiddlewareNil` / `ErrMiddlewareDuplicate` / `ErrMiddlewareNotFound` | middleware registry errors |
| `sdk.ErrSessionNotFound` / `ErrSessionDuplicate` / `ErrSessionMessageNotFound` / `ErrSessionEmptyTitle` | session store errors |
| `sdk.ErrCacheNotFound` | cache miss |

### Provider errors

Drivers return `*sdk.ProviderError` with a status code:

```go
var perr *sdk.ProviderError
if errors.As(err, &perr) {
	switch {
	case perr.RateLimited():
		// HTTP 429
	case perr.Temporary():
		// 429 or 5xx — the engine retries these automatically
	}
}
```

### Checking errors

```go
if sdk.Is(err, sdk.ErrNoSession, sdk.ErrNotFound) {
	// matches any of the targets
}
```

`Chat`/`Complete` failures are wrapped (`sdk: run: ...`, `sdk: select model: ...`),
so unwrap with `errors.Is` / `errors.As` against the sentinels above.