<div align="center">

<img src="https://cdn.pegg.dev/logo/infinity.gif" alt="Pegg" width="560" />

# Pegg

### Your models. Your machine. Your rules.

Pegg is a provider-agnostic AI coding agent in a single Go binary.
Run it in your terminal, your editor, CI, or your own product — with the
models and keys you already trust.

[Website](https://pegg.dev) · [Docs](https://docs.pegg.dev) · [Interfaces](https://pegg.dev/interfaces) · [Discussions](https://github.com/peggco/pegg/discussions)

[![Latest release](https://img.shields.io/github/v/release/peggco/pegg?style=flat-square&sort=semver)](https://github.com/peggco/pegg/releases)
[![License: FSL-1.1-MIT](https://img.shields.io/badge/license-FSL--1.1--MIT-black?style=flat-square)](LICENSE.md)
[![Go 1.26.5+](https://img.shields.io/badge/go-1.26.5%2B-00ADD8?style=flat-square&logo=go&logoColor=white)](go.mod)
[![Platforms](https://img.shields.io/badge/platforms-Linux%20%7C%20macOS%20%7C%20Windows-black?style=flat-square)](#install)

<sub><strong>36</strong> providers · <strong>26</strong> themes · <strong>5</strong> permission modes · <strong>1</strong> binary · <strong>0</strong> runtime deps</sub>

**Install in one line**

```bash
curl -fsSL https://pegg.dev/install | bash
```

</div>

```text
$ pegg
› Refactor the auth module and add tests

● orchestrator · claude-sonnet-4-5
  plan   1. map the auth flow
         2. rotate refresh tokens
         3. cover expiry cases

  bash   go test ./internal/auth
  ✓      41 passed

  edit   internal/auth/token.go   +18 −6
  ✓ 6 files changed · 2m 14s · $0.031
```

> **Use the models you already pay for. Keep your code on your machine.**
> Give the agent enough power to be useful, and enough guardrails to trust it.

---

<div align="center">
<sub>01 · WHY PEGG</sub>

## Not another chat wrapper.
</div>

<table>
<tr>
<td width="50%" valign="top">

### Your models, your keys

36 providers, custom endpoints, local runtimes, and your existing Claude Code,
Codex, Cline, or ClinePass subscription. No hosted model proxy.

</td>
<td width="50%" valign="top">

### Local-first by design

Sessions, files, memory, and shell execution stay on your machine. Prompts go
directly from you to the provider you chose.

</td>
</tr>
<tr>
<td width="50%" valign="top">

### One engine, every surface

The same agent loop, tools, sessions, and permissions in the terminal, editors,
CI, HTTP/ACP, the SDK, and GitHub Actions.

</td>
<td width="50%" valign="top">

### Powerful, but permissioned

Five permission modes per tool, an LLM judge, a sandboxed VFS, secret redaction,
and a double-`Esc` kill switch.

</td>
</tr>
</table>

---

<div align="center">
<sub>02 · SURFACES</sub>

## One engine. Every surface.
</div>

Pick the surface that fits the task. Keep the same sessions everywhere.

<table>
<tr>
<td width="50%" valign="top">

**[TUI](https://pegg.dev/tui)**  
The flagship terminal workspace: streaming markdown, tool cards, diffs,
subagent transcripts, 26 themes, and live cost tracking.

<sub>`pegg`</sub>

</td>
<td width="50%" valign="top">

**[CLI](https://pegg.dev/cli)**  
One-shot prompts, piped stdin, session resumption, and clean output for scripts
and CI.

<sub>`pegg run "…"`</sub>

</td>
</tr>
<tr>
<td width="50%" valign="top">

**[VS Code](https://pegg.dev/vscode)**  
Sidebar chat, the `@pegg` participant, editor actions, diagnostics fixes, and
working-tree review.

<sub>`code --install-extension peggco.pegg-vscode`</sub>

</td>
<td width="50%" valign="top">

**[JetBrains](https://pegg.dev/jetbrains)**  
The same chat UI in a tool window across IntelliJ IDEA, PhpStorm, WebStorm,
PyCharm, CLion, and Android Studio.

<sub>[IntelliJ guide](https://docs.pegg.dev/pegg/usage/intellij/)</sub>

</td>
</tr>
<tr>
<td width="50%" valign="top">

**[Go SDK](https://pegg.dev/sdk)**  
Embed the engine and register your own tools, middlewares, agents, and
providers.

<sub>`pkg/sdk`</sub>

</td>
<td width="50%" valign="top">

**[PeggBot](https://pegg.dev/peggbot)**  
Solve labeled issues, review PRs with inline comments, answer `@peggbot`, and
write release notes from GitHub Actions.

<sub>`peggbot/peggbot@v1`</sub>

</td>
</tr>
<tr>
<td width="50%" valign="top">

**[Connect](https://pegg.dev/connect)**  
Pair a machine and chat with your local agent from any browser. Execution stays
local.

<sub>`pegg connect`</sub>

</td>
<td width="50%" valign="top">

**[HTTP API & ACP](https://docs.pegg.dev/pegg/usage/http/)**  
REST with SSE streaming, plus JSON-RPC over stdio, HTTP+SSE, or WebSocket for
editor integrations.

<sub>`pegg serve`</sub>

</td>
</tr>
</table>

---

<div align="center">
<sub>03 · CAPABILITIES</sub>

## Enough power to finish the job.
</div>

<table>
<tr>
<td width="50%" valign="top">

**Reads the repository.** Project structure, dependencies, git state, and code
references with `file:line` precision. It opens what it needs instead of
guessing.

**Edits with reviewable diffs.** Every change is shown before and after. Revert
what you do not want, or fork the session and try another path.

**Runs the loop.** Installs dependencies, runs tests, starts servers, reads the
failures, and fixes them. Long-running processes stay in the background.

</td>
<td width="50%" valign="top">

**Plans before it acts.** Build, Plan, and Auto modes let the agent explore and
strategize before it touches a file. Approval gates stay on every change.

**Delegates to specialists.** The orchestrator spawns explorer, planner, and
developer subagents that run concurrently, persist across turns, and can be
resumed by name.

**Keeps the context.** SQLite sessions with token and cost tracking, live run
attach across clients, fork/revert, and compaction that preserves the original
history.

</td>
</tr>
</table>

---

<div align="center">
<sub>04 · CONTROL</sub>

## Permissioned by design.
</div>

<table>
<tr>
<td width="50%" valign="top">

**Five modes per tool**  
`ask`, `semi-ask`, `judge`, `semi-judge`, and `allow`.

**A judge for risky work**  
A decision model or judge LLM scores bash and write operations against your
threshold.

**Remembered decisions**  
Allow or reject once; the choice persists in `~/.pegg/permissions.json`.

</td>
<td width="50%" valign="top">

**A real sandbox**  
The VFS is rooted at the workspace, respects `.gitignore` and `.peggignore`, and
refuses out-of-bounds paths.

**Protected config**  
`.pegg/` is off-limits to the developer agent. Planners only write plans;
explorers are read-only.

**Emergency stop**  
Double-`Esc` cancels the run and every subagent.

</td>
</tr>
</table>

---

<div align="center">
<sub>05 · MODELS</sub>

## 36 providers. Zero proxy.
</div>

Pegg talks directly to provider APIs with your credentials.

<table>
<tr>
<td width="50%" valign="top">

**32 API-key providers**

Anthropic, OpenAI, Google, xAI, Mistral, Cohere, DeepSeek, Groq, Cerebras,
Fireworks, Together, OpenRouter, NVIDIA, Perplexity, and more.

**4 subscription providers**

Reuse credentials from Claude Code, Codex, Cline, and ClinePass.

</td>
<td width="50%" valign="top">

**5 speech-to-text providers**

AssemblyAI, Fireworks, Groq, OpenAI, and Together.

**Any compatible endpoint**

Point a generic `openai`, `claude`, or `gemini` driver at a gateway, proxy,
Ollama, LM Studio, vLLM, or your own runtime.

</td>
</tr>
</table>

Models are discovered from the provider and enriched with context window,
pricing, modalities, tool-call support, and reasoning options. A **Smart Router**
can pick per-agent and per-difficulty models when you want it to.

---

<div align="center">
<sub>06 · EXTEND</sub>

## Extend without forking.
</div>

<table>
<tr>
<td width="50%" valign="top">

**Skills**  
`SKILL.md` bundles loaded on demand with `/name`, with optional `context: fork`
background execution.

**Rules**  
Global and project Markdown injected into every system prompt.

**MCP**  
Servers over stdio or SSE; tools appear as `server__tool` on every surface.

</td>
<td width="50%" valign="top">

**LSP**  
30+ bundled language servers inject diagnostics into every file read and write.

**Plugins**  
Separate Go binaries over HashiCorp RPC with full tool and event access.

**SDK**  
Register tools, seven middleware hooks, agents, providers, and skills inside
your own Go product.

</td>
</tr>
</table>

---

<div align="center">
<sub>07 · AUTOMATION</sub>

## It remembers. It runs on your schedule.
</div>

<table>
<tr>
<td width="50%" valign="top">

**Project memory**  
An observer distills each run into markdown observations. Future runs get a
relevance-gated, budget-bounded memory panel, searchable with `mem-search` and
`mem-read`.

**Cron schedules**  
Daily summaries, weekly dependency audits, recurring reviews. Each run gets its
own session, workspace, model override, and tags.

</td>
<td width="50%" valign="top">

**System reminders**  
Agents stay aware of context usage, background subagent results, and standing
notes without you repeating yourself.

**Voice and notifications**  
Streaming voice input across surfaces, plus focus-gated desktop notifications
for permissions, asks, completions, and failures.

</td>
</tr>
</table>

---

<div align="center">
<sub>08 · TEAMS</sub>

## Your team's models. Your team's rules.
</div>

<table>
<tr>
<td width="50%" valign="top">

### Team AI

Centralize provider keys for the team. Owners add encrypted BYOK providers and
defaults on the website; members run `pegg team` and use a team-scoped provider.

Usage, tokens, and cost are tracked by provider and model.

</td>
<td width="50%" valign="top">

### Connect

Pair a machine with the website so teammates can chat from a browser while
prompts, files, and shell execution stay local.

Device scopes, revocation, and role-based access are built in.

</td>
</tr>
</table>

---

<div align="center">
<sub>09 · INSTALL</sub>

## Up and running in a minute.
</div>

**macOS / Linux**

```bash
curl -fsSL https://pegg.dev/install | bash
```

**Windows**

```powershell
irm https://pegg.dev/install.ps1 | iex
```

<details>
<summary>npm, Go, and prebuilt binaries</summary>

```bash
npm i -g @peggco/pegg
go install github.com/peggco/pegg/cmd/pegg@latest
```

Prebuilt archives for Linux, macOS, and Windows on `amd64` and `arm64` are
published on the [releases page](https://github.com/peggco/pegg/releases), with
`checksums.txt` for verification.

</details>

### First run

```bash
pegg login   # pick a provider and paste your API key
pegg         # open the TUI
```

No runtime dependencies. No Docker required. Linux, macOS, and Windows, on
`amd64` and `arm64`.

---

<div align="center">

<sub>10 · GO DEEPER</sub>

## Read the manual. Or skip it.

Everything you need to get started, customize, and go further.

</div>

<br>

### Start here

The essentials to get up and running.

- **[Documentation](https://docs.pegg.dev)** — Explore the full documentation
- **[Installation](https://docs.pegg.dev/pegg/installation/)** — Set up Pegg in minutes
- **[TUI Guide](https://docs.pegg.dev/pegg/usage/tui/)** — Navigate the terminal interface
- **[CLI Reference](https://docs.pegg.dev/pegg/usage/cli/)** — Master the command line
- **[Troubleshooting](https://docs.pegg.dev/pegg/troubleshooting/)** — Find answers and fixes

<br>

### Go deeper

Extend your workflow and unlock more capabilities.

**Integrations**
  
[VS Code](https://docs.pegg.dev/pegg/usage/vscode/) · [JetBrains](https://docs.pegg.dev/pegg/usage/intellij/) · [HTTP API](https://docs.pegg.dev/pegg/usage/http/) · [ACP](https://docs.pegg.dev/pegg/usage/acp/)

**Development**

[Go SDK](https://docs.pegg.dev/sdk/) · [PeggBot](https://docs.pegg.dev/peggbot/)

**Customization**

[Skills](https://docs.pegg.dev/pegg/configurations/skills/) · [Rules](https://docs.pegg.dev/pegg/configurations/rules/) · [MCP](https://docs.pegg.dev/pegg/configurations/mcp/) · [LSP](https://docs.pegg.dev/pegg/configurations/lsp/) · [Plugins](https://docs.pegg.dev/pegg/configurations/plugins/)

<br>

---

<div align="center">
<sub>11 · BUILD</sub>

## Built in Go. Yours to extend.
</div>

Requires Go 1.26.5+.

```bash
git clone https://github.com/peggco/pegg
cd pegg
make build     # ./bin/pegg
make test
make lint
```

<details>
<summary>Build the editor extensions</summary>

```bash
# VS Code
npm --prefix pkg/chat-ui ci
cd pkg/vscode
npm ci
npm run build

# JetBrains
npm --prefix pkg/chat-ui ci
cd pkg/intellij
npm ci
./gradlew buildPlugin
```

</details>

Contributions are welcome. Start with [CONTRIBUTING.md](CONTRIBUTING.md), open
an issue, or send a pull request.

---

## License

Pegg is source-available under the **Functional Source License 1.1 with an MIT
future license** (`FSL-1.1-MIT`). It converts to MIT after the change date. See
[LICENSE.md](LICENSE.md) for the exact terms.

If Pegg fits how you work, [star the repository](https://github.com/peggco/pegg) —
it helps other developers find it.

<div align="center">

**Use the models you already pay for. Keep your code on your machine.**

[Website](https://pegg.dev) · [Docs](https://docs.pegg.dev) · [GitHub](https://github.com/peggco/pegg) · [Discussions](https://github.com/peggco/pegg/discussions)

</div>
