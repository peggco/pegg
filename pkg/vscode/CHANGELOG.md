# Changelog

## 0.1.9

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
