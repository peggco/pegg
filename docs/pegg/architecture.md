---
icon: lucide/network
---

# Architecture

Pegg is a provider-agnostic AI coding agent written in Go. A single **engine** — the
agent runtime, session store, tool registry, and provider layer — is shared by every
surface: the [CLI](usage/cli.md), [TUI](usage/tui.md), [HTTP API](usage/http.md),
[ACP server](usage/acp.md), [Go SDK](../sdk/index.md), [Desktop](usage/desktop.md),
[VS Code](usage/vscode.md), and [IntelliJ](usage/intellij.md) clients.

This page explains how the pieces fit together. Every diagram is self-contained and
can be lifted straight into a slide deck.

## System map

The full technical picture in one diagram: surfaces drive the agent runtime, every LLM
and tool call passes through the middleware chain, tools are sandboxed by the VFS and
gated by permissions, and the event bus wires the runtime to memory, storage, and
integrations.

```mermaid
flowchart TB
    classDef surface fill:#6366f1,fill-opacity:0.16,stroke:#6366f1,stroke-width:1.5px
    classDef runtime fill:#3b82f6,fill-opacity:0.16,stroke:#3b82f6,stroke-width:1.5px
    classDef mw fill:#f59e0b,fill-opacity:0.16,stroke:#f59e0b,stroke-width:1.5px
    classDef cap fill:#22c55e,fill-opacity:0.16,stroke:#22c55e,stroke-width:1.5px
    classDef safe fill:#ef4444,fill-opacity:0.16,stroke:#ef4444,stroke-width:1.5px
    classDef mem fill:#a855f7,fill-opacity:0.16,stroke:#a855f7,stroke-width:1.5px
    classDef llm fill:#06b6d4,fill-opacity:0.16,stroke:#06b6d4,stroke-width:1.5px
    classDef route fill:#0ea5e9,fill-opacity:0.16,stroke:#0ea5e9,stroke-width:1.5px
    classDef intg fill:#f97316,fill-opacity:0.16,stroke:#f97316,stroke-width:1.5px
    classDef store fill:#64748b,fill-opacity:0.16,stroke:#64748b,stroke-width:1.5px
    classDef bus fill:#64748b,fill-opacity:0.3,stroke:#64748b,stroke-width:2px

    subgraph SURF["Surfaces"]
        direction LR
        CLI["CLI"]
        TUI["TUI"]
        HTTP["HTTP + SSE"]
        ACP["ACP"]
        SDK["Go SDK"]
        IDE["Desktop · VS Code · IntelliJ"]
    end

    subgraph RT["Agent runtime"]
        direction TB
        ORCH["Orchestrator"]
        SUB["Subagents<br/>explorer · developer"]
        LOOP["Run loop<br/>LLM → tools → repeat"]
        ORCH --> SUB
        ORCH --> LOOP
    end

    subgraph MW["Middleware chain"]
        direction LR
        LD["loop-detector"] --> RETRY["retry"] --> STALL["stall"] --> PERM["permission"] --> COMP["compaction"] --> RED["redaction"]
    end

    subgraph CAP["Tools & capabilities"]
        direction LR
        T_FILE["file<br/>read · write · edit · glob · grep · list"]
        T_SHELL["shell<br/>bash"]
        T_WEB["web<br/>search · fetch"]
        T_TASK["task<br/>subagent"]
        T_PLAN["plan mode"]
        T_SKILL["skills"]
        T_MEM["memory tools"]
    end

    subgraph SAFE["Safety & sandbox"]
        direction LR
        VFS["VFS sandbox<br/>.gitignore · no escape"]
        PSTORE["Permission store<br/>allow / reject"]
        JUDGE["LLM judge"]
    end

    subgraph MEM["Memory"]
        direction LR
        WORKER["Background worker<br/>bounded queue"] --> OBS["Observer"] --> LIB["Librarian"] --> PANEL["Memory panel"]
    end

    subgraph LLM["LLM layer"]
        direction LR
        LMGR["LLM Manager"] --> DRV["drivers<br/>openai · claude · gemini · codex"] --> CAT["300+ providers"]
    end

    subgraph DEC["Routing & decisions"]
        direction LR
        ROUTER["Smart Router"] --> DECMGR["Decision layer"]
    end

    subgraph INTG["Integrations"]
        direction LR
        MCP["MCP"]
        LSP["LSP"]
        PLUG["Plugins"]
        STT["STT"]
        SCHED["Scheduler"]
        NOTIF["Notifications"]
    end

    subgraph STORE["Storage"]
        direction LR
        SESS["Sessions<br/>SQLite / JSON"]
        RUNS["Run coordinator<br/>leases · heartbeats"]
        LIVE["Live hub"]
        CACHE["Cache"]
        CFG["Config"]
    end

    BUS{{"Event bus + hooks"}}

    SURF --> RT
    RT --> MW
    MW --> CAP
    MW --> LLM
    CAP --> SAFE
    CAP --> INTG
    RT --> DEC
    DEC --> LLM
    RT --> MEM
    RT --> STORE
    RT <-.-> BUS
    MEM <-.-> BUS
    STORE <-.-> BUS
    INTG <-.-> BUS

    class CLI,TUI,HTTP,ACP,SDK,IDE surface
    class ORCH,SUB,LOOP runtime
    class LD,RETRY,PERM,COMP,RED mw
    class T_FILE,T_SHELL,T_WEB,T_TASK,T_PLAN,T_SKILL,T_MEM cap
    class VFS,PSTORE,JUDGE safe
    class OBS,LIB,PANEL,WORKER mem
    class LMGR,DRV,CAT llm
    class ROUTER,DECMGR route
    class MCP,LSP,PLUG,STT,SCHED,NOTIF intg
    class SESS,RUNS,LIVE,CACHE,CFG store
    class BUS bus
```

