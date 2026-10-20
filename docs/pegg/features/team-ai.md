---
icon: lucide/key-round
---

# Team AI

Team AI lets a team own its model access. Instead of each member adding an
upstream API key locally, a team owner stores provider keys on the website once.
Members then run `pegg team`, approve the connection in a browser, and receive a
team-scoped API key plus the team's defaults.

Requests go through the team's BYOK gateway using the team's keys. In pegg the
provider appears as **`pegg-team <team>`**, for example `pegg-team Acme`.

## Who can do what

| Permission | Allows |
|---|---|
| `team.ai.providers.view` | See providers, models and the Models page |
| `team.ai.providers.manage` | Add, edit, verify, sync or delete providers and models |
| `team.ai.defaults.view` | See agent, smart-router and STT defaults |
| `team.ai.defaults.manage` | Change those defaults |

Team owners always have every team permission.

## Set up the team (website)

Open **Team AI** in the sidebar (`/app/team-ai`). The page has four tabs.

### Providers

Add the upstream accounts the team owns:

- **Named presets** cover OpenAI-compatible providers such as OpenAI, DeepSeek,
  OpenRouter, Groq, Together, Moonshot, Fireworks, Cerebras, Mistral, Google
  Gemini (OpenAI-compatible), xAI and others. Named presets use their built-in
  endpoint; the OpenAI preset also lets you change the base URL for a compatible
  gateway.
- **AssemblyAI** is available for speech-to-text.
- **Custom** asks for the driver (`OpenAI-compatible` or `AssemblyAI`), base URL,
  timeout and optional headers.

The API key is encrypted at rest and is never returned to the browser. Only a
short key hint is shown afterwards. Providers can be verified, enabled/disabled
or deleted; deleting a provider removes its model entries.

A provider can be marked **Decision** and expose decision models. The smart
router uses those models as its decision model.

### Models

After a provider is added, pegg fetches its `/models` endpoint and **includes
every returned model by default**.

- **Sync all models** refreshes the catalog. Existing exclusions are preserved.
- **Exclude all** removes every model from the CLI catalog; **Include all** adds
  them back.
- Toggle **Included** per model to exclude or restore it.
- The **model key** is what pegg sends; the **upstream model** is what the
  provider receives. Metadata can describe context size, tool/vision support and
  reasoning.

**Decision** models are used by the smart router. If a team has no decision
models, the router falls back to the LLM model list.

### Defaults

- **Agent defaults** — default and fallback model for `orchestrator`,
  `developer`, `explorer` and `planner`.
- **Smart router** — enable it, choose a decision model, and pick preferred,
  vision and per-difficulty models. The model pickers are multi-select over all
  enabled LLM models.
- **Speech to text** — default STT model and an optional streaming model.

`pegg team` writes these into the global `~/.pegg/pegg.json`; projects can still
override them.

### Usage

The **Usage** tab shows team BYOK traffic: requests, tokens, success rate,
upstream cost, breakdowns by provider/model/kind/status, and recent requests.
Logs are written by the team gateway to the shared log database.

## Connect a machine

```bash
pegg team
```

1. The CLI opens the website's **Connect this computer** page.
2. The page selects the user's **current team** by default; an explicit
   `pegg team --team <name>` takes priority.
3. Approving creates or reuses a team-scoped API key assigned to that user.
4. The CLI validates the key, fetches the team's models and defaults, and saves
   the provider as `pegg-team <team>`.

In [VS Code](../usage/vscode.md), run **Pegg: Connect Team** from the command
palette. It opens an integrated terminal and runs `pegg team`. After connecting,
run **Pegg: Restart Server** so the extension reloads the team provider.

The API key is reused for later connections to the same team and user. It is
stored encrypted on the server; revoking it on the website forces a new key on
the next connection. Keys created directly on the website cannot be reused
automatically because their raw value is never stored.

```bash
pegg team --team Acme        # preselect by name
pegg team --no-browser       # print the URL instead of opening a browser
pegg team --port 8765        # fixed local callback port (useful over SSH)
pegg team --refresh          # refresh defaults without creating a key
pegg team list               # list connected team providers
pegg team remove "pegg-team Acme"
```

| Flag | Description |
|---|---|
| `--team` | Team name to preselect on the website |
| `--provider` | Team provider to refresh |
| `--app-url` / `--api-url` | Override the website or API URL |
| `--device` / `--key-name` | Device label and API key name |
| `--port` | Local callback port; `0` chooses a free port |
| `--no-browser` | Print the approval URL |
| `--refresh` | Refresh defaults from the team gateway without a new key |
| `--print` | Print the merged pegg config after connecting |
| `--print-secret` | Include API keys in `--print` output |
| `--force` | Save even if the provider check fails |

## SSH and remote servers

The browser callback is a loopback URL on the machine running `pegg team`, so a
local browser cannot reach it without forwarding.

On the server:

```bash
pegg team --no-browser --port 8765
```

On the laptop:

```bash
ssh -L 8765:127.0.0.1:8765 user@server
```

Open the printed URL, approve, and the callback reaches the server over SSH. If
the session is already open, add the forward with the OpenSSH escape:

```text
~C
-L 8765:127.0.0.1:8765
```

The forwarded port must match `--port`. The server must also be able to reach the
website API (`--api-url`) and the team service URL (`TEAM_SERVICE_URL`). If the
team service runs on your laptop, add a reverse tunnel or use a public URL.

As an alternative, connect once locally, copy `~/.pegg/pegg.json` to the server,
and run `pegg team --refresh` there. Back up the server's config first and make
sure the provider's base URL is reachable from the server.

## Security

- Provider keys are encrypted at rest and never returned to the browser.
- CLI API keys are team-scoped, assigned to the user, and revocable from
  **API Keys** on the website.
- The callback listens on loopback only. Do not expose it publicly; use an SSH
  tunnel instead.
- BYOK usage is not metered against a subscription; the **Usage** tab is for
  visibility.

## Related

- [CLI reference](../usage/cli.md)
- [Providers & Models](../providers-and-models.md)
- [Troubleshooting](../troubleshooting.md)
