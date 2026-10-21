---
icon: lucide/shell
---

# Adding Context

Pegg has several ways to give the agent more context than a single prompt: file
attachments, `@` mentions, project instructions, rules, and skills.

## Attaching files

### In the TUI

- **Paste a path** — pasting a single line that is an existing file path attaches
  that file. In a multi-line paste, every line that is a file path is attached.
- **Attachment bar** — attachments appear above the input as cards with type icons
  (image, audio, video, file). Focus it with ++shift+tab++, navigate with ++left++/++right++, and
  remove with ++backspace++/++delete++.
- Attachments are sent with your next message and are persisted with it, so they
  reappear as chips when you resume the session.

### In VS Code

- Use the `+` button, drag files from the explorer, or paste an image into the
  composer.
- Type `@` to open the inline file picker. It shows file-type icons, the file name,
  and the relative path.
- Use **Pegg: Add Selection to Chat** (or right-click → **Pegg**) to insert the
  current selection; with no selection it adds the current line.
- **Pegg: Add File to Chat** attaches the whole file.
- Editor actions such as Explain, Edit, Review, Refactor, Optimize, Document, and
  Generate Tests include the selection automatically.
- Voice input is recorded by the Pegg engine and inserted at the cursor.

See the [VS Code guide](../usage/vscode.md#chat).

### In the CLI

```bash
pegg run --file screenshot.png --file notes.md "What's wrong with my UI?"
```

`--file` is repeatable. The type is inferred from the extension:

| Extension | Type |
|---|---|
| `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`, `.bmp`, `.svg` | image |
| `.mp3`, `.wav`, `.ogg`, `.m4a` | audio |
| `.mp4`, `.webm`, `.mov` | video |
| everything else | file |

### Where attachments go

Every attachment is copied into the attachment store at `os.TempDir()/pegg`
(`/tmp/pegg` on Linux). The agent is told each file's name, media type, size, and
path in an `<attachments>` block appended to your message, so it can open the file
with its tools (for example `read`).

Only **media** the active model accepts (image/audio/video) is also sent to the
model inline. Everything else — PDFs, text, JSON, CSV, archives, and so on — is
**not** uploaded to the model; the agent reads it from the store path instead. This
keeps the request valid for chat-completions providers that reject arbitrary file
uploads. Because the block is part of the message, earlier attachments stay
available to the agent when you resume a session.

### Validation

Image, audio, and video are checked against the active model's input modalities
before being sent inline; media the model does not support is never uploaded. In the
TUI, unsupported images and audio are refused at attach time. Either way the file is
stored and the agent can still read it by path. Non-media files are always accepted
and referenced by path.

## `@` mentions

Typing `@` opens a picker.

=== "TUI"

    The TUI picker offers **agents** (subagent types), **files and folders**, and
    **attachments** already in the attachment bar. Mentions are atomic chips.

=== "VS Code"

    The VS Code picker lists workspace files with a file-type badge, file name, and
    relative path. It is fuzzy-filtered and keyboard navigable.

Mentions are sent to the model as literal `@name`/`@path` text. Unlike skills,
mentions are **not** expanded by Pegg — the model decides how to use the
reference (for example `@internal/server` prompts it to look at that directory).

## Long text

Pasting text longer than 100 characters inserts it as a collapsible **long text**
chip showing a short preview. The full text is sent with the message. Shorter
pastes are inserted character by character.

## Persistent context

Some context is always available to the agent, no matter how you phrase the prompt:

| Source | When it applies |
|---|---|
| [`AGENTS.md`](../configurations/rules.md#rules-vs-agentsmd) | Read from the current working directory, injected as **Project Instructions** |
| [`rules/*.md`](../configurations/rules.md) | Global (`~/.pegg/rules/`) and project (`.pegg/rules/`) rule files appended to the system prompt |
| [Skills](../configurations/skills.md) | Every skill is listed; including `/name` in a message injects its instructions |
| Git status | The current branch and dirty/clean state are included in the environment block |

## Injecting skills into context

Include `/skill-name` in any message or in a subagent task message to load the
skill's instructions. In the TUI, type `/` to pick a skill. See
[Skills](../configurations/skills.md#invoking-a-skill) and
[Commands](commands.md).

## Context and sessions

History accumulates across turns and is persisted per
[session](sessions.md). When you resume a session, its prior messages are replayed
into the model context, so context added earlier is still available.