## Design principles

- **Provider-agnostic core.** The agent talks to an `llm.Provider` interface, never to a
  vendor SDK. Drivers (OpenAI, Claude, Gemini, Codex) and a catalog of 300+ providers
  plug in behind it. See [Providers & Models](providers-and-models.md).
- **Event-driven.** Components communicate over an in-process event bus and a generic
  filter-hook system, so the TUI, memory, recorder, and live hub observe runs without
  the agent knowing they exist.
- **Middleware pipeline.** Cross-cutting concerns — loop detection, retries,
  permissions, compaction, redaction — are composable middleware around the LLM and
  tool calls, not hard-coded branches.
- **Sandboxed by default.** All file access goes through a virtual filesystem that
  respects `.gitignore`/`.peggignore` and blocks directory escape.
- **One engine, many surfaces.** Frontends are thin; they drive the same runtime and
  render the same event stream.

## Layered architecture

The system is organized in five layers. Surfaces drive the runtime; the runtime uses
capabilities, providers, and storage.

```mermaid
flowchart TB
    subgraph Surfaces["Surfaces"]
        CLI["CLI (cobra)"]
        TUI["TUI (tcell)"]
        HTTP["HTTP API + SSE"]
        ACP["ACP (JSON-RPC 2.0)"]
        SDK["Go SDK"]
        Desktop["Desktop (Wails)"]
        VSCode["VS Code"]
        IntelliJ["IntelliJ"]
    end

    subgraph Runtime["Core runtime"]
        Agent["Agent framework<br/>run loop · middleware · hooks"]
        Subagents["Subagents<br/>orchestrator · explorer · developer"]
        Router["Smart Router"]
        RunCoord["Run Coordinator"]
        LiveHub["Live Hub"]
    end

    subgraph Capabilities["Capabilities"]
        Tools["Tools"]
        Skills["Skills"]
        Memory["Memory"]
        Scheduler["Scheduler"]
        Reminders["Reminders"]
    end

    subgraph Providers["Providers & integrations"]
        LLM["LLM Manager<br/>300+ providers"]
        Decision["Decision layer"]
        MCP["MCP servers"]
        LSP["LSP servers"]
        Plugins["Plugins"]
    end

    subgraph Storage["Storage"]
        Sessions["Sessions (SQLite / JSON)"]
        Cache["Cache"]
        VFS["Sandboxed VFS"]
        Config["Config"]
    end

    Surfaces --> Runtime
    Runtime --> Capabilities
    Runtime --> Providers
    Runtime --> Storage
    Capabilities --> Providers
    Capabilities --> Storage
```

## The agent run loop

A run is a loop of *LLM call → tool calls → feed results back*, repeated until the
model stops asking for tools or the iteration budget is exhausted. Middleware wraps
every step.

```mermaid
sequenceDiagram
    autonumber
    participant U as User / Surface
    participant A as Agent
    participant M as Middleware chain
    participant L as LLM Provider
    participant T as Tools

    U->>A: Run(input)
    A->>A: resolve tools, middleware, model
    A->>M: BeforeRun
    loop until finish or max iterations
        A->>M: BeforeLLM
        M->>L: Chat / ChatStream
        L-->>A: message + tool calls
        A->>M: AfterLLM
        alt tool calls present
            A->>M: BeforeTool
            M->>T: Execute
            T-->>A: result
            A->>M: AfterTool
        else no tool calls
            A->>A: finish
        end
    end
    A->>M: AfterRun
    A-->>U: RunResult
```

