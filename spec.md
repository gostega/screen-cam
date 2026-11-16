# screen-cam — spec

## What it is for

Testing camera-based scanners — first of all ubervent's QR scan — against
codes on screen instead of printed stickers. You point a virtual webcam at part
of the screen by moving the mouse; the browser sees that part as its camera.

## Who and when

James, at the desk, with a label sheet or a sticker PDF open on one side and
ubervent's scan screen on the other. He starts the camera with a shortcut,
hovers over a code, watches the scan land, resizes or switches preset when a
code is too small or a row of labels needs to fit, and stops the camera.

## v0.1 line

- Tray icon with the menu in the design review (`tray-menu`), camera off at
  start, live indicator.
- Viewport follows the mouse or is pinned; resize in 10% steps, by
  Ctrl+Alt+scroll, and by aspect cycle; live, without the browser losing the
  camera.
- Named presets of viewport size, saved from the tray or a shortcut,
  switchable with Ctrl+Alt+1–9, editable as a JSON file.
- Global shortcuts (Ctrl+Alt+…), configurable.
- On-screen outline and size tag, outside the captured area.
- `make install`: binary in `~/.local/bin`, menu entry, optional autostart.

Pending James's verdict on the design review; the list above is the proposal.

## Non-goals

- Wayland, recording, audio, scenes or overlays (OBS does that).
- Being a dependency of ubervent or any suite app. It is a desk tool.
- Root privileges. The kernel module is set up once by the dotfiles installer.

## Boundary

- `dotfiles/…/install-screen-cam.sh` owns the kernel module and device.
- ubervent owns scanning; its automated tests use Chromium's fake camera
  with recorded videos and do not need this app.
