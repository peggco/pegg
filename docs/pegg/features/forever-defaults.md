---
icon: lucide/cloud-download
---

# Forever defaults

After `pegg login-pegg` succeeds, Pegg can fetch a recommended defaults document
from `https://cdn.pegg.dev/defaults.json` and apply it to `~/.pegg/pegg.json`.

Pegg always shows the proposed changes first. Nothing is written until you
confirm.

## Consent

| Situation | Behavior |
|---|---|
| Terminal, no flag | Prints the changes and asks `Apply these defaults? [Y/n]` |
| TUI first-run login or API key | Shows the changes and offers **Apply recommended defaults** / **Skip for now** |
| Desktop first-run login or API key | Shows the changes and offers **Apply recommended defaults** / **Skip for now** |
| Non-interactive (scripts, CI) | Prints the changes and the `--defaults` hint, then skips |
| `--defaults` | Applies without asking |
| `--no-defaults` | Skips the document entirely |

The document is fetched with a 5 second timeout and the login still succeeds if
the CDN is unreachable. The endpoint can be overridden with `--defaults-url` or
the `PEGG_DEFAULTS_URL` environment variable.

## Document format

The file is versioned and every field is optional except `version`. A provider
that is not configured on the machine is skipped instead of being written.

```json
{
  "version": 1,
  "updated_at": "2026-11-15T00:00:00Z",
  "agents": {
    "orchestrator": {
      "provider": "pegg-forever",
      "model": "GLM-5.3",
      "fallback_provider": "pegg-forever",
      "fallback": "deepseek-ai/DeepSeek-V4.1-Flash-1017"
    },
    "explorer": {
      "provider": "pegg-forever",
      "model": "deepseek-ai/DeepSeek-V4.1-Flash-1017",
      "fallback_provider": "pegg-forever",
      "fallback": "zai-org/GLM-5.3-Flash"
    },
    "developer": {
      "provider": "pegg-forever",
      "model": "GLM-5.3",
      "fallback_provider": "pegg-forever",
      "fallback": "moonshotai/Kimi-K3"
    }
  },
  "smart_router": {
    "enabled": true,
    "provider": "pegg-forever",
    "decision_model": "typesafe/jev-1.14",
    "agents": {
      "orchestrator": {
        "default": [
          "GLM-5.3",
          "deepseek-ai/DeepSeek-V4.1-Flash-1017",
          "moonshotai/Kimi-K3"
        ],
        "difficulty": {
          "trivial": [
            "deepseek-ai/DeepSeek-V4.1-Flash-1017",
            "zai-org/GLM-5.3-Flash"
          ],
          "moderate": [
            "GLM-5.3",
            "deepseek-ai/DeepSeek-V4.1-Flash-1017"
          ],
          "complex": [
            "moonshotai/Kimi-K3",
            "GLM-5.3",
            "deepseek-ai/DeepSeek-V4.1-Flash-1017"
          ]
        },
        "images": [
          "moonshotai/Kimi-K3",
          "deepseek-ai/DeepSeek-V4.1-Flash-1017",
          "Qwen/Qwen3.8-27B"
        ]
      },
      "explorer": {
        "default": [
          "deepseek-ai/DeepSeek-V4.1-Flash-1017",
          "zai-org/GLM-5.3-Flash"
        ],
        "difficulty": {
          "trivial": [
            "zai-org/GLM-5.3-Flash",
            "deepseek-ai/DeepSeek-V4.1-Flash-1017"
          ],
          "moderate": [
            "deepseek-ai/DeepSeek-V4.1-Flash-1017",
            "zai-org/GLM-5.3-Flash"
          ],
          "complex": [
            "deepseek-ai/DeepSeek-V4.1-Flash-1017",
            "GLM-5.3"
          ]
        }
      },
      "developer": {
        "default": [
          "GLM-5.3",
          "moonshotai/Kimi-K3",
          "deepseek-ai/DeepSeek-V4.1-Flash-1017"
        ],
        "difficulty": {
          "trivial": [
            "deepseek-ai/DeepSeek-V4.1-Flash-1017",
            "zai-org/GLM-5.3-Flash"
          ],
          "moderate": [
            "GLM-5.3",
            "deepseek-ai/DeepSeek-V4.1-Flash-1017"
          ],
          "complex": [
            "GLM-5.3",
            "moonshotai/Kimi-K3",
            "deepseek-ai/DeepSeek-V4.1-Flash-1017"
          ]
        }
      }
    }
  },
  "stt": {
    "provider": "pegg-forever",
    "model": "whisper-large-v3-turbo"
  },
  "judge": {
    "provider": "pegg-forever",
    "model": "typesafe/jev-1.14",
    "threshold": 0.6
  },
  "memory": {
    "enabled": true,
    "gate": "decision",
    "threshold": 0.6,
    "observer": {
      "provider": "pegg-forever",
      "model": "deepseek-ai/DeepSeek-V4.1-Flash-1017"
    },
    "relevance": {
      "provider": "pegg-forever",
      "model": "typesafe/jev-1.14"
    }
  },
  "compaction": {
    "summarizer_provider": "pegg-forever",
    "summarizer_model": "deepseek-ai/DeepSeek-V4.1-Flash-1017"
  }
}
```

### Fields

| Field | Maps to | Notes |
|---|---|---|
| `agents.<name>.provider` / `.model` / `.fallback_provider` / `.fallback` | `agent_defaults.<name>` | Supported names: `orchestrator`, `explorer`, `developer`. `provider` and `fallback_provider` default to the login provider |
| `smart_router.enabled` | `smart_router.enabled` | |
| `smart_router.provider` / `decision_model` | `smart_router.provider` / `smart_router.model` | Use `pegg-forever` (or another decision provider) for decision models |
| `smart_router.agents.*` | `smart_router.agents.*` | `default` is an ordered preference list |
| `stt.provider` / `model` / `streaming_model` | `stt.*` and `stt.providers[]` | The provider entry shares the login credentials |
| `judge.provider` / `model` / `threshold` | `permission.judge_*` | |
| `memory.enabled` / `gate` / `threshold` | `memory.*` | |
| `memory.observer` | `memory.observer` | Observer LLM model |
| `memory.relevance` | `memory.relevance` | Decision model for relevance gating |
| `compaction.summarizer_provider` / `summarizer_model` | `compaction.summarizer_*` | Adds the `summarization` strategy when absent |

The `provider` in `smart_router`, `stt`, `judge`, `memory.observer`,
`memory.relevance` and `compaction` is optional. When omitted, it defaults to
the provider that was just configured (`pegg-forever`). Decision models and STT
reuse the `pegg-forever` login credentials, so there is no separate login step.

## Team AI

The same document is the source for a new team's Team AI defaults. When a team
is provisioned, the service fetches the file once, keeps only model references
that exist in the team's catalog, and stores the result. Existing defaults are
never overwritten. Set `PEGG_DEFAULTS_SEED=false` on the service to disable
seeding, or `PEGG_DEFAULTS_URL` to point at a different file.
