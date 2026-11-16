# ADR-0001: Pure Go capture and a fixed 1280×720 output

Status: accepted (spike evidence 2026-11-17; James accepted D1 the same day)

## Context

The first version was a shell script running `ffmpeg -f x11grab
-follow_mouse centered … -f v4l2 /dev/video10`. Changing the viewport size
means restarting ffmpeg, which closes the device: a browser reading the
camera sees the stream end, and a new size is a new format it must
renegotiate. Resizing while scanning is the main feature wanted.

## Decision

The app captures the screen itself (X11 `GetImage` through
`github.com/jezek/xgb`) and writes to the loopback device itself. It sets the
device format **once**, 1280×720 YU12, and keeps the device open for the
app's lifetime. Any viewport size is scaled to fit that picture, letterboxed.

## Evidence

The spike set the format once and wrote frames while switching the viewport
640×480 → 320×240 → 1200×900 every 3 s. An attached reader received frames
throughout (106 frames, 9 s, 11.7 fps unoptimised). When the writer
stopped, the reader blocked rather than erroring.

## Consequences

- No ffmpeg dependency.
- Small viewports are upscaled (nearest neighbour), which keeps QR modules
  sharp-edged.
- While the camera is "off" the app keeps writing a still card, because a
  closed writer leaves readers waiting.
- Changing the output size is a restart-level setting, not a live control.
