#!/bin/sh
# install.sh: install Screen Cam from its GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/gostega/screen-cam/main/install.sh | sh
#
# Installs the binary for this machine's architecture into ~/.local/bin,
# verified against the release's SHA256SUMS, adds an applications-menu entry
# and starts it at login. No root is needed for any of that.
#
# The virtual camera itself is a kernel module (v4l2loopback). If
# /dev/video10 is missing, the script offers to set it up via apt, using
# sudo, and asks first. Elsewhere it prints what to install.
#
# Options (after `sh -s --` when piped):
#   --version <tag>      install that release instead of the latest
#   --no-camera-module   skip the virtual-camera setup
#   --no-start           don't start or restart Screen Cam afterwards
#   --yes                answer yes to the camera-setup question
#   --uninstall          remove what this script installed
#
# Environment: PREFIX (default ~/.local), SCREEN_CAM_RELEASES (base URL of
# the releases, default the GitHub repo's; used for testing).
set -eu

REPO_URL=https://github.com/gostega/screen-cam
RELEASES=${SCREEN_CAM_RELEASES:-$REPO_URL/releases}
PREFIX=${PREFIX:-$HOME/.local}
BIN=$PREFIX/bin/screen-cam
APPS=${XDG_DATA_HOME:-$HOME/.local/share}/applications
AUTOSTART=${XDG_CONFIG_HOME:-$HOME/.config}/autostart
DEVICE_NR=10

TAG=latest
CAMERA=1
START=1
YES=0
ACTION=install

say() { printf '%s\n' "$*"; }
die() { printf 'screen-cam install: %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --version) [ $# -ge 2 ] || die "--version needs a tag, e.g. v0.2.0"; TAG=$2; shift ;;
    --no-camera-module) CAMERA=0 ;;
    --no-start) START=0 ;;
    --yes) YES=1 ;;
    --uninstall) ACTION=uninstall ;;
    -h|--help) sed -n '2,22p' "$0" 2>/dev/null || say "see $REPO_URL"; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
  shift
done

# ask <question>: yes/no on the terminal, even when this script arrives on a
# pipe (stdin is the script then). No terminal means no.
ask() {
  [ $YES -eq 1 ] && return 0
  [ -r /dev/tty ] && [ -w /dev/tty ] || return 1
  printf '%s [Y/n] ' "$1" > /dev/tty
  read -r reply < /dev/tty || return 1
  case "$reply" in ''|y|Y|yes|Yes) return 0 ;; *) return 1 ;; esac
}

fetch() { # fetch <url> <file>
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$2" "$1"
  else
    die "needs curl or wget"
  fi
}

running() { pgrep -x screen-cam >/dev/null 2>&1; }

stop_app() {
  running || return 0
  pkill -x screen-cam 2>/dev/null || true
  i=0
  while running && [ $i -lt 20 ]; do sleep 0.1; i=$((i + 1)); done
}

# ── uninstall ──────────────────────────────────────────────────────────────
if [ "$ACTION" = uninstall ]; then
  stop_app
  rm -f "$BIN" "$APPS/screen-cam.desktop" "$AUTOSTART/screen-cam.desktop"
  say "Removed $BIN and its menu and login entries."
  say "Left in place: your settings (~/.config/screen-cam), your screenshots,"
  say "and the v4l2loopback camera module, if it was set up (remove it with"
  say "your package manager, plus /etc/modprobe.d/v4l2loopback-screen-cam.conf"
  say "and /etc/modules-load.d/v4l2loopback-screen-cam.conf)."
  exit 0
fi

# ── install ────────────────────────────────────────────────────────────────
[ "$(uname -s)" = Linux ] || die "Screen Cam runs on Linux only (X11 and a V4L2 camera device)"
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "no release build for $(uname -m); build from source: $REPO_URL" ;;
esac
command -v sha256sum >/dev/null 2>&1 || die "needs sha256sum (coreutils)"

ASSET=screen-cam-linux-$ARCH
if [ "$TAG" = latest ]; then BASE=$RELEASES/latest/download; else BASE=$RELEASES/download/$TAG; fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM

say "Downloading Screen Cam ($TAG, linux/$ARCH)"
fetch "$BASE/$ASSET" "$TMP/$ASSET" ||
  die "could not download $BASE/$ASSET (no published release for $TAG?)"
