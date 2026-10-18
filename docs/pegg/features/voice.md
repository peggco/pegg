# Voice input

Pegg can record speech and insert the transcript into the chat input. The
feature is available through the TUI and the Connect web page. Speech-to-text is
configured on the machine that runs Pegg.

## Configure

Open **Settings → General** and use the **STT provider** and **STT model** rows.

- Selecting an unconfigured provider asks for its API key. Selecting a provider
  that already has a key asks whether to use the saved key or replace it.
- Each provider's key is saved in `pegg.json` under `stt.providers`, so multiple
  providers can be kept side by side.
- The model list contains every model from every configured provider.
- Streaming-capable models show a muted **stream** label on the right. Streaming
  is enabled automatically for those models.
- The spoken language follows the app language; there is no separate STT
  language setting.
- Voice input is available as soon as a provider and model are selected.
- `stt.max_duration_secs` can be set by hand in `pegg.json`.

Example `pegg.json` fragment:

```json
{
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

AssemblyAI defaults:

- Async model: `best`
- Streaming models: `universal-3-6-pro`, `universal-3-5-pro`,
  `universal-streaming-english`, `universal-streaming-multilingual`
- Streaming endpoint: `wss://streaming.assemblyai.com/v3/ws`

## TUI

Press `Ctrl+G` or click the `[mic Ctrl+G]` button on the chat input.

- Streaming models show live partial text above the input.
- Final turns are appended to the input at the cursor.
- Press `Ctrl+G` again to stop, or `Esc` to cancel.
- `sox`, `arecord`, or `ffmpeg` must be installed. The recorder is auto-detected.

## Connect web

The Connect composer has a microphone button.

- If the selected model supports streaming and the browser supports Web Audio,
  the page streams raw PCM and shows live partial text.
- Otherwise it records with `MediaRecorder` and sends one clip when you stop.
- The transcript is inserted at the cursor; it is not sent automatically.

## Proxy

The proxy exposes `POST /v1/audio/transcriptions` for OpenAI-compatible
providers and AssemblyAI. Plan rows use `kind = "stt"`; the model id in the
request must match the plan row. STT requests consume the shared plan budget and
may charge `overage_unit_price` when overage is allowed.

## CLI

Record a message and press Enter to stop:

```bash
pegg --stt
pegg run --stt
```

The transcript is used as the message; if a message argument is also given, the
transcript is appended to it.

## HTTP API

```http
POST /api/stt
Content-Type: application/json

{ "audio": "<base64>", "media_type": "audio/webm", "file_name": "voice.webm" }
```

Response:

```json
{ "text": "...", "provider": "assemblyai", "model": "universal-3-6-pro" }
```

## ACP

ACP clients can call the JSON-RPC method `stt/transcribe` with the same base64
audio fields. ACP prompts may also include an `audio` content block; Pegg
transcribes it when the prompt has no text block.

## SDK

```go
res, err := engine.Transcribe(ctx, sdk.STTRequest{
    Audio:     pcm,
    MediaType: "audio/webm",
})
```

For streaming models, use `engine.TranscribeStream(ctx, pcmReader, handler)`.
`engine.STTModels()`, `engine.STTConfigured()`, and `engine.STTProviders()` expose
the current STT configuration.
