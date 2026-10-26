# Changelog

## Unreleased

## 0.2.2

## 0.2.1

- The binary version check now reports why a candidate was rejected instead of
  a generic "failed its version check" message.
- The version probe runs inside the workspace instead of the extension host
  working directory, which can be `/` or the application bundle.
- Binary discovery also searches the well known install directories and the
  `PATH` declared in the shell profiles, so VS Code started from the Dock or
  the Finder finds an existing installation.
- `pegg.binary.version` set to `latest` now uses the release download endpoint
  instead of a tag that does not exist.
- New `Pegg: Diagnose Binary` command that lists every probed location, the
  detected version and the failure reasons.
- The Pegg process inherits the augmented `PATH`, so shell tools keep working
  in GUI launches.
- Tool cards follow the TUI: simple tools (`read`, `glob`, `grep`, `fetch`, …)
  collapse to one status line and stay collapsed unless they fail; raw JSON
  arguments are no longer shown, `bash` keeps its output and `edit`/`write`
  keep the diff button.
- Subagents render as their own card with the task and status. Clicking it
  opens the subagent chat (only that agent's messages) with a Back button.
  Internals of a subagent no longer leak into the main transcript. Requires the
  `pegg` CLI from this version (`toolName` and `subagent` updates).
- Click a user message to open a small menu with **Copy**, **Fork from here** and
  **Revert to here**, matching the TUI. Forking switches to a new session with
  the messages before that point; reverting removes the message and everything
  after it and puts the text back into the composer. Requires the
  `pegg.history` CLI capability.

## 0.2.0

- Initial VS Code extension release.
- Streaming ACP chat, sessions, modes, models, permissions, questions, plans,
  subagents, diffs, attachments, voice input, and slash commands.
- `@pegg` chat participant and editor, diagnostics, SCM, and status bar
  integrations.
- Automatic checksum-verified binary download with PATH and workspace overrides.
- Inline composer design with an autocomplete `@` file picker and `/` skill picker.
- Stable streaming message grouping, merged reasoning/content blocks, and visible command output in tool cards.
- Server-side voice recording through Pegg (no browser microphone permission).
- New `Pegg: Connect Team` command.
- New chats start a fresh session instead of resuming the last one automatically.
- Fixed the engine exiting after a prompt, so follow-up messages stay in the same session.
- Current sessions are marked in the sessions tree and resume picker.
- Editor right-click Pegg submenu, editor-title AI actions, selection/file quick picks, and new Ask/Edit/Document/Optimize/Security actions.
