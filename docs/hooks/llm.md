---
icon: lucide/cpu
---

# LLM hooks

The LLM hooks wrap every provider call the agent loop makes, streaming or not.
Filters receive the exact request the host is about to send, so a plugin can change
the model, messages, tools, or sampling parameters.

## `llm.request`

**Filter** — after middleware, immediately before the provider call.

```json
{
  "agent_id": "a-1",
  "agent_name": "developer",
  "request": {
    "model": "gpt-5",
    "messages": [{ "role": "user", "content": "fix the bug" }],
    "tools": [],
    "temperature": 0.2,
    "max_tokens": 8192
  }
}
```

Return the same shape. Common uses:

- swap `request.model`
- inject a system message
- restrict or add tools
- clamp `temperature` or `max_tokens`

Fields you leave out become zero values, so start from the payload you received and
change only what you need.

## `llm.response`

**Filter** — after the provider response and after middleware.

```json
{
  "agent_id": "a-1",
  "agent_name": "developer",
  "request": { "...": "the request that produced it" },
  "response": {
    "model": "gpt-5",
    "choices": [
      {
        "index": 0,
        "message": { "role": "assistant", "content": "done" },
        "finish_reason": "stop"
      }
    ],
    "usage": { "prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120 }
  }
}
```

In streaming runs this hook sees the assembled response after the last chunk, before
the assistant message is emitted as an agent event.

## `llm.stream.chunk`

**Filter** — once per streamed chunk. Budget: **100 ms**.

```json
{
  "agent_id": "a-1",
  "model": "gpt-5",
  "content": "partial ",
  "reasoning": "",
  "tool_calls": [],
  "finish_reason": "",
  "done": false
}
```

Return the chunk with modified `content`, `reasoning`, or `tool_calls`. Because this
fires for every token, keep handlers allocation-free and consider disabling it with
`hooks.disabled` when not needed. Usage and provider metadata on the original chunk
are preserved.

## `llm.model.resolved`

**Action** — after the smart router and agent defaults pick a provider/model.

```json
{
  "agent_id": "a-1",
  "agent_name": "explorer",
  "input": "find the auth handler",
  "provider": "anthropic",
  "model": "claude-sonnet"
}
```

Observe-only: provider instances are host-owned, so to change the model use
`llm.request` instead.

## `llm.usage`

**Action** — whenever cumulative usage is published, including every chunk that
carries usage.

```json
{
  "agent_id": "a-1",
  "agent_name": "developer",
  "model": "gpt-5",
  "usage": { "prompt_tokens": 1200, "completion_tokens": 340, "total_tokens": 1540 }
}
```

## `llm.error`

**Action** — when the provider call fails and the run aborts.

```json
{
  "agent_id": "a-1",
  "agent_name": "developer",
  "model": "gpt-5",
  "error": "provider rate limited"
}
```

## Example: pin a model per agent

```go
func (p *Plugin) ExecuteHook(req peggplugin.HookRequest) (*peggplugin.HookResult, error) {
    if req.Name != "llm.request" {
        return &peggplugin.HookResult{}, nil
    }
    var payload struct {
        AgentName string `json:"agent_name"`
        Request   struct {
            Model string `json:"model"`
        } `json:"request"`
    }
    if err := json.Unmarshal(req.Value, &payload); err != nil {
        return nil, err
    }
    if payload.AgentName == "explorer" {
        payload.Request.Model = "fast-model"
    }
    out, _ := json.Marshal(payload)
    return &peggplugin.HookResult{Value: out, Handled: true}, nil
}
```