Key behaviours in the loop:

| Concern | Handling |
|---|---|
| Streaming | `ChatStream` emits tokens, reasoning, tool calls, and a final `done` event |
| Parallel tools | Up to 8 tool calls execute concurrently per iteration |
| Empty responses | Retried up to 3 times with a continuation nudge |
| Truncation | Recovered up to 3 times, escalating the output-token budget |
| Iterations | Default 150; the orchestrator runs unlimited |
| Cancellation | Context cancellation stops the loop and returns a partial result |

## Agent framework and subagents

The **orchestrator** is the top-level agent. It delegates to specialised subagents via
the `task` tool. Subagents inherit the parent's model and provider, run concurrently,
persist their own session, and can be resumed with follow-up messages.

```mermaid
flowchart TB
    O["Orchestrator<br/>plans · delegates · verifies"]
    O -->|task tool| E["Explorer<br/>read-only research"]
    O -->|writes| P["Plan + todos<br/>.pegg/plans/"]
    O -->|task tool| D["Developer<br/>implements + tests"]
    E -.->|background reminder| O
    D -.->|background reminder| O
    D -->|task tool| N["Nested subagent"]
```

- **Foreground** subagents block the parent until they finish and return their output.
- **Background** subagents run detached; the parent gets a system reminder when they
  complete and is never blocked.
- Re-using a subagent **name** resumes it with its existing conversation history.

See [Subagents](features/subagents.md) for the full model.

## Middleware pipeline

Middleware implements a fixed set of hooks and can also wrap the LLM and tool
invocations (an onion model). The orchestrator's default chain is shown below;
`redaction` is registered globally and applied where configured.

```mermaid
flowchart LR
    In["Request"] --> LD["loop-detector"]
    LD --> RT["retry"]
    RT --> ST["stall"]
    ST --> PM["permission"]
    PM --> CP["compaction"]
    CP --> RD["redaction"]
    RD --> Call["LLM / Tool call"]
    Call --> Out["Response"]
```

| Middleware | Responsibility |
|---|---|
| `loop-detector` | Detects repeated tool calls and text across turns, and aborts a stream when the same sentence or block repeats inside a single response |
| `retry` | Retries transient provider failures with backoff |
| `stall` | Detects silent provider requests and re-issues the same request without surfacing an error |
| `permission` | Gates sensitive tools (five modes, LLM judge, allow store) |
| `compaction` | Summarises or trims history near the context limit |
| `redaction` | Strips secrets (API keys, JWTs) before they reach the model |

The `Middleware` interface exposes seven hooks — `BeforeRun`, `AfterRun`, `BeforeLLM`,
`AfterLLM`, `BeforeTool`, `AfterTool`, `OnError` — plus optional `InvokeLLM`,
`InvokeLLMStream`, and `InvokeTool` wrappers. See [Permissions](features/permissions.md).

## Event bus and hooks

Components are decoupled by an in-process **event bus** (topic → handlers) and a
generic **filter-hook** system (value in → value out). The agent publishes; observers
subscribe.

```mermaid
flowchart LR
    subgraph Producers
        Agent["Agent"]
        Session["Session"]
        Run["Run"]
        Schedule["Schedule"]
    end
    Bus["Event bus<br/>topic → handlers"]
    subgraph Consumers
        TUI["TUI"]
        Memory["Memory"]
        Recorder["Session recorder"]
        LiveHub["Live hub"]
        Notify["Notifications"]
    end
    Producers --> Bus
    Bus --> Consumers
```

Representative topics:

| Group | Topics |
|---|---|
| Agent | `agent.started`, `agent.input`, `agent.message`, `agent.token`, `agent.tool.call`, `agent.tool.result`, `agent.usage`, `agent.finished`, `agent.error`, `agent.ask`, `agent.ask.answer` |
| Session | `session.created`, `session.updated`, `session.deleted`, `session.message.added`, `session.forked`, `session.reverted`, `session.compacted` |
| Run | `run.started`, `run.finished`, `run.ask.resolved` |
| App / LLM | `app.mounted`, `provider.added`, `provider.removed`, `llm.models.loaded` |
| Schedule | `schedule.created`, `schedule.started`, `schedule.finished`, `schedule.failed` |

