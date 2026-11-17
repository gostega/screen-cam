# Screen Cam

Turn part of your screen into a webcam. A frame follows your mouse, or stays
where you pin it, and whatever is inside it goes out as a camera any app can
open. It's useful for testing QR and barcode scanners in a browser against
codes on screen, or for showing a slice of your screen in a video call.

Screen Cam lives in the system tray and is driven by global keyboard shortcuts.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/gostega/screen-cam/main/install.sh | sh
```

This installs the latest release for your architecture (amd64 or arm64) into
`~/.local/bin`, checks it against the release's `SHA256SUMS`, adds it to your
applications menu, and starts it at login.

Options go after `sh -s --`, for example:

```sh
curl -fsSL https://raw.githubusercontent.com/gostega/screen-cam/main/install.sh | sh -s -- --version v0.2.0
```

| Option | Does |
|---|---|
| `--version <tag>` | install that release instead of the latest |
| `--no-camera-module` | skip the virtual-camera setup |
| `--no-start` | don't start Screen Cam afterwards |
| `--yes` | set up the virtual camera without asking |
| `--uninstall` | remove Screen Cam (settings and screenshots are kept) |

### The virtual camera

The camera device comes from the
[v4l2loopback](https://github.com/v4l2loopback/v4l2loopback) kernel module.
If `/dev/video10` doesn't exist, the installer offers to set it up on
apt-based systems (Debian, Ubuntu, Mint). That step needs `sudo`. On other
distributions, install v4l2loopback from your package manager and load it with:

```sh
sudo modprobe v4l2loopback devices=1 video_nr=10 card_label="Screen Cam" exclusive_caps=1
```

`exclusive_caps=1` is what Chrome-based browsers need before they will list
the device as a camera.

### Requirements

- Linux with an X11 session. Wayland isn't supported.
- A system tray that speaks StatusNotifierItem. Cinnamon, KDE and XFCE all do;
  GNOME needs the AppIndicator extension.

## Use

Pick **Screen Cam** as the camera in the app that's scanning or calling, then
start Screen Cam from the tray icon or with `Ctrl+Alt+C`. The camera always
sends 1280×720, letterboxed, so resizing the frame never interrupts the app
reading it.

| Shortcut | Does |
|---|---|
| `Ctrl+Alt+C` | start or stop the camera |
| `Ctrl+Alt+1` … `9` | switch to a saved size preset |
| `Ctrl+Alt+=` / `Ctrl+Alt+-`, `Ctrl+Alt`+scroll | bigger / smaller |
| `Ctrl+Alt+A` | cycle the aspect ratio: 4:3, 16:9, 1:1, 9:16 |
| `Ctrl+Alt+P` | pin the frame; drag its tag to move it and its handles to resize |
| `Ctrl+Alt+S` | save the current size as a preset |

Shortcuts and presets are plain JSON in `~/.config/screen-cam/`
(`config.json`, `presets.json`). Open them from the tray menu; changes apply
when you save.

## Build from source

Go 1.25 or later:

```sh
make build      # ./screen-cam
make test
make install    # ~/.local/bin, menu entry, start at login
```

## Licence

Not yet chosen; all rights reserved until a licence file is added.
