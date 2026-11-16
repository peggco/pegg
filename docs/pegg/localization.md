---
icon: lucide/languages
---

# Localization

Pegg ships with English and Turkish interfaces for both the TUI and Pegg
Desktop. English is the source of truth: every user-facing string exists in the
English catalog first, and the Turkish catalog must contain the same keys with
non-empty, natural Turkish values.

## Where the catalogs live

| Surface | English | Turkish |
|---|---|---|
| TUI | `internal/tui/i18n/en.go` | `internal/tui/i18n/tr.go` |
| Desktop | `pkg/desktop/frontend/src/lib/i18n.ts` (`en`) | `pkg/desktop/frontend/src/lib/i18n.ts` (`tr`) |

Choose the language under **Settings → General → Language**. The value is saved
as `language` in `~/.pegg/pegg.json` and shared by the TUI and the desktop app.

## Glossary

Use one term per concept across both surfaces. When you need a new concept, add
it to this table and use it consistently.

| Concept | Turkish term | Avoid |
|---|---|---|
| plugin | eklenti | uzantı |
| skill | beceri | yetenek |
| session | oturum | seans |
| tool | araç | alet |
| permission | izin | müsaade |
| reasoning effort | akıl yürütme eforu | akıl yürütme seviyesi |
| revert | geri al | tersine çevir |
| fork | dallandır | çatalla |
| answer | yanıt | cevap |
| provider | sağlayıcı | sağlayıcı servis |
| model | model | — |
| catalog | katalog | dizin |
| hub | hub | merkez |
| judge model | yargıç modeli | değerlendirici model |
| async | eşzamansız | asenkron |
| stream | akış | yayın |
| context | bağlam | kontekst |
| subagent | alt ajan | alt temsilci |
| capability | yetenek | beceri |

`beceri` and `yetenek` are deliberately different words: a **skill** is a
loadable Pegg capability file, while **capabilities** in catalog metadata are
generic abilities an entry provides.

Keep product and protocol names as written: Pegg, Pegg Desktop, Pegg Forever,
MCP, LSP, API, Hub, stdio, SSE, Streamable HTTP. Loanwords that are standard in
developer tooling (token, model, prompt, commit) stay untranslated when a
Turkish rendering would be less clear.

## Address style

- Write to the user in the formal plural: **siz**, e.g. "Esc'ye basın",
  "tekrar deneyebilirsiniz".
- Never mix in the informal singular ("basar", "deneyebilirsin").
- Buttons and one-word menu actions keep the short imperative: **Kur**,
  **Kaldır**, **Kaydet**, **Uygula**, **Dallandır**.
- Descriptions and help text use full sentences; avoid noun piles copied from
  English.

## Casing and punctuation

- Use sentence case for labels, modal titles, and section headers:
  "Akıl yürütme eforu", "Bellek gözlemcisi", "Güncelleme hazır".
- Use the ellipsis character `…`, not three dots.
- Keep key hints in the catalog (`Ctrl+P'ye basın`) rather than translating the
  key name; translate only the action.
- Preserve interpolation placeholders exactly: `%s`/`%d` in Go, `{name}` in
  TypeScript.

## Adding or changing a string

1. Add the key and English value.
2. Add the Turkish value using the glossary and address style above.
3. Keep format verbs and `{placeholder}` names identical between catalogs.
4. Run the parity tests and the app checks:

```bash
go test ./internal/tui/i18n/...
npm --prefix pkg/desktop/frontend test
make fmt && make vet && make test
```

The tests fail when a key exists in only one catalog, when a value is empty, or
when interpolation placeholders differ. Because both surfaces are covered, a
new TUI string and its desktop counterpart stay in sync.