Filter hooks let extensions rewrite values in flight — for example `OnModelResolve`
(the Smart Router), `OnMessageInput` (memory panel injection), and the VFS hooks
`OnBeforeRead` / `OnAfterWrite` (LSP diagnostics).

## LLM layer

The `llm.Manager` owns provider instances, model discovery, and pricing. A request
resolves to a `Provider` implementation, which is one of a handful of wire drivers
backed by a large provider catalog.

```mermaid
flowchart TB
    Req["Chat request"] --> Mgr["LLM Manager"]
    Mgr --> Reg["Provider registry"]
    Reg --> D1["openai driver"]
    Reg --> D2["claude driver"]
    Reg --> D3["gemini driver"]
    Reg --> D4["codex driver"]
    D1 --> Cat["300+ provider catalog"]
    D2 --> Cat
    D3 --> Cat
    D4 --> Cat
    Cat --> API["Provider HTTP API"]
    Mgr --> Cache["Model + price cache"]
```

- Models are discovered from the provider API, cached, and enriched with context-window,
  pricing, and capability metadata.
- Credentials are read from provider config or local CLI credential stores.
- The manager exposes `Select` (exact or preferred) and `Provider` lookups used by the
  router, scheduler, and every surface.

## Smart Router and decision layer

When enabled, the **Smart Router** picks a model per task at run start by registering an
`OnModelResolve` hook. It scores task difficulty, builds a candidate list, and asks a
**decision model** (or an LLM fallback) to choose.

```mermaid
flowchart TB
    Start["OnModelResolve hook"] --> En{"Smart Router enabled?"}
    En -- no --> Keep["Keep configured model"]
    En -- yes --> Diff{"Difficulty configured?"}
    Diff -- yes --> Score["Score task difficulty<br/>decision → LLM"]
    Diff -- no --> Cand
    Score --> Cand["Build candidates<br/>preferences + vision filter"]
    Cand --> Dec{"Decision provider?"}
    Dec -- yes --> Pick["Decision picks model"]
    Dec -- no --> LLM["LLM picks model"]
    Pick --> Set["Set agent model"]
    LLM --> Set
    Set --> Keep
```

The **decision layer** (`internal/decision`) is a small interface — `noul`, `choice`,
and `score` questions — with providers such as OpenRouter's JEV. It is reused by the
router, the permission judge, and memory relevance gating.

## Sessions and run coordination

Sessions are persisted in SQLite (or JSON) and can be forked, reverted, and compacted
into chains. Concurrent runs are serialised per session by the **Run Coordinator**,
which hands out leases, heartbeats them, and reaps stale runs.

```mermaid
stateDiagram-v2
    [*] --> Running: Claim lease
    Running --> Running: heartbeat
    Running --> Done: finish
    Running --> Canceled: cancel
    Running --> Error: failure
    Done --> [*]
    Canceled --> [*]
    Error --> [*]
```

- The **Live Hub** polls run events and fans them out to subscribers (SSE, ACP, connect),
  so a run started in one process can be watched from another.
- The **Session Recorder** subscribes to agent events and persists messages as they
  stream.
- Compaction creates a child session linked to its parent, preserving the full history.

See [Sessions](features/sessions.md).

## Tools, skills, and plan mode

Tools are registered in a global registry and exposed to the model as function
definitions. Skills are markdown playbooks loaded on demand. Plan mode gates the
plan tools so the agent can research before acting.

```mermaid
flowchart LR
    Reg["Tool registry"] --> File["file: read · write · edit · glob · grep · list"]
    Reg --> Shell["shell: bash"]
    Reg --> Web["web: websearch · webfetch"]
    Reg --> Todo["todo: todowrite · todoread"]
    Reg --> Subagent["subagent: task · taskstatus"]
    Reg --> Plan["plan: enterplanmode · exitplanmode"]
    Reg --> Skill["skills: loadskill · searchskill"]
    Reg --> Mem["memory: mem-search · mem-read"]
    Reg --> Misc["ask · sendfile · schedule"]
```

| Group | Tools |
|---|---|
| Files | `read`, `write`, `edit`, `glob`, `grep`, `list` |
| Shell | `bash` |
| Web | `websearch`, `webfetch` |
| Planning | `enterplanmode`, `exitplanmode`, `todowrite`, `todoread` |
| Delegation | `task`, `taskstatus` |
| Skills | `loadskill`, `searchskill` |
| Memory | `mem-search`, `mem-read` |
| Interaction | `askuserquestion`, `sendfile` |
| Automation | `schedule` |

