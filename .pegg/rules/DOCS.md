# Documentation & Changelog Rules

## Documentation

1. **Never finish a task without updating the docs.** After completing any change, check whether the relevant docs under `docs/` need updating and update them.
2. **Do not skip docs.** If a change affects behavior, configuration, CLI flags, protocol, tools, or user-facing features, the matching doc must be updated before the task is considered done.
3. **Only skip when truly irrelevant.** If no doc is affected, say so explicitly instead of silently moving on.

## Changelogs

1. **VS Code changes require a `pkg/vscode/CHANGELOG.md` entry.** Any change under `pkg/vscode/` (or shared `pkg/chat-ui/` code that affects it) must be recorded in the VS Code changelog.
2. **IntelliJ changes require a `pkg/intellij/CHANGELOG.md` entry.** Any change under `pkg/intellij/` (or shared `pkg/chat-ui/` code that affects it) must be recorded in the IntelliJ changelog.
3. **Never leave an empty changelog section.** Every version heading must have content; fill it from the relevant commits.
4. **Use the `Unreleased` section** for changes not yet tied to a released version.
5. **Match the existing format** of each changelog (heading style, bullet style, link refs) — do not reformat unrelated sections.
