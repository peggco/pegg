---
icon: lucide/mic
---

# Speech-to-Text

The SDK exposes the same STT providers and model selection as the TUI, CLI, and
HTTP server.

## Configuration

STT uses the `stt` block from `pegg.json` (or `Options.Config`):

```json
{
  "language": "en",
  "stt": {
    "provider": "assemblyai",
    "model": "universal-3-6-pro",
    "max_duration_secs": 120,
    "providers": [
      { "provider": "assemblyai", "api_key": "..." }
    ]
  }
}
```

- `provider` + `model` select the active pair.
- `providers` stores API keys for any number of STT providers.
- Custom providers can set `driver` (for example `openai`) plus `base_url` to
  reuse a registered driver for an OpenAI-compatible endpoint.
- Selecting a model enables STT; there is no separate enabled flag.
- The spoken language follows the top-level `language`.
- If the selected model supports streaming, `TranscribeStream` is available.

## Transcribe

```go
res, err := eng.Transcribe(ctx, sdk.STTRequest{
	Audio:     audioBytes,
	MediaType: "audio/webm",
	FileName:  "voice.webm",
})
fmt.Println(res.Text)
```

| Field | Description |
|---|---|
| `Audio` | Raw audio bytes |
| `MediaType` | MIME type, e.g. `audio/webm`, `audio/wav` |
| `FileName` | Optional original file name |

`STTResponse` contains `Text`, `Provider`, and `Model`.

## Streaming

```go
err := eng.TranscribeStream(ctx, pcmReader, func(ev sdk.STTStreamEvent) error {
	if ev.Final {
		fmt.Println(ev.Text)
	} else {
		fmt.Print("\r", ev.Text)
	}
	return nil
})
```

`pcmReader` must yield 16 kHz mono signed 16-bit little-endian PCM. Each event
carries the current turn's `Text`; `Final` marks the end of a turn.

## Introspection

```go
eng.STTConfigured() // bool
eng.STTProviders()  // registered STT provider names
eng.STTModels()     // models for the selected provider, including stream support
```

`STTModels` returns `STTModel` values with `ID`, `Name`, and `Streaming` fields.

## Errors

- `sdk.ErrSTTNotConfigured` — no provider/model/API key is configured.
- `sdk.ErrSTTStreamUnsupported` — the selected model or provider cannot stream.
- Provider errors are returned from `Transcribe` / `TranscribeStream` unchanged.
