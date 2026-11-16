---
icon: lucide/globe
---

# Browser Use

A real Chromium browser for the agent. Browser Use drives a persistent browser through
go-rod with stealth evasions: navigation, accessibility snapshots with stable refs,
forms, clicks, uploads, downloads, screenshots, PDFs, tabs, JavaScript evaluation, and
low-level mouse control.

## Install

```bash
pegg plugins install peggco/pegg-browser-use
```

Then load the `browser-use` skill before the first browser tool call:

```text
/browser-use
```

The skill activates the `browser` tool bundle.

## Usage

```text
Open http://localhost:3000 and check the signup form.
Take a screenshot of the pricing page.
Fill the form with the test account and submit it.
Download the invoice PDF.
Open this site in two tabs and compare them.
```

The browser lives inside the plugin process and is shared across tool calls, so tabs,
cookies, and logins persist for the session.

## Configuration

Options live under `plugins.options["browser-use"]` in `~/.pegg/pegg.json`:

```json
{
  "plugins": {
    "options": {
      "browser-use": {
        "enabled": true,
        "headless": true,
        "stealth": true,
        "profile_dir": "~/.pegg/browser/profile",
        "viewport_width": 1280,
        "viewport_height": 800,
        "timeout_seconds": 30,
        "language": "en-US",
        "max_snapshot_elements": 400,
        "auto_accept_dialogs": true,
        "ignore_cert_errors": false,
        "chrome_path": "",
        "cdp_url": "",
        "proxy": ""
      }
    }
  }
}
```

| Setting | Meaning |
|---|---|
| `headless` | Run without a visible window. Set `false` to watch the browser. |
| `stealth` | Apply stealth evasions. |
| `profile_dir` | Persistent profile directory. Defaults to `~/.pegg/browser/profile`. |
| `viewport_width` / `viewport_height` | Viewport size. |
| `timeout_seconds` | Default navigation/tool timeout. |
| `max_snapshot_elements` | Cap for accessibility snapshots. |
| `chrome_path` | Use a specific Chrome/Chromium binary. |
| `cdp_url` | Attach to an existing Chrome DevTools endpoint. |
| `proxy` | Proxy URL. |

Environment overrides:

| Variable | Meaning |
|---|---|
| `PEGG_BROWSER_HEADLESS` | `true`/`false` |
| `PEGG_BROWSER_STEALTH` | `true`/`false` |
| `PEGG_BROWSER_PATH` | Chrome/Chromium binary |
| `PEGG_BROWSER_CDP` | Existing Chrome DevTools endpoint |
| `PEGG_BROWSER_PROFILE` | Profile directory |
| `PEGG_BROWSER_PROXY` | Proxy URL |
| `PEGG_BROWSER_NO_SANDBOX` | `1` to add `--no-sandbox` |

The profile is dedicated to Pegg and keeps cookies and logins across runs. To sign in
interactively, run `pegg browser open https://example.com` or set `"headless": false`.

## Tools

Read-only: `browser_status`, `browser_navigate`, `browser_snapshot`, `browser_screenshot`,
`browser_pdf`, `browser_scroll`, `browser_hover`, `browser_wait`, `browser_tabs`,
`browser_logs`, `browser_close`.

State-changing, permission-gated: `browser_click`, `browser_type`, `browser_press`,
`browser_select`, `browser_upload`, `browser_download`, `browser_eval`, `browser_mouse`.

Artifacts are written under `.pegg/artifacts/plugins/browser-use/` and attached to the
conversation.

## Commands

```bash
pegg browser status
pegg browser install        # download Chromium into the rod cache
pegg browser open <url>
```

## Notes

- Chromium is downloaded on first use unless `chrome_path` or `cdp_url` is set.
- A headed browser using the same profile must be closed before another process uses it.
- Use `PEGG_BROWSER_NO_SANDBOX=1` only in trusted containers or as root.
