---
icon: lucide/heart-handshake
---

# Subagents

Pegg is built around a main **orchestrator** agent that can delegate work to
specialized **subagents**. Each subagent runs its own loop with its own tools,
prompt, and middleware, and runs **concurrently** with its siblings.

## The agent roster

| Agent | Role | Write access | Tools |
|---|---|---|---|
| `orchestrator` | Top-level coordinator; plans, delegates, verifies | Full workspace | All file tools, `askuserquestion`, `bash`, `task`, `taskstatus`, `todoread`, `todowrite`, `webfetch`, `websearch`, `searchskill`, `loadskill`, `enterplanmode`, `exitplanmode` |
| `developer` | Implementation | Full workspace except `.pegg/` config (`.pegg/plans/` and `.pegg/rules/` remain writable) | All file tools, `bash`, `webfetch`, `websearch`, `todoread`, `todowrite` |
| `explorer` | Read-only research | None | `glob`, `grep`, `list`, `read`, `bash`, `webfetch`, `websearch` |

The **orchestrator** owns planning: for non-trivial work it writes the plan to
`.pegg/plans/<feature-name>.md`, creates the todo hierarchy, and then delegates
execution to developers. Load the `/orchestrate` skill for the full
plan → delegate → verify → report contract.

All agents share a common system prompt that establishes the interaction style
(tone, proactiveness, code conventions), injects the working environment, git
status, the [skills list](../configurations/skills.md), [AGENTS.md and rules](../configurations/rules.md), and
requires code references in `file:line` format. Every agent runs the same middleware
chain: loop detection, secret redaction, retry, and permissions.

## Sampling and limits

| Agent | Temperature | Top-p | Max tokens | Max iterations |
|---|---|---|---|---|
| `orchestrator` | 0.2 | 0.9 | `max_tokens` / `max_output_tokens` | unlimited |
| `explorer` | 0.1 | 0.9 | `max_tokens` / `max_output_tokens` | 150 |
| `developer` | 0.2 | 0.9 | `max_tokens` / `max_output_tokens` | 150 |

Sampling stays close to deterministic for tool routing, file research, and code
changes (`orchestrator`, `explorer`, `developer`). Top-p remains near the
provider default; the primary dial is temperature. Iteration caps are guardrails
against runaway tool loops, not targets. The orchestrator is intentionally
unlimited because it coordinates subagents and long multi-step tasks; `explorer`
and `developer` keep a finite cap.

## Spawning subagents

The orchestrator uses the `task` tool to delegate. Each call spawns exactly one
subagent:

```json
{
  "name": "explore-api",
  "subagent_type": "explorer",
  "prompt": "Find all REST endpoints in internal/server and report file:line references.",
  "task_id": ["todo-1"],
  "background": false
}
```

| Field | Description |
|---|---|
| `name` | Unique, descriptive name for this run. Reusing a finished name resumes with full history. |
| `subagent_type` | One of `orchestrator`, `explorer`, `developer` |
| `prompt` | Instructions: what to do, constraints, what to report back |
| `task_id` | Optional todo ids this subagent is working on |
| `background` | `false` (default) blocks until finished; `true` returns immediately |

Foreground mode (`background: false`) blocks until the subagent completes or fails,
then returns its result directly. Background mode returns a confirmation immediately
and lets the orchestrator continue; results arrive as system reminders. See
[Reminders](reminders.md) for the reminder system overview.

While a background subagent is running, the orchestrator receives a **standing
system reminder** on every message telling it the subagent is working in the
background and that it may continue its own work or finish its turn — the system
wakes it when the subagent completes. The standing reminder is removed automatically
once no background subagent of the same run is left running.

To spawn multiple subagents in parallel, issue multiple `task` tool calls in a
single message.

## Delegation details

- Subagents inherit the parent's **provider**, **model**, and **event bus** — they
  stream into the same transcript.
- In the TUI, each `task` call is one **TASK** card with a live activity line, an
  output preview, usage, and a **History** view of the full transcript. Forked
  subagents (no `task` call) keep the standalone subagent card.
