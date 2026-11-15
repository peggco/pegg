---
icon: lucide/database
---

# Session hooks

Session hooks observe and rewrite persisted conversation state. They fire in the
host process for every agent, including subagents and compaction chains.

## `session.create`

**Filter** — before a session row is inserted.

```json
{
  "title": "New session",
  "provider": "openai",
  "model": "gpt-5",
  "reasoning_effort": "",
  "mode": "auto",
  "project_dir": "/home/me/project",
  "is_subagent": false
}
```

Return the same shape to rename, change the initial mode/model, or tag the project
directory.

## `session.open`

**Action** — when the current session changes.

```json
{ "session_id": "s-1" }
```

## `session.message.append`

**Filter** — before a message is stored. Fires for user, assistant, system, and tool
messages.

```json
{
  "session_id": "s-1",
  "message": {
    "role": "assistant",
    "content": "the answer",
    "reasoning": "",
    "name": "",
    "tool_call_id": "",
    "tool_calls": []
  }
}
```

Return the message to rewrite `content` or `reasoning`; the host normalizes content
back into the `llm.Message` shape. This hook is high-volume — each turn appends
several messages — so keep handlers fast or disable it.

## `session.compact`

**Filter** — before a compaction session is created.

```json
{
  "source_id": "s-1",
  "messages": [{ "role": "user", "content": "..." }],
  "strategy": "sliding-window"
}
```

Return `messages` to change what compaction preserves. The rewritten messages are
written to the new compaction session.

## `session.fork`

**Action** — after a fork or fork-before-message creates a new session.

```json
{
  "source_id": "s-1",
  "new_id": "s-2",
  "at_message_id": "m-42",
  "before_message": false
}
```

`before_message` is `true` for fork-before.

## `session.delete`

**Action** — after a session and its compaction children are removed.

```json
{ "session_id": "s-1" }
```

## `session.title`

**Filter** — before a title change is stored.

```json
{ "session_id": "s-1", "title": "Refactor auth" }
```

## Example: tag generated titles

```go
func (p *Plugin) ExecuteHook(req peggplugin.HookRequest) (*peggplugin.HookResult, error) {
    if req.Name != "session.title" {
        return &peggplugin.HookResult{}, nil
    }
    var payload struct {
        SessionID string `json:"session_id"`
        Title     string `json:"title"`
    }
    if err := json.Unmarshal(req.Value, &payload); err != nil {
        return nil, err
    }
    payload.Title = "[" + p.name + "] " + payload.Title
    out, _ := json.Marshal(payload)
    return &peggplugin.HookResult{Value: out, Handled: true}, nil
}
```