See [Tools](configurations/tools.md) and [Skills](configurations/skills.md).

## Integrations

Beyond the LLM, the agent talks to several external systems. Each is optional and
degrades gracefully when unavailable.

```mermaid
flowchart LR
    Agent["Agent"] --> MCP["MCP servers<br/>stdio / SSE / Streamable HTTP"]
    Agent --> LSP["LSP servers<br/>diagnostics on read/write"]
    Agent --> Plugins["Plugins<br/>go-plugin RPC"]
    Agent --> Memory["Memory<br/>observer + librarian"]
    Agent --> Scheduler["Scheduler<br/>cron"]
    Agent --> STT["Speech-to-text"]
    Agent --> Notify["Notifications"]
```

| Integration | Transport / mechanism | Docs |
|---|---|---|
| MCP | stdio, legacy SSE, or Streamable HTTP; tools registered into the registry | [MCP](configurations/mcp.md) |
| LSP | stdio; diagnostics appended to file reads/writes | [LSP](configurations/lsp.md) |
| Plugins | HashiCorp go-plugin over net/rpc | [Plugins](configurations/plugins.md) |
| Memory | markdown files + background LLM passes | [Memory](features/memory.md) |
| Scheduler | cron, each run saved as its own session | [Scheduling](features/scheduling.md) |
| STT | provider drivers (OpenAI, AssemblyAI, …) | [Voice input](features/voice.md) |

## Surfaces over one engine

Every frontend drives the same engine. The editor and desktop clients speak **ACP**
(JSON-RPC 2.0) to a local Pegg process; the CLI, TUI, HTTP server, and SDK embed the
engine directly.

```mermaid
flowchart TB
    Engine["Pegg engine<br/>agent · sessions · tools · providers"]
    CLI["CLI"] --> Engine
    TUI["TUI"] --> Engine
    HTTP["HTTP API"] --> Engine
    SDK["Go SDK"] --> Engine
    ACP["ACP server"] --> Engine
    Desktop["Desktop"] -->|ACP client| ACP
    VSCode["VS Code"] -->|ACP client| ACP
    IntelliJ["IntelliJ"] -->|ACP client| ACP
```

| Surface | Entry point | Transport |
|---|---|---|
| CLI | `pegg run` | in-process |
| TUI | `pegg` | in-process |
| HTTP API | `pegg serve` | REST + SSE |
| ACP | `pegg serve --acp` | stdio / HTTP+SSE / WebSocket |
| Go SDK | `pkg/sdk` | in-process |
| Desktop | Wails app | ACP over stdio |
| VS Code | extension | ACP over stdio |
| IntelliJ | plugin | ACP over stdio |

## Package map

```text
cmd/pegg/                 binary entry point
internal/
├── agent/                agent framework: run loop, middleware, hooks, tools, prompt
├── builtin/              built-in agents, middlewares, tools, reminders, skills
├── cli/                  cobra command tree and all subcommands
├── connect/              relay daemon (WebSocket) for the web client
├── core/                 event bus, hooks, config, bootstrap, cache, logger
├── decision/             decision-model interface and providers
├── live/                 run-event hub and fan-out
├── llm/                  provider manager, drivers, 300+ provider catalog
├── lsp/                  language-server manager and diagnostics
├── mcp/                  Model Context Protocol client
├── memory/               observer, librarian, store, search tools
├── plugin/               go-plugin host and shared interface
├── router/               Smart Router (per-task model selection)
├── run/                  run coordinator: leases, heartbeats, commands
├── schedule/             cron scheduler and run history
├── server/               HTTP API and SSE handlers
├── session/              session store, recorder, runs, compaction
├── skill/                skill engine and loader
├── skillhub/             skill hub install/search
├── stt/                  speech-to-text drivers and providers
├── tui/                  tcell terminal UI
├── vfs/                  sandboxed virtual filesystem
└── acp/                  Agent Client Protocol server
pkg/
├── sdk/                  embeddable Go SDK
├── desktop/              Wails desktop app + Go ACP client
├── chat-ui/              shared React chat renderer
├── vscode/               VS Code extension
└── intellij/             IntelliJ plugin
```

## Next steps

- [Overview](overview.md) — what Pegg is and what it can do.
- [Providers & Models](providers-and-models.md) — the provider layer in depth.
- [Subagents](features/subagents.md) — the delegation model.
- [Go SDK](../sdk/index.md) — embed the engine in your own program.
