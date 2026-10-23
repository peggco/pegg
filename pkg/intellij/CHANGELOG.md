# Changelog

## [Unreleased]

### Added

- IntelliJ Platform plugin with a Pegg tool window, ACP chat, sessions, tools,
  subagents, plans, permissions, attachments, voice input, and editor actions.
- Shared `pkg/chat-ui` renderer used by both the VS Code extension and the
  JetBrains plugin.
- Inline, searchable mode and model pickers anchored above the composer.
- Subagent cards with in-place transcripts; the orchestrator's `task` tool is
  hidden in favour of the subagent card.
- Sessions tab with search, resume, rename, delete, and refresh.
- Editor, project-view, and intention actions for explain, edit, ask, review,
  fix, tests, docs, and diagnostics.
- Checksum-verified CLI discovery and download, including workspace `bin/pegg`,
  PATH/profile directories, and the IDE config cache.
- Status bar widget, settings UI, logging, binary diagnostics, and restart.

### Fixed

- Chat tab blank state when the server state omitted empty collections.
- Theme-aware hover colors, font fallbacks, and scrollbars for JCEF.
- Mode/model pickers no longer open as separate windows or stay above the IDE.

## [0.2.0] - 2026-10-22

- Initial IntelliJ Platform plugin release with chat, sessions, tools,
  subagents, permissions, attachments, voice, editor actions, and binary
  management.

[Unreleased]: https://github.com/peggco/pegg/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/peggco/pegg/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/peggco/pegg/commits/v0.2.0
