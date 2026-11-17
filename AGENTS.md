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
4. `KICKOFF.md` — **history**: the prompt the M2 build started from. v0.1.0
   is released (2026-11-17). **Current: M3** (hints under the frame, frame
   screenshots, release builds, `install.sh`) — backlog milestone m-3, GitHub
   issues #1–#4, one PR each.
5. The M3 design review (v1, 2026-11-17):
   https://claude.ai/artifact/DKsU1ZM9ug9kVFo18DqBei — figures
   `screen-hints`, `screenshot-flash`, `tray-menu-m3`, `config-file`;
   proposals H1–H3, S1–S5, R1–R2, I1–I3. **Check its verdict in
   `docs/design.md` before building the hint bar or screenshots.** The M2
   review (HAUvCxn5zaX6zKdeJStgBo) is accepted and recorded there.
6. The backlog: `backlog task list --plain` (the `backlog/` symlink points into
   the vault at `APPDEV/screen-cam/backlog`; the CLI lives under fnm —
   `export PATH=$HOME/.local/share/fnm/node-versions/v24.19.0/installation/bin:$PATH`).

## Layout

- `main.go` — entry point: no arguments runs the tray app; `--no-tray`,
  `follow`, `pattern`, `desktop-entry` for checks and install.
- `internal/app` — the tray app: state, actions, tray menu, camera loop,
  desktop glue (zenity, notifications, autostart, single-instance lock).
- `internal/xui` — global shortcuts, the outline, size tag and drag handles
  (override-redirect windows, all outside the viewport).
- `internal/config` — config.json and presets.json, key parsing.
- `internal/draw` — tray icons, tag text, the off card.
- `internal/capture` — X11 connection, pointer, `GetImage`.
- `internal/frame` — BGRA → letterboxed I420 at the fixed output size.
- `internal/v4l2` — opens the loopback device, sets the format once, writes frames.
- `internal/viewport` — rectangle maths: centre on a point, resize, keep on screen.

## Working rules

- `make test` before every commit, and `scripts/e2e.sh` (Docker, Xvfb, the
  real /dev/video10 — stop any running screen-cam first) before a release.
  `go run -tags shots ./cmd/shots ./screen-cam DIR` retakes the report
  screenshots on the real display over a test screen it draws.
- `make test` before every commit. Tests must not read the real screen: capture
  tests paint an off-screen pixmap and grab that. **Never capture James's live
  desktop to verify anything** — use `screen-cam pattern` and read the device
  back, or a window/pixmap the test draws itself.
- The kernel module and `/dev/video10` come from `install.sh`'s camera step
  (apt + sudo, asked first) or, on James's machines,
  `dotfiles/debian-native/install/install-screen-cam.sh`. The app itself never
  needs root. GitHub issues and PR text are public-facing: app features only,
  neutral wording.
- Go floor 1.25: `golang.org/x/sys` is pinned at v0.47.0 because v0.48+ needs
  Go 1.26. Don't let `go get -u` raise the floor silently.
- Branch per milestone; commit locally; never push without asking.
- Commits: scoped subjects (`frame: …`), changelog trailers per the suite
  conventions (`Changelog:`, `Upgrade:`) in the final trailer block.
