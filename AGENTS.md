# screen-cam — agent instructions

A Linux tray app that turns a viewport of the screen into a webcam: it
captures a rectangle that follows the mouse (or is pinned) and writes it to a
v4l2loopback device, so a browser scanner — ubervent's QR scan — can be tested
against codes shown on screen. Pure Go, X11 only.

## Read first, in this order

1. `spec.md` — what it is for, the v0.1 line, non-goals.
2. `docs/adr/` — decisions not to relitigate. ADR-0001: pure Go, fixed
   1280×720 output, no ffmpeg.
3. `docs/design.md` — smaller decisions with their reasons.
4. `KICKOFF.md` — the paste-ready prompt for the M2 tray build.
5. The design review (proposal v1, 2026-11-17):
   https://claude.ai/artifact/HAUvCxn5zaX6zKdeJStgBo — figures `tray-menu`,
   `screen-live`, `save-preset`; proposals D1–D8. **Check its verdict in
   `docs/design.md` §Design review before building any UI.** Until a verdict
   is recorded there, only the engine (capture, frame, v4l2, viewport) is
   cleared to build.
6. The backlog: `backlog task list --plain` (the `backlog/` symlink points into
   the vault at `APPDEV/screen-cam/backlog`; the CLI lives under fnm —
   `export PATH=$HOME/.local/share/fnm/node-versions/v24.19.0/installation/bin:$PATH`).

## Layout

- `main.go` — entry point and commands (`follow`, `pattern`; the tray comes later).
- `internal/capture` — X11 connection, pointer, `GetImage`.
- `internal/frame` — BGRA → letterboxed I420 at the fixed output size.
- `internal/v4l2` — opens the loopback device, sets the format once, writes frames.
- `internal/viewport` — rectangle maths: centre on a point, resize, keep on screen.

## Working rules

- `make test` before every commit. Tests must not read the real screen: capture
  tests paint an off-screen pixmap and grab that. **Never capture James's live
  desktop to verify anything** — use `screen-cam pattern` and read the device
  back, or a window/pixmap the test draws itself.
- The kernel module and `/dev/video10` come from
  `dotfiles/debian-native/install/install-screen-cam.sh` (needs root). This
  app never needs root.
- Go floor 1.25: `golang.org/x/sys` is pinned at v0.47.0 because v0.48+ needs
  Go 1.26. Don't let `go get -u` raise the floor silently.
- Branch per milestone; commit locally; never push without asking.
- Commits: scoped subjects (`frame: …`), changelog trailers per the suite
  conventions (`Changelog:`, `Upgrade:`) in the final trailer block.