fetch "$BASE/SHA256SUMS" "$TMP/SHA256SUMS" || die "could not download $BASE/SHA256SUMS"
(cd "$TMP" && grep " $ASSET\$" SHA256SUMS | sha256sum -c --status) ||
  die "checksum mismatch for $ASSET; nothing was installed"

WAS_RUNNING=0
if running; then WAS_RUNNING=1; fi
mkdir -p "$PREFIX/bin" "$APPS" "$AUTOSTART"
# Install beside the target, then rename: a running copy keeps its old file.
install -m 755 "$TMP/$ASSET" "$BIN.new" && mv -f "$BIN.new" "$BIN"
"$BIN" desktop-entry > "$APPS/screen-cam.desktop"
"$BIN" desktop-entry > "$AUTOSTART/screen-cam.desktop"
say "Installed $("$BIN" --version) to $BIN, with a menu entry and start at login."

case ":$PATH:" in
  *":$PREFIX/bin:"*) ;;
  *) say "Note: $PREFIX/bin is not on your PATH; the menu entry works regardless." ;;
esac
if [ "${XDG_SESSION_TYPE:-}" = wayland ]; then
  say "Note: this is a Wayland session. Screen Cam needs X11 and won't capture here."
fi

# ── the virtual camera ─────────────────────────────────────────────────────
setup_module() {
  conf="options v4l2loopback devices=1 video_nr=$DEVICE_NR card_label=\"Screen Cam\" exclusive_caps=1"
  # DKMS builds the module against kernel headers: the running kernel's, so it
  # loads now, and the distribution's metapackage, so it follows kernel
  # upgrades. Looked up after apt-get update, when the package lists are real.
  sudo sh -c "
    set -e
    apt-get update -qq
    pkgs='v4l2loopback-dkms v4l2loopback-utils'
    for h in linux-headers-\$(uname -r) linux-headers-\$(dpkg --print-architecture) linux-headers-generic; do
      if [ -n \"\$(apt-cache madison \$h 2>/dev/null)\" ]; then pkgs=\"\$pkgs \$h\"; fi
    done
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \$pkgs
    mkdir -p /etc/modprobe.d /etc/modules-load.d
    printf '%s\\n' '$conf' > /etc/modprobe.d/v4l2loopback-screen-cam.conf
    printf 'v4l2loopback\\n' > /etc/modules-load.d/v4l2loopback-screen-cam.conf
    modprobe v4l2loopback 2>/dev/null || true
  "
}

if [ $CAMERA -eq 1 ] && [ ! -e /dev/video$DEVICE_NR ]; then
  say ""
  say "The virtual camera (/dev/video$DEVICE_NR) isn't set up yet."
  if command -v apt-get >/dev/null 2>&1 && command -v sudo >/dev/null 2>&1; then
    say "It needs the v4l2loopback kernel module from your distribution (root, via sudo)."
    if ask "Set it up now?"; then
      setup_module || say "Camera setup failed; Screen Cam still installed. Re-run with this script, or see $REPO_URL#the-virtual-camera"
      if [ -e /dev/video$DEVICE_NR ]; then say "Camera ready: /dev/video$DEVICE_NR"; else say "The module loads after a reboot (it may need the kernel's headers)."; fi
    else
      say "Skipped. Set it up later by re-running this script."
    fi
  else
    say "Install v4l2loopback from your distribution, then load it with:"
    say "  sudo modprobe v4l2loopback devices=1 video_nr=$DEVICE_NR card_label=\"Screen Cam\" exclusive_caps=1"
  fi
fi

# ── start ──────────────────────────────────────────────────────────────────
if [ $START -eq 1 ] && [ -n "${DISPLAY:-}" ]; then
  stop_app
  if command -v setsid >/dev/null 2>&1; then setsid "$BIN" >/dev/null 2>&1 < /dev/null &
  else nohup "$BIN" >/dev/null 2>&1 < /dev/null & fi
  if [ $WAS_RUNNING -eq 1 ]; then say "Restarted Screen Cam."; else say "Started Screen Cam: look for the camera icon in the tray."; fi
fi
say "Uninstall: curl -fsSL https://raw.githubusercontent.com/gostega/screen-cam/main/install.sh | sh -s -- --uninstall"
