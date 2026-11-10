---
icon: lucide/biceps-flexed
---

# Skills

A **skill** is a named bundle of instructions that can be injected into a
conversation on demand. Skills keep long, reusable workflows out of the base system
prompt while making them available to the agent with a single `/name` token.

## SKILL.md format

A skill is a directory containing a `SKILL.md` file: YAML frontmatter followed by a
Markdown body. The body is the instruction text.

```markdown title="SKILL.md"
---
name: pdf-tools
description: Extract text from PDF files. Use when handling PDFs.
license: MIT
compatibility: Requires pdfplumber
metadata:
  author: example-org
  version: "1.0"
allowed-tools: "Bash(python:*) Read"
---

# PDF Tools

Extract text with pdfplumber:

1. Run `python scripts/extract.py <file>`.
2. Summarize the output for the user.
```

| Frontmatter field | Required | Description |
|---|---|---|
| `name` | no | Invocation name. Defaults to the directory name. Must match `^[a-z0-9]+(?:-[a-z0-9]+)*$`, max 64 characters |
| `description` | recommended | Shown in the agent's skill list and in the TUI |
| `license` | no | Free-form license text |
| `compatibility` | no | Free-form prerequisites |
| `metadata` | no | Arbitrary key/value pairs |
| `allowed-tools` | no | Space-separated list of tool names the skill expects to use |
| `context` | no | `fork` runs the skill in a forked background subagent; anything else (default) loads it into the current conversation |

A `scripts/` subdirectory next to `SKILL.md` is detected automatically and its path
is included in the expansion, so skills can ship helper scripts.

## Locations

Skills are loaded from two global directories at startup:

| Directory | Notes |
|---|---|
| `~/.agents/skills/` | Shared agent-skills location |
| `~/.pegg/skills/` | Pegg skills, including the built-ins |

If both define the same skill name, `~/.pegg/skills/` wins. Only immediate
subdirectories are scanned, and each must contain a parseable `SKILL.md`; invalid
skills are skipped silently.

The built-in skills are **materialized** into `~/.pegg/skills/` on first
run. Existing directories are never overwritten, so you can edit the built-ins in
place.

There is currently no project-level skill directory.

## Invoking a skill

Include `/skill-name` anywhere in your message and Pegg detects it before the
model runs, strips the token, and issues a synthetic `loadskill` tool call. The
loop executes it like any other tool call:

- **Default context** — the tool returns the skill as a `<skill:...>` block that
  becomes a tool result, so the model sees the full skill (description, paths,
  allowed tools, instructions) in its context:

  ```
  <skill:pdf-tools>
  Description: Extract text from PDF files. Use when handling PDFs.
  Path: /home/user/.pegg/skills/pdf-tools
  Scripts: /home/user/.pegg/skills/pdf-tools/scripts
  Allowed tools: Bash(python:*), Read

  # PDF Tools
  ...
  </skill:pdf-tools>
  ```

- **`context: fork`** — the tool spawns a **background subagent** from a forked
  session containing the skill instructions. The agent can keep working; the system
  wakes it when the forked subagent completes. See
  [Subagents](../features/subagents.md).

Notes:

- The match ignores URLs (`https://...`) and file paths (`a/b/c`).
- Unknown skill names are left untouched in the message.
- Skills can also be loaded by the agent itself with the `loadskill` tool
  (`name`, optional `source`, optional `arguments` substituted into `$arg_name`
  placeholders).

In the TUI, type `/` to open the skill picker and select a skill by name. The agent's
system prompt lists every available skill with its description, so it can also use
skills mentioned in a task.

## Discovering skills from the hub

When a task needs a capability that is not already loaded, the agent can search the
[Pegg hub](https://hub-api.pegg.dev/v1) with the `searchskill` tool and then load the
best match with `loadskill`:

1. Check the available skills list.
2. If nothing matches, call `searchskill` with a capability-focused query.
3. Call `loadskill` with the result's `skill_id`, or pass its full `ref`
   (`owner/repo/skill`) as `name`.

`loadskill` also performs this fallback automatically: if the requested name is not
loaded locally, it searches the hub, installs the best exact match into
`~/.pegg/skills/`, and loads it for the current conversation. Installed skills remain
available in future sessions.

Typical cases:

- The user uploads a `.docx` file or asks to create or edit a Word document, and no
  document skill is loaded — search `docx word document` and load the matching skill
  before touching the file.
- The user asks for an `.xlsx` spreadsheet or `.pptx` slide deck — search `xlsx
  spreadsheet` or `pptx slides`, then load.
- The user uploads a PDF or asks to fill or extract a PDF form — search `pdf forms`.
- The user asks for a polished frontend page and no design skill is loaded — search
  `frontend design`.

If several hub skills share the same name, pass the result's `source` (`owner/repo`)
or its full `ref` to `loadskill` to disambiguate. Searches that return no exact match
report candidate references so the agent can retry without guessing.

## Built-in skills

<div class="grid cards" markdown>

- [:lucide-file-plus:{ .lg } **`/init`**](../features/commands.md#init)

    Analyze a codebase and create or improve an `AGENTS.md` file, with a fixed
    section structure and a token budget.

- [:lucide-list-checks:{ .lg } **`/batch`**](../features/commands.md#batch)

    Orchestrate a large, parallelizable change: explore, plan, delegate to
    subagents, verify, and report.

- [:lucide-workflow:{ .lg } **`/orchestrate`**](../features/commands.md#orchestrate)

    The general plan-first contract for large projects: explore, write the plan
    yourself, create todos, delegate, verify, and report.

- [:lucide-git-pull-request:{ .lg } **`/review`**](../features/commands.md#review)

    Run a structured pull-request review through explorer, developer, and verifier
    phases.

- [:lucide-shield:{ .lg } **`/rule`**](../features/commands.md#rule)

    Create or update a rule file under `.pegg/rules/` (project) or
    `~/.pegg/rules/` (global), asking you for the content when not specified.

</div>

See [Commands](../features/commands.md) for the full descriptions.

## Creating your own

1. Create a directory under `~/.pegg/skills/` (or `~/.agents/skills/`) with a
   kebab-case name:

   ```bash
   mkdir -p ~/.pegg/skills/release-notes
   ```

2. Add a `SKILL.md` with frontmatter and instructions.

3. Restart Pegg. The skill appears in the TUI picker and in the agent's
   **Available Skills** list.

## Viewing loaded skills

Open Settings with ++ctrl+p++ and switch to the **Skills** tab. The list shows each
skill's name, description, and source directory. There is no create/edit UI — manage
skills on disk.

## Tips

- Write the `description` as a trigger: *"Use when ..."* helps the model decide when
  to load the skill.
- Keep the body procedural. Reference scripts by relative path so the expansion's
  `Scripts:` location stays meaningful.
- Use `allowed-tools` to document expected tool usage; it is informational and does
  not restrict the agent.
