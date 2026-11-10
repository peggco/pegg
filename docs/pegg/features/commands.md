---
icon: lucide/square-terminal
---

# Commands

Pegg's slash commands are implemented as **skills**: type `/name` in the input
and Pegg issues a `loadskill` tool call, so the skill's instructions arrive as a
tool result (or run as a background subagent for `context: fork` skills). The
built-in commands are `/init`, `/batch`, `/orchestrate`, `/review`, `/rule`, and
`/skillify` — and you can [add your own](../configurations/skills.md#creating-your-own)
with a `SKILL.md` file.

In the TUI and the [VS Code extension](../usage/vscode.md), type `/` in the input to
open the picker and filter by name. The same token works in the CLI
(`pegg run "/init"`) and inside subagent task messages.

In VS Code, the `@pegg` chat participant also exposes `/explain`, `/fix`,
`/review`, `/tests`, `/init`, `/rule`, and `/batch` directly in the Chat view.

## `/init`

Instructs Pegg to analyze a codebase and create or improve an `AGENTS.md` file.
Use it on a repository that lacks agent guidance, or to refresh an outdated one.

The generated file follows a fixed structure:

- Project Overview
- Repository Structure
- Build & Development Commands
- Code Style & Conventions
- Architecture Notes
- Testing Strategy
- Security & Compliance
- Agent Guardrails
- Extensibility Hooks
- Further Reading

Rules the init command follows:

- The file must start with the `# AGENTS.md` heading.
- Content is limited to roughly 12k tokens; keep it dense and factual.
- Unresolved details are marked inline with `> TODO:` so you can fill them in.
- Information the codebase doesn't reveal is not invented — it is left as a
  placeholder or omitted.

The result is a file the agent framework reads on every run as
[Project Instructions](../configurations/rules.md#rules-vs-agentsmd).

## `/batch`

Instructs Pegg to orchestrate a large, parallelizable change from exploration to
report, using subagents. This is the orchestrator's working contract for multi-file
work.

The workflow:

1. **Decompose** the task into independent chunks that can be parallelized.
2. **Explore** the codebase to map the affected areas.
3. **Plan** by writing a plan file under `.pegg/plans/<feature-name>.md`.
4. **Delegate** chunks to explorer and developer subagents via the
   `task` tool, running them concurrently.
5. **Verify** the results before accepting them.
6. **Report** the outcome, including what changed and what remains.

Supporting rules the batch command applies:

- Subagent names are unique and task-descriptive; results can be re-used from the
  session's subagent store (`.pegg/subagents/<session-id>.json`).
- The plan file is the source of truth for what is being changed and why.
- Todo items use a hierarchy (`todo-1`, `todo-1.1`, ...), priorities, and
  `dependsOn` links so progress and dependencies stay visible.
- Every subagent either proves its work (tests, checks) or reports a failure.

The lifecycle tools — `task` and `taskstatus` — back the whole flow. See
[Subagents](subagents.md).

## `/orchestrate`

The full orchestration contract for large, multi-step projects: understand, plan,
delegate, verify, and report. Load it when a request spans many files or
subsystems and needs a written plan rather than a quick edit.

The workflow:

1. **Understand** the request and resolve ambiguity with the user.
2. **Explore** the codebase, delegating to `explorer` subagents.
3. **Plan** by writing `.pegg/plans/<feature-name>.md` yourself (use plan mode
   for large work), then create a dependency-aware todo hierarchy.
4. **Delegate** independent tasks to `developer` subagents, in parallel where
   the dependency graph allows.
5. **Verify** each result before marking it done; re-delegate failures to the
   same subagent with specific feedback.
6. **Report** with a summary, key decisions, the execution strategy, the plan
   path, and a todo status table (`Todo ID`, `Title`, `Description`, `Status`,
   `Priority`, `Depends On`, `Parallelizable Tasks`).

Unlike `/batch` (a bulk change decomposed into many independent work units),
`/orchestrate` is the general plan-first contract for any large project. It
replaces the former planner subagent: the orchestrator writes the plan and todo
hierarchy itself.

## `/review`

Instructs Pegg to review a pull request or changeset and produce structured
feedback. Use it when you want a second pass over a diff before merging.

The review pipeline:

1. **Assess size** — determine whether the review can be done in one pass or must be
   partitioned.
2. **Gather context** — an Explorer subagent collects relevant background from the
   codebase.
3. **Review hunks** — Developer subagents review the diff piece by piece, recording
   findings as:

   ```
   - **[File:Line]** [Severity] - [Issue] - [Fix]
   ```

4. **Verify findings** — a Verifier pass filters the list into `[Confirmed]`,
   `[Plausible]`, and `[Refuted]`, dropping noise and duplicates.
5. **Sweep for gaps** — look for issues the hunk-by-hunk review might have missed
   (security, tests, docs).
6. **Report** — produce inline review comments on the VCS, or a Markdown report.

## `/rule`

Creates or updates a rule file under `<project>/.pegg/rules/` (project scope) or
`~/.pegg/rules/` (global scope). See
[Rules](../configurations/rules.md#creating-rules-with-the-agent) for the full
workflow.

## Other commands

These are the only literal slash commands in the chat loop:

| Input | Effect |
|---|---|
| `exit`, `quit`, `/exit`, `/quit`, `bye` | End the CLI `run --chat` loop |

Anything else starting with `/` that is not a registered skill is left in the
message text. There is no hidden command palette — Settings (++ctrl+p++ in the TUI)
and the `/` picker are the full command surface.