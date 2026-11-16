# screen-cam — spec

## What it is for

Testing camera-based scanners — first of all a web app's QR scan — against
codes on screen instead of printed stickers. You point a virtual webcam at part
of the screen by moving the mouse; the browser sees that part as its camera.

## Who and when

Someone at a desk, with a label sheet or a sticker PDF open on one side and
a browser scanner on the other. They start the camera with a shortcut,
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

Accepted by the maintainer 2026-11-17 (design review v2: drag handles on a pinned frame, Edit shortcuts file… in the menu).

## Non-goals

- Wayland, recording, audio, scenes or overlays (OBS does that).
- Being a dependency of any other app. It is a desk tool.
- Root privileges. The kernel module is set up once, outside the app.

## Boundary

- A separate machine-setup script owns the kernel module and device.
- The scanner app owns scanning; its automated tests use Chromium's fake camera
  with recorded videos and do not need this app.
