---
icon: lucide/wrench
---

# Tool hooks

Tool hooks wrap each tool execution. Use them to inspect or rewrite arguments,
block calls, or post-process output. They fire after the built-in `BeforeTool`
middleware and before the tool runs.

## `tool.before`

**Filter** — after middleware approval, before the tool executes.

```json
{
  "agent_id": "a-1",
  "agent_name": "developer",
  "call_id": "call_1",
  "tool": "bash",
  "arguments": "{\"command\": \"ls\"}",
  "blocked": false,
  "reason": ""
}
```

Return `arguments` to rewrite the call. Return `blocked: true` with a `reason` to
stop the tool; the reason is shown to the model as the tool error.

```go
if payload.Tool == "bash" && strings.Contains(payload.Arguments, "rm -rf") {
    payload.Blocked = true
    payload.Reason = "destructive command blocked by policy plugin"
}
```

## `tool.after`

**Filter** — after execution, before `AfterTool` middleware.

```json
{
  "agent_id": "a-1",
  "agent_name": "developer",
  "call_id": "call_1",
  "tool": "bash",
  "arguments": "{\"command\": \"ls\"}",
  "output": "main.go\nREADME.md",
  "error": ""
}
```

Return `output` to replace what the model and the transcript see. `error` is empty on
success.

## `tool.permission`

**Filter** — the first thing the permission middleware checks.

```json
{
  "agent_id": "a-1",
  "tool": "bash",
  "arguments": "{\"command\": \"git status\"}",
  "decision": "",
  "reason": ""
}
```

Set `decision` to `allow` to run the tool without prompting, or `deny`/`reject` with
a `reason` to refuse. Leave it empty to continue through the normal mode, judge, and
prompt flow. This lets a policy plugin approve safe commands and deny risky ones
before the judge model runs.

!!! warning
    `allow` bypasses the permission prompt for that call. Only approve calls you have
    fully validated yourself.