- VS Code, IntelliJ, and desktop use the same **TASK** card as the TUI: name,
  subagent type, prompt, `background` badge, live activity, response preview,
  usage/duration, and a **History** view while running. Task-launched subagents
  do not get a second subagent card; fork-launched subagents keep it. The final
  response is persisted in the `task` tool message, so restored sessions show it.
- The `task_id` field links a subagent to todo items, so progress is trackable via
  the todo tools.

## Background subagent notifications

When a background subagent completes, the parent agent receives a **system
reminder** automatically — no polling required. The reminder is injected into the
parent's LLM context as an XML block before the next iteration:

```xml
<system-reminders>
<system-reminder tag="subagent" task="todo-1,todo-2" agent="dark-mode-implementation">
  Subagent "dark-mode-implementation" finished.
  Response:
  Dark mode toggle implemented. All 47 tests pass.
</system-reminder>
</system-reminders>
```

For failed subagents:

```xml
<system-reminders>
<system-reminder tag="subagent" task="todo-3" agent="payment-fix">
  Subagent "payment-fix" failed: currency conversion rounding error in line 42
</system-reminder>
</system-reminders>
```

Key details:

- Multiple reminders from different subagents are **batched** into a single
  `<system-reminders>` block per loop iteration.
- Reminders are only sent for **background** subagents. Foreground subagents return
  results directly as the tool output.
- The parent does not need to poll or wait — the reminder arrives automatically when
  the subagent finishes.
- In the TUI, the **TASK** card stays in the running state after the orchestrator
  finishes or is stopped; the background subagent is not tied to the orchestrator's
  context. When it completes, the reminder wakes the orchestrator and the card closes
  with the result. The subagent's History stays live while it runs, even after the
  main run has finished.
- The final response is written back to the `task` tool message, so restored sessions
  show the background subagent's result instead of only the start confirmation.
- Wake-up is handled by the core, not by an individual surface. A process-wide waker
  claims a continuation run for the parent session and calls `Continue` with the
  persisted history, so the reminder is injected as a system message. This applies
  to the TUI, interactive CLI, `pegg serve`/ACP clients (VS Code, IntelliJ, desktop),
  HTTP/connect, and the SDK.
- ACP clients receive the continuation as a normal streamed run (`run_state_update`
  plus message/thought chunks). No new prompt is required from the editor client;
  the shared TASK card updates live and closes when the subagent finishes.

## Checking subagent status

Use `taskstatus` to inspect subagents:

```json
{}
```

Returns all subagents, or filter by name:

```json
{
  "agent_names": ["explore-api", "plan-refactor"]
}
```

Status values: `pending`, `running`, `completed`, `failed`, `interrupted`.

## Resuming subagents

Subagents are scoped to the session that spawned them, like todos. Their state is
stored per session in `.pegg/subagents/<session-id>.json` and they keep their full
conversation history. To resume a finished subagent from the **same session**, call
`task` again with the **same name**:

```json
{
  "name": "explore-api",
  "subagent_type": "explorer",
  "prompt": "Now look at the authentication middleware specifically — check for token expiry handling."
}
```

The subagent resumes with its complete prior context. Use this to iterate on
completed work without losing context. A new session starts with an empty subagent
list — subagents from other sessions are neither listed nor resumable.

Only spawn a fresh subagent with a new name when no relevant prior subagent exists.

## Persistence and restart

Subagent state is persisted to `.pegg/subagents/<session-id>.json` in the project
directory. If Pegg restarts while subagents are running, those subagents are marked
`interrupted` (with reason "app restarted"). You can inspect them with
`taskstatus` and resume one by re-spawning with the same name.

Subagent conversations are also stored as sessions — titled
`<parent session title> - <subagent name>` — so their history is durable and
replayable.

## `/batch` orchestration

For large, parallelizable changes, invoke the `/batch` skill. It turns the
orchestrator into a batch manager that explores, plans, delegates to subagents,
verifies results, and reports back — following the subagent lifecycle described
above. See [Commands](commands.md#batch).

For the general plan-first contract on any large project, invoke `/orchestrate`.
It makes the orchestrator write the plan and todo hierarchy itself, then delegate
execution and verify the results. See [Commands](commands.md#orchestrate).
