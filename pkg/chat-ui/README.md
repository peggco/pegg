# @pegg/chat-ui

The shared Pegg chat renderer used by the VS Code extension and the JetBrains
plugin. It is an internal package, not published separately yet.

The UI is host-agnostic:

- `src/state.ts` defines the `ChatState`/`ChatItem` model and conversation helpers.
- `src/types.ts` defines the ACP wire types shared by hosts.
- `src/webview/App.tsx` is the React chat UI.
- `src/webview/styles.css` is the full UI stylesheet. Hosts map their theme to the
  `--vscode-*` CSS variables used by the stylesheet.
- `src/webview/markdown.ts` renders markdown with syntax highlighting and sanitizing.
- `src/webview/host.ts` exposes a tiny `ChatHost` bridge (`postMessage`).
- `src/webview/main.tsx` exports `mount(host)`.

Hosts only need to provide:

1. A `ChatHost` that delivers outgoing webview messages.
2. A backend that produces `ChatState` JSON and handles the existing message
   protocol (`send`, `stop`, `setMode`, `setModel`, `pickModel`, permissions,
   questions, attachments, voice, open file/diff, and so on).
3. Theme CSS variables and any optional feature flags:
   `window.__peggHideReady` and `window.__peggInlinePickers`.

VS Code uses its native quick pick unless `__peggInlinePickers` is set; the
JetBrains host sets it to render the mode/model picker inside the webview.

## Development

```bash
npm ci
npm run typecheck
```

The hosts bundle this source directly; there is no separate build artifact.
