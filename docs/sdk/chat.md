---
icon: lucide/message-circle
---

# Chat

The SDK offers two levels of interaction: **chat** runs the full agent loop with
tools, subagents, and permissions; **completions** are raw single-shot LLM calls.

## Chat

```go
resp, err := eng.Chat(ctx, sdk.ChatRequest{
	Input: "Refactor this package",
	Provider: "openai",
	Model: "gpt-4o",
})
```

### `ChatRequest`

| Field | Description |
|---|---|
| `Input` | The user prompt (required) |
| `SessionID` | Resume a session; its messages are replayed as history |
| `History` | Extra prior messages, prepended before any session messages |
| `Provider` | Provider to use; empty auto-selects |
| `Model` | Model to use; empty auto-selects |
| `Files` | Local file paths; copied into the attachment store (`os.TempDir()/pegg`) |
| `Attachments` | Direct `Attachment` values |
| `Tools` | Names of registered tools to enable for the run |
| `SystemPrompt` | Overrides the orchestrator system prompt |
| `MaxIterations` | Agent loop cap; defaults to 150 |
| `ReasoningEffort` | For reasoning models, e.g. `"low"`, `"medium"`, `"high"` |
| `Timeout` | Per-call timeout (wraps the context) |

### `ChatResponse`

```go
resp.Output       // final text
resp.Usage        // prompt/completion/total tokens and cost
resp.Iterations   // number of LLM turns
resp.FinishReason // stop | length | content_filter | tool_calls | null
resp.AgentID      // id of the orchestrator run
resp.AgentName    // "orchestrator"
resp.SessionID    // persisted session id ("" if the recorder is disabled)
resp.History      // the full message history
```

When `SessionID` is set, the engine loads the session's messages, appends `History`
before them, publishes `session.resume`, and returns the session id used for the run
in `resp.SessionID`. An unknown session returns `sdk.ErrNoSession`.

### Model resolution

| You provide | Behavior |
|---|---|
| `Provider` + `Model` | Exact match against the provider's model list; not found → `sdk.ErrNoModels` |
| `Provider` only | Exact mode; must resolve to a model |
| Neither | **Preferred** mode: the last session's provider/model for the workspace, else the first provider with models |
| No providers at all | `sdk.ErrNotConfigured` |
| Resolution timeout | `sdk.ErrModelTimeout` |

## Streaming chat

```go
_, err := eng.ChatStream(ctx, req, func(ev sdk.ChatEvent) error {
	// ev.Type is one of:
	//   sdk.EventToken       -> ev.Content
	//   sdk.EventReasoning   -> ev.Reasoning
	//   sdk.EventToolCall    -> ev.ToolCall
	//   sdk.EventToolResult  -> ev.ToolOutput or ev.ToolErr
	//   sdk.EventCompaction  -> ev.Strategy, ev.Messages, ev.Tokens
	//   sdk.EventDone        -> ev.Content + ev.Usage
	return nil
})
```

Events arrive in order: reasoning/token, tool call, tool result, then `done`. A
`nil` handler degrades to a plain non-streaming `Chat`.

When the context is compacted mid-run, an `sdk.EventCompaction` event is emitted
with `ev.Strategy` (e.g. `sliding-window`), `ev.Messages` (messages the model now
sees) and `ev.Tokens` (prompt tokens at compaction time).

Compaction can also be observed bus-style with
`eng.OnCompaction(func(ev sdk.CompactionEvent) { ... })`; the returned function
unsubscribes.

## Completions

Raw LLM calls with no agent loop:

```go
resp, err := eng.Complete(ctx, sdk.CompleteRequest{
	Provider:    "openai",
	Model:       "gpt-4o-mini",
	Messages:    []sdk.Message{sdk.UserMessage("Hello!")},
	Temperature: 0.2,
	MaxTokens:   100,
})
fmt.Println(resp.GetContent())
```

### `CompleteRequest`

| Field | Description |
|---|---|
| `Provider`, `Model` | Model resolution, same rules as chat |
| `Messages` | Conversation history |
| `Temperature`, `TopP` | Sampling parameters |
| `MaxTokens` | Output token limit |
| `Tools` | `[]LLMTool` function specifications |
| `ToolChoice` | `"auto"`, `"none"`, or a specific tool |
| `ReasoningEffort` | Reasoning level for supporting models |
| `User` | End-user identifier |
| `ResponseFormat` | `text`, `json_object`, or `json_schema` |
| `Timeout` | Per-call timeout |

Streaming completions use `StreamChunk` events:

```go
err := eng.CompleteStream(ctx, req, func(chunk sdk.StreamChunk) error {
	fmt.Print(chunk.Content)
	return nil
})
```

## Models and providers

```go
models, err := eng.Models(ctx, "openai")     // cached model list for a provider
prov,   err := eng.Provider(ctx, "openai")   // the live provider handle
names       := eng.Providers()               // all registered provider names
```

## Attachments

```go
sdk.NewImageAttachmentFromBase64("image/png", data)
sdk.NewImageAttachmentFromURL(url)
sdk.NewAudioAttachmentFromBase64("audio/mpeg", data)
sdk.NewVideoAttachmentFromBase64("video/mp4", data)
sdk.NewFileAttachmentFromBase64("application/pdf", data, "doc.pdf")
```

`Files` are copied into the attachment store and the agent is told their paths.
Image, audio, and video the model supports are also sent inline; other files are
referenced by path. Image, audio, and video attachments are validated against the
model's input modalities — unsupported media is never sent inline.