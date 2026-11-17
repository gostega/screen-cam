# Changelog

All notable changes, newest first. One section per promoted release; work
lands in Unreleased first. Every release says what upgrading does to your data.

## Unreleased

- **Added:** a hint bar under the frame reminds you of two shortcuts
  (pin & resize, screenshot) and never fades. Choose which in config.json's
  `"hints"`; turn it off with Show shortcut hints in the tray menu.
- **Added:** Ctrl+Alt+X (or Take screenshot in the tray) captures the frame
  area at full resolution to the clipboard and to `~/Pictures/ScreenCam`. The
  outline flashes white to confirm, and it works with the camera off. The
  folder is set with Screenshot folder… in the menu.
- **Added:** one-line install:
  `curl -fsSL https://raw.githubusercontent.com/gostega/screen-cam/main/install.sh | sh`.
  It installs the latest release for amd64 or arm64, checksum-verified, into
  `~/.local/bin` with a menu entry and start at login, and offers to set up
  the v4l2loopback camera on apt systems. `--uninstall` removes it.
- **Added:** releases carry Linux amd64 and arm64 binaries and `SHA256SUMS`.

Upgrade: config change — on first start the app adds `show_hints`, `hints`,
`screenshot_dir` and `shortcuts.screenshot` (Ctrl+Alt+X) to
`~/.config/screen-cam/config.json`, keeping every existing value.

## v0.1.0 — 2026-11-17

- **Added:** Screen Cam runs in the tray. Start and stop with Ctrl+Alt+C or
  a left-click; the camera starts off and shows an "off" card until then.
- **Added:** named presets of viewport size: Ctrl+Alt+1–9, saved with
  Ctrl+Alt+S through a name prompt, and editable from "Edit presets file…".
- **Added:** live resizing (Ctrl+Alt+= / -, Ctrl+Alt+scroll, aspect cycle
  Ctrl+Alt+A) without the browser losing the camera; Ctrl+Alt+P pins the
  frame, which can then be moved by its tag and resized by eight handles.
- **Added:** shortcuts are configurable from "Edit shortcuts file…" and
  reload without a restart; a shortcut another program holds is reported.
- **Added:** `make install` adds a menu entry and starts Screen Cam at login.
- **Added:** capture engine — a viewport that follows the mouse is written to
  `/dev/video10` at a fixed 1280×720, so resizing never drops the browser's
  camera (`screen-cam follow`); `screen-cam pattern` writes colour bars for
  checks that don't touch the screen.

Upgrade: nothing to do
