---
icon: lucide/store
---

# Catalog

The Pegg catalog is a single index for **skills**, **MCP servers**, and **plugins**.
The CLI reads it from the hub API and installs entries locally.

```text
https://hub-api.pegg.dev/v1
  /skills       /mcp       /plugins
```

## Endpoint

| Setting | Description |
|---|---|
| `PEGG_CATALOG_API` | Catalog API base URL. Takes precedence over the legacy variable |
| `PEGG_SKILLS_API` | Legacy skills API base URL, still honored as a fallback |
| `GITHUB_TOKEN` / `GH_TOKEN` | Optional token used for catalog requests that need GitHub data |

The default is `https://hub-api.pegg.dev/v1`. Point it at a local hub API while
developing:

```bash
PEGG_CATALOG_API=http://localhost:8787/v1 pegg mcp search github
```

## MCP servers

```bash
pegg mcp search github
pegg mcp search github --official --limit 5
pegg mcp info github
pegg mcp install github
```

`mcp search` prints `ID NAME TRANSPORT OFFICIAL`; `--json` returns the raw catalog
entries. `mcp info` shows the vendor, repository, license, transport, required
environment variables, and the install command.

`mcp install` fetches the entry, resolves its `${VAR}` placeholders, and writes the
server to the global config by default. Use `--project` for the project
`.mcp.json`.

```bash
# Global (default)
pegg mcp install github --env GITHUB_TOKEN=ghp_...

# Project scope, custom server name
pegg mcp install github --project --name gh --env GITHUB_TOKEN=ghp_...

# Inspect the config without writing it
pegg mcp install github --dry-run
```

| Flag | Description |
|---|---|
| `--global` | Write to `~/.pegg/pegg.json` (default) |
| `--project` | Write to `.mcp.json` in the current directory |
| `--name` | Server name; defaults to the catalog id |
| `--env KEY=VALUE` | Value for a `${KEY}` placeholder (repeatable) |
| `--header KEY=VALUE` | Override a remote server header (repeatable) |
| `--dry-run` | Print the config JSON without writing |
| `--yes` | Skip the interactive confirmation |

## Plugins

```bash
pegg plugins search browser
pegg plugins search browser --json
pegg plugins info peggco/pegg-browser-use
pegg plugins install peggco/pegg-browser-use
```

Catalog plugin ids are `owner/repo`, exactly the value `plugins install` accepts, so
a search result can be installed directly. `plugins info <owner/repo>` reads catalog
metadata; `plugins info <name>` keeps showing the installed plugin's runtime details.

In Pegg Desktop, the Pegg Hub **Plugins** tab filters by the current operating system
automatically (`linux`, `darwin`, or `windows`). There is no manual platform
selector: the OS comes from the running desktop app and is sent with every plugin
search. The tab keeps the same `All / Official / Community` split used by skills and
MCP servers.

## Skills

```bash
pegg skills search pdf
pegg skills add owner/repo/skill
pegg skills list
```

The same `internal/catalog` client backs skills, MCP servers, and plugins. Skill
search, GitHub tree resolution, README fetching, install, list, and remove all live
in the catalog package.

## Entry format

Catalog entries are JSON objects served by the hub API.

### MCP entry

```json
{
  "id": "github",
  "name": "GitHub",
  "description": "Issues, pull requests, repositories and code search.",
  "repository": "github/github-mcp-server",
  "license": "MIT",
  "isOfficial": true,
  "version": "1.2.0",
  "tags": ["git", "issues"],
  "transport": {
    "type": "stdio",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-github"],
    "env": { "GITHUB_TOKEN": "${GITHUB_TOKEN}" }
  },
  "requiredEnv": ["GITHUB_TOKEN"],
  "readme": { "type": "github", "source": "github/github-mcp-server" }
}
```

Remote transports use a URL instead of a command:

```json
"transport": {
  "type": "http",
  "url": "https://example.com/mcp",
  "headers": { "Authorization": "Bearer ${API_TOKEN}" }
}
```

### Plugin entry

```json
{
  "id": "peggco/pegg-browser-use",
  "name": "browser-use",
  "description": "Drive a real Chromium browser through agent tools.",
  "repository": "peggco/pegg-browser-use",
  "license": "MIT",
  "isOfficial": true,
  "version": "0.3.1",
  "requires": {
    "pegg": ">=0.2.4 <0.3.0",
    "protocol": 2
  },
  "tags": ["browser"],
  "capabilities": ["tools", "skills"],
  "platforms": ["linux", "darwin"],
  "arch": ["amd64", "arm64"],
  "install": { "type": "github-release", "source": "peggco/pegg-browser-use" }
}
```

### Plugin compatibility

`requires.pegg` is a semver range for the Pegg CLI/desktop version and
`requires.protocol` is the plugin protocol version (`pkg/plugin.ProtocolVersion`,
currently `2`). Pegg compares them with the running version and prints a warning
when they are missing or do not match. Installation is **not blocked**; the user
can still install the plugin and is told that breakage is possible.

`plugins info` shows the declared requirements and any warnings. `plugins install`
and `plugins update` print the same warnings before continuing.

## Placeholders and secrets

MCP entries may use `${VAR}` placeholders in `command`, `args`, `env` values, `url`,
and header values. Every placeholder must also be listed in `requiredEnv`.

Resolution order for each key:

1. `--env KEY=VALUE` passed to `pegg mcp install`
2. The process environment
3. An interactive masked prompt when stdin is a terminal

Non-interactive runs fail with an actionable error listing the missing keys. Values
are written to the target config (mode `0600` for project files) and are never
printed. `--dry-run` replaces resolved values with their `${VAR}` placeholders so
you can inspect the structure safely.

## Publishing

Catalog entries live in [`peggco/hub-registry`](https://github.com/peggco/hub-registry).
Publishing is PR-first: a validated entry becomes a pull request, and merging it
regenerates the indexes on the CDN. There is no upload API.

```bash
pegg catalog publish --file ./github-mcp.json
pegg catalog publish --kind plugin --file ./browser-use.json
```

The command validates the entry locally and prints a GitHub new-file URL. Open the
URL, review the diff, and submit the pull request. Use `--kind` when the file omits
`kind`, or to catch a mismatch. The target repo and branch can be overridden with
`--repo` and `--branch`.

Entries need `schemaVersion: 1` and a matching `kind`. MCP entries also need
`transport` and `requiredEnv`; plugin `id` must equal `repository`; skill overlays
use `catalog/skills/<source>/<id>.json`.

## See also

- [MCP](configurations/mcp.md)
- [Plugins](configurations/plugins.md)
- [Skills](configurations/skills.md)
