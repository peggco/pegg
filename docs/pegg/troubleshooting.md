---
icon: lucide/bug
---

# Troubleshooting

## First checks

```bash
pegg version      # confirm what you are running
pegg files        # where config, logs, cache, and sessions live
pegg doctor       # test every configured provider and check for updates
pegg config show  # print the resolved config (API keys masked)
```

## Logs

Pegg logs to SQLite by default. Every command, request, tool call, and error is
recorded in `~/.pegg/logs.db`.

```bash
pegg logs                          # most recent 50 entries
pegg logs --level ERROR            # only errors
pegg logs --search "provider"      # substring search in message and level
pegg logs --level DEBUG --size 200 --page 2
```

If `logger.driver` is `console`, `pegg logs` prints only the current process's
in-memory buffer instead. Switch back to `"sqlite"` in
[Config](config.md#logger) to persist logs.

To increase verbosity, set the logger driver to `sqlite` (the default already records
debug-level events) and query with `--level DEBUG`.

## Cache

Model lists and model metadata are cached locally. Stale cache is the usual cause of
"model not found" after a provider adds new models.

```bash
pegg cache clear                        # clear everything
pegg cache clear --provider openai      # clear one provider
pegg providers refresh                  # re-fetch models from the network
pegg providers refresh --provider openai
```

Cache files: `~/.pegg/cache.db` (SQLite driver) or `~/.pegg/cache.json` (JSON
driver).

## Common problems

??? question "`cli: no providers configured` or an empty provider list"

    No provider has been added yet. Run `pegg login` or add an entry to the
    `providers` array in `~/.peggco/pegg.json`.

??? question "`cli: model \"x\" not found`"

    The model is not in the local cache. Refresh and try again:

    ```bash
    pegg providers refresh
    pegg models
    ```

    Model matching is case-sensitive and checks both the model ID and its display
    name.

??? question "`cli: timed out selecting model`"

    Model resolution waits 30 seconds for the LLM manager to become ready. This
    happens when providers are still syncing, no provider is reachable, or the cache
    is empty. Run `pegg doctor` to see which providers fail, then retry. Passing
    `--provider` and `--model` explicitly skips the selection step.

??? question "`pegg team` browser cannot connect to 127.0.0.1"

    The team callback is a loopback URL on the machine running pegg. Over SSH you
    must forward the same port to the server:

    ```bash
    # server
    pegg team --no-browser --port 8765

    # laptop
    ssh -L 8765:127.0.0.1:8765 user@server
    ```

    Open the printed URL and approve. If the SSH session is already open, use the
    OpenSSH escape `~C` followed by `-L 8765:127.0.0.1:8765`. The port must match
    `--port`, and the local port must be free. If `AllowTcpForwarding` is disabled
    on the SSH server, connect locally and copy the resulting
    `~/.pegg/pegg.json`, or use a public team service URL.

??? question "`pegg team` returns `provider check failed` or `connection refused`"

    The CLI reached the website but could not reach the team service returned as
    the provider base URL. Check `TEAM_SERVICE_URL` in the ginit environment and
    make sure the machine running `pegg team` can reach it. If the team service
    runs on your laptop, forward it with `ssh -R 8081:127.0.0.1:8081` or use a
    public URL.

    Also check that the website API is reachable from the same machine
    (`--api-url`).

??? question "`pegg team` returns `401` or `503 team_store_unavailable`"

    `401` means the team gateway rejected the API key. Retry after the embedded
    replica sync interval, or lower `DATABASE_SYNC_INTERVAL` in the team service
    environment. `503 team_store_unavailable` means the gateway's database or the
    `team_ai_*` tables are unavailable; run ginit migrations and check the team
    service logs.

??? question "`pegg team` keeps creating a new API key"

    The CLI reuses a key only when it can decrypt the stored per-team/user CLI
    credential. Revoking the key, rotating `BYOK_ENCRYPTION_KEY`, or deleting the
    key removes that ability. Connect again to create a new key. Website-created
    keys are not reused because only their hash is stored.

??? question "HTTP 401 / 403 from a provider"

    The API key is missing, expired, or lacks access to the model. Re-run
    `pegg login --provider <name>`, or check the key in `pegg config show`.

??? question "HTTP 429 or 5xx responses"

    Temporary errors are retried automatically with a 1s, 3s, 10s, 30s, 1m, 5m
    backoff. If the run keeps failing, wait for the rate limit to reset, switch to
    another provider, or lower concurrency.

??? question "`Model does not support image/audio attachments`"

    The selected model's metadata does not list that input modality (image, audio, or
    video). Choose a multimodal model, or remove the attachment. Elsewhere the file is
    still stored and handed to the agent by path, so it can read it with its tools.

