---
icon: lucide/package-plus
---

# Plugins

Pegg supports plugins that extend its functionality. Plugins are standalone binaries
that run in separate processes and communicate with Pegg via RPC. A plugin can
contribute tools, skills, permission defaults, CLI commands, and event subscriptions.

## Configuration

Add plugin configuration to `~/.pegg/pegg.json`:

```json
{
  "plugins": {
    "enabled": true,
    "exclude": ["broken-plugin"],
    "options": {
      "browser-use": {
        "headless": true
      }
    }
  }
}
```

### Options

| Key | Type | Default | Description |
|---|---|---|---|
| `enabled` | boolean | `true` | Enable or disable the plugin system |
| `exclude` | string[] | `[]` | Plugin names or binary names to skip during loading |
| `options` | object | `{}` | Per-plugin configuration passed to the plugin |

### Plugin directory

Plugins are loaded from `~/.pegg/plugins/`. Pegg scans this directory at startup
for executable, extensionless files and attempts to load each one as a plugin.

```bash
mkdir -p ~/.pegg/plugins
```

## Installing from GitHub releases

```bash
pegg plugins install <owner>/<repo>
pegg plugins install <owner>/<repo> --version v1.2.3
pegg plugins install <owner>/<repo> --asset '*linux_amd64*' --binary bin/my-plugin
pegg plugins install <owner>/<repo> --name my-plugin --force
```

The installer selects an asset for the current OS/architecture, supports raw
binaries and `.tar`, `.tar.gz`, `.tgz`, and `.zip` archives, and installs the binary
atomically as `~/.pegg/plugins/<name>` (mode `0755`).

Asset selection prefers names containing the current OS/architecture and the
repository name. Use `--asset` (name or glob) and `--binary` (path inside an archive)
when a release does not follow that convention.

### Checksums

If the release provides a `checksums.txt`, `sha256sums.txt`, a per-asset
`*.sha256` file, or a GitHub `digest`, the download is verified. If no checksum is
available the install continues with a warning. Use `--require-checksum` to refuse
unverified downloads or `--skip-checksum` to disable verification.

### Authentication and GitHub Enterprise

Set `GITHUB_TOKEN` or `GH_TOKEN` for authenticated API requests and higher rate
limits. Set `PEGG_GITHUB_API` to the API base URL for GitHub Enterprise.

### Metadata and updates

Installed plugins are tracked in `~/.pegg/plugins/.state/<name>.json` with the
repository, release tag, asset, checksum, and installation time.

```bash
pegg plugins list                 # Fast on-disk view with status
pegg plugins list --json
pegg plugins list --runtime       # Launch plugins for reported runtime metadata
pegg plugins update <name|owner/repo>
pegg plugins update --all --check
pegg plugins remove <name|owner/repo> --yes
```

`plugins update` only updates plugins installed through `pegg plugins install`.
Manually copied binaries remain manual and are shown with source `manual`.

## Enabling and disabling

```bash
pegg plugins exclude <name|owner/repo>
pegg plugins include <name|owner/repo>
```

Excluding writes the plugin binary name and repository to `plugins.exclude`;
including removes those entries. The legacy `pegg plugin toggle <name>` command is
still available.

## Lifecycle

1. At startup, after all core modules are initialized, Pegg scans the plugin
   directory for executables.
2. Each executable is launched as a separate process via
   [HashiCorp go-plugin](https://github.com/hashicorp/go-plugin).
3. The plugin's `Boot` method receives the application config and its
   `plugins.options` entry.
4. Tool bundles are activated lazily when a skill with `tools: <bundle>` is loaded.
5. If a plugin fails to load, a warning is logged and Pegg continues normally.
6. On shutdown, plugin processes receive a close request and are then terminated.

## Scaffolding

```bash
pegg plugins new my-plugin --module github.com/me/my-plugin
```

The scaffold creates a working plugin module with the public SDK, an example tool,
and build/install instructions.

## Troubleshooting

```bash
pegg plugins list                 # Check installed and excluded plugins
pegg plugins list --runtime       # Check loaded plugins and metadata
pegg logs --search plugin         # Check plugin-related logs
```

Common issues:

- **Plugin not appearing** — The executable must be in `~/.pegg/plugins/`, be
  extensionless, and have execute permission.
- **Plugin fails to load** — Check logs for the specific error. Common causes include
  missing dependencies, a protocol mismatch, or a missing shared library.
- **Plugin excluded** — Check the `exclude` list in your config or run
  `pegg plugins include <name>`.
- **Update unavailable** — Only installs created by `pegg plugins install` have the
  metadata required for updates.
