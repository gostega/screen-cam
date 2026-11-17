# screen-cam — design decisions

## Design review

Proposal v2, 2026-11-17: https://claude.ai/artifact/HAUvCxn5zaX6zKdeJStgBo
(v1 was the same page; v2 applied the maintainer's two comments below.)

**Verdict: accepted, 2026-11-17.** The maintainer: *"all other proposals accepted"*,
after two comments that v2 applied (D2 menu item, D3 drag handles). Build to
proposal v2.

| ID | Proposal | Status |
|---|---|---|
| D1 | Pure Go, fixed 1280×720 output (ADR-0001) | **accepted** |
| D2 | Ctrl+Alt shortcut set | **accepted**, amended — **the maintainer 2026-11-17 on the config file: "This is fine, but also add a menu item to 'Edit shortcuts..' like you did for the presets."** Added as tray-menu 5. |
| D3 | Resize by steps, Ctrl+Alt+scroll, aspect cycle, **plus drag handles on a pinned frame** (figure `screen-pinned`) | **accepted as changed by the maintainer 2026-11-17:** "I would like drag handles, we can add a hotkey to lock the frame in position for dragging." Pin (Ctrl+Alt+P) is that lock. |
| D4 | Preset = name + size, no position | **accepted** |
| D5 | Camera off at launch; red tray dot; "off" card while stopped | **accepted** |
| D6 | Autostart at login; `make install` to `~/.local/bin` | **accepted** |
| D7 | Own repo to suite conventions; no dev port | **accepted** |
| D8 | Retire the earlier ffmpeg script once the app works | **accepted** |

## No dev port, no dev-publish

A desktop tray tool has nothing to serve, so it is not in the dev-publish
registry (suite conventions §3) and has no `dev-publish` target. The version
stamp (§1) appears in the tray menu and on `--version`.

## Never capture the real screen in checks

The 2026-11-17 spike recorded whatever was on the screen. Verification
therefore uses `screen-cam pattern` read back from the device, and capture
tests grab an off-screen pixmap they paint themselves.

## Session reports

- 2026-11-17, M2 tray build (v0.1.0-rc1): https://claude.ai/artifact/D9cqzygop3HJEzCmLPNtBw

## M3 design review

Proposal v1, 2026-11-17: https://claude.ai/artifact/DKsU1ZM9ug9kVFo18DqBei
(hint bar H1–H3, screenshots S1–S5, release builds R1–R2, installer I1–I3).
**Verdict: pending.** R1 and I1–I2 are being built ahead of it as independent
work; the hint bar and screenshots wait for it.

- **Installer, not an apt repo (I1).** A signed apt repo needs key management
  and hosting for a one-binary app with no dependencies; a checksummed release
  download gets the same result. The installer stays user-level except the
  optional camera-module step, which uses sudo and asks first.