??? question "A tool is blocked by a permission prompt or denied"

    Pegg asks before running gated tools (or consults the judge LLM, depending on
    the mode). Denials are remembered in `~/.pegg/permissions.json`, keyed by the
    tool call arguments. See [Permissions](features/permissions.md).

    To reset every remembered decision:

    ```bash
    rm ~/.pegg/permissions.json
    ```

??? question "An MCP server does not connect"

    Check the server definition and run Pegg with the `sqlite` logger, then inspect
    `pegg logs --search mcp`. Common causes:

    - The `command` is not on `PATH` (stdio transport).
    - The `url` is not reachable or does not emit an `endpoint` event (SSE transport).
    - The server fails its `initialize` handshake or lists no tools.

    Connection failures are logged as warnings and the server is skipped; the rest of
    Pegg keeps working.

    ```bash
    pegg mcp list
    pegg mcp tools --server db
    ```

??? question "No LSP diagnostics appear in file reads"

    Diagnostics require a language server whose `filetypes` match the file. Servers
    start lazily on first access and are stopped after 10 minutes idle. If the binary
    is missing, Pegg tries the configured `download` URL or `install` command.

    ```bash
    pegg lsp list
    pegg lsp diag --file src/main.go
    ```

    See [LSP](configurations/lsp.md).

??? question "Config file fails to parse"

    `~/.peggco/pegg.json` must be valid JSON. Validate it with your editor or
    `jq . ~/.peggco/pegg.json`. A missing file is fine; a malformed one aborts
    startup.

??? question "The TUI looks wrong or the terminal is unsupported"

    Try a different theme with `Ctrl+T` (the choice is saved). The TUI enables mouse
    and bracketed paste support when the terminal provides it; if the terminal
    misbehaves, resize the window or switch to the CLI with `pegg run`.

??? question "The agent keeps running and I want to stop it"

    Press ++esc++ twice within two seconds. The first press shows
    *Press Esc to interrupt*; the second cancels the run and all subagents.
    ++ctrl+c++ quits the whole application.

??? question "Sessions disappeared or moved"

    Sessions are filtered to the current project directory by default. List them all
    with:

    ```bash
    pegg sessions list --all
    ```

    Session storage lives in `~/.pegg/sessions.db` (SQLite driver) or
    `~/.pegg/sessions/` (JSON driver). See [Sessions](features/sessions.md).

## Reset to defaults

Remove pieces selectively, or everything:

```bash
rm ~/.pegg/permissions.json    # forget tool approvals
rm -rf ~/.pegg/vfs             # forget read-before-edit state
pegg cache clear               # rebuild model cache
rm ~/.peggco/pegg.json         # regenerate default config (loses providers)
rm -rf ~/.pegg                 # full reset: config, sessions, logs, cache
```

## Reporting issues

If the problem persists, open an issue at
[github.com/peggco/pegg/issues](https://github.com/peggco/pegg/issues) with:

1. The output of `pegg version` and your OS/architecture.
2. Relevant log lines (`pegg logs --level ERROR`).
3. A redacted copy of `pegg config show` (keys are masked already).
4. Steps to reproduce, including the exact command or prompt.
