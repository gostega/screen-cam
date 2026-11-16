# screen-cam — design decisions

## Design review

Proposal v1, 2026-11-17: https://claude.ai/artifact/HAUvCxn5zaX6zKdeJStgBo

**Verdict: pending.** Record the maintainer's answers here by proposal id (D1–D8) and
figure/callout, with the date, before building the tray, shortcuts, outline
or presets.

| ID | Proposal | Status |
|---|---|---|
| D1 | Pure Go, fixed 1280×720 output (ADR-0001) | proven by spike; pending verdict |
| D2 | Ctrl+Alt shortcut set | proposed |
| D3 | Resize by steps, Ctrl+Alt+scroll, aspect cycle; no drag handles | proposed |
| D4 | Preset = name + size, no position | proposed |
| D5 | Camera off at launch; red tray dot; "off" card while stopped | proposed |
| D6 | Autostart at login; `make install` to `~/.local/bin` | proposed |
| D7 | Own repo to suite conventions; no dev port | proposed |
| D8 | Retire the earlier ffmpeg script once the app works | proposed |

## No dev port, no dev-publish

A desktop tray tool has nothing to serve, so it is not in the dev-publish
registry (suite conventions §3) and has no `dev-publish` target. The version
stamp (§1) appears in the tray menu and on `--version`.

## Never capture the real screen in checks

The 2026-11-17 spike recorded whatever was on the screen. Verification
therefore uses `screen-cam pattern` read back from the device, and capture
tests grab an off-screen pixmap they paint themselves.
