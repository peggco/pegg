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
  (image, audio, video, file). Focus it with ++tab++, navigate with ++left++/++right++, and
  remove with ++backspace++/++delete++.
- Attachments are sent with your next message and are persisted with it, so they
  reappear as chips when you resume the session.

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

Typing `@` in the input opens the mention picker. It offers:

- **Agents** — subagent types you can address in your message.
- **Files and folders** — discovered by scanning the workspace.
- **Attachments** already in the attachment bar.

Mentions are inserted as atomic chips and sent to the model as literal `@name`
text. Unlike skills, mentions are **not** expanded by Pegg — the model decides how
to use the reference (for example `@internal/server` prompts it to look at that
directory).

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