---
icon: lucide/monitor
---

# Computer Use

Control the real desktop from the agent: screenshots, mouse, keyboard, windows,
clipboard, and a fast decision-model `computer_task` loop.

## Install

```bash
pegg plugins install peggco/pegg-computer-use
```

Then load the `computer-use` skill before calling any `computer_*` tool:

```text
/computer-use
```

## Platform support

| Platform | Backend | Screenshot | Input | Windows |
|---|---|---|---|---|
| Linux X11 | `x11` (pure Go) | yes | yes | list, focus, minimize, maximize, close |
| Linux wlroots Wayland (Sway, Hyprland, Wayfire) | `wayland` | yes | yes | focus, minimize, maximize, close (no pid mapping) |
| Linux GNOME/KDE Wayland | `libei` RemoteDesktop portal + screenshot command | yes, via `grim`/`gnome-screenshot`/`spectacle`/`flameshot`/`scrot` | yes | not supported |
| Windows | `win` (Win32) | yes | yes | list, focus, minimize, maximize, close |
| macOS | `darwin` (Quartz/CoreGraphics) | yes | yes | list, focus, minimize, maximize, close |

No CGO is required.

### Permissions

- **macOS** — grant **Screen Recording** and **Accessibility**, then restart Pegg.
- **Linux X11** — `DISPLAY` must be set.
- **Linux wlroots Wayland** — the compositor must expose virtual pointer, virtual keyboard,
  screencopy, and foreign-toplevel protocols.
- **Linux GNOME/KDE Wayland** — accept the RemoteDesktop consent dialog on first run.
- **Windows** — if the target app is elevated, run Pegg elevated too.

## Configuration

```json
{
  "plugins": {
    "options": {
      "computer-use": {
        "enabled": true,
        "display_id": -1,
        "max_image_width": 1920,
        "grid_size": 100,
        "jpeg_quality": 90,
        "type_interval_ms": 0,
        "timeout_seconds": 60,
        "allow_window_control": true,
        "allow_clipboard": true,
        "artifact_dir": ".pegg/artifacts/plugins/computer-use",
        "task_max_steps": 12,
        "task_image_width": 1280,
        "task_timeout_seconds": 180,
        "task_send_screenshot": true
      }
    }
  }
}
```

Environment overrides use the `PEGG_COMPUTER_` prefix, for example
`PEGG_COMPUTER_DISPLAY`, `PEGG_COMPUTER_ALLOW_CLIPBOARD`, and
`PEGG_COMPUTER_TASK_MODEL`.

## Tools

| Tool | Read-only | Permission | Description |
|---|---|---|---|
| `computer_status` | yes | allow | Platform, backend, screen, displays, permissions, active window |
| `computer_screenshot` | yes | allow | Capture the screen or region, optional grid and downscale |
| `computer_click` | no | semi-judge | Move and click at screen coordinates |
| `computer_move` | no | semi-judge | Move the pointer |
| `computer_drag` | no | semi-judge | Press, move, release |
| `computer_scroll` | no | semi-judge | Wheel scroll at an optional position |
| `computer_type` | no | semi-judge | Type text, optionally press Enter |
| `computer_key` | no | semi-judge | Press a key or chord |
| `computer_windows` | no | semi-judge | Active/list/focus/minimize/maximize/close |
| `computer_clipboard` | no | semi-judge | Get or set the clipboard |
| `computer_wait` | yes | allow | Wait for the UI to settle |
| `computer_task` | no | ask | Decision-model loop: goal in, trace + latest screenshot out |

`computer_windows` requires `confirm=true` for `close` and `allow_window_control` must
not be disabled. `computer_clipboard` requires `allow_clipboard`.

## `computer_task` decision model

`computer_task` chooses its provider/model in this order:

1. `provider` / `model` on the call.
2. `plugins.options.computer-use.task_provider` / `task_model`.
3. `permission.judge_provider` / `judge_model`.
4. `smart_router.provider` / `model`.
5. The first configured provider with an API key.

The model must accept image content for visual tasks. If it does not, the loop retries
without images and prefers keyboard/window actions. Set `task_send_screenshot: false` to
force keyboard-only mode.

## CLI

```bash
pegg-computer-use computer status
pegg-computer-use computer screenshot --out shot.png --grid
pegg-computer-use computer click 640 360 --button left
pegg-computer-use computer type "hello" --enter
pegg-computer-use computer key ctrl+s
pegg-computer-use computer task "open Settings and enable notifications" --max-steps 12
pegg-computer-use computer windows list
```

## Limitations

- The mouse cursor is usually not drawn in screenshots. Use the grid or the focus ring.
- Wayland has no global cursor-position protocol; positions are best-effort.
- Window listing is unavailable on GNOME/KDE Wayland and the libei backend.
