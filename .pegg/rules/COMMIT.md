# Commit Message Rules

## Format

Use Conventional Commits:

```
<type>(<scope>): <summary>
```

- `feat(desktop): add update button for installed plugins`
- `fix(cli): prompt for base URL before API key in providers add`
- `docs: add datacenter support to kanban-jira docs`
- `ci(release): fix macOS/Cloudsmith/gh failures`

## Subject only

1. **Never add a body or description.** One line per commit, always.
2. **Never add footers**, including `Fixes #123`, `Closes #123`, `BREAKING CHANGE:`,
   and co-author trailers.
3. Reference issues in the pull request description, not in the commit message.
4. Squash merges may append `(#NNN)` to the subject; that is added by the host and is
   acceptable.

## Summary line

1. Use imperative mood: `add`, `fix`, `remove`, `refactor` — not `added` or `adds`.
2. Start with a lowercase letter.
3. Do not end with a period.
4. Keep the whole subject at or under about 72 characters.
5. Describe the change, not the files touched.

## Types

Use one of:

- `feat` — new user-facing feature
- `fix` — bug fix
- `refactor` — behavior-preserving code change
- `docs` — documentation only
- `test` — tests only
- `chore` — maintenance, dependencies, version bumps
- `ci` — CI/CD configuration
- `perf` — performance improvement

## Scopes

Add a scope in parentheses when the change is confined to one area:

- `internal/<pkg>` — core packages, e.g. `internal/agent`, `internal/llm`,
  `internal/tui`
- `desktop` — `pkg/desktop/`
- `cli` — command-line interface
- `tui` — terminal interface
- `vscode` — `pkg/vscode/`
- `intellij` — `pkg/intellij/`
- `acp` — Agent Client Protocol
- `docs` — documentation site

Also common: `release`, `plugin`, `sdk`, `skill`, `session`, `router`, `update`.

Omit the scope for changes that span the whole repository, such as `chore: version
update to 0.2.6`.

## Commits

1. One logical change per commit.
2. Do not mix refactoring with behavior changes.
3. **Exception:** include a body only when the user explicitly asks for one.
