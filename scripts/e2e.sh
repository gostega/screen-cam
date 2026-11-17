#!/usr/bin/env bash
# e2e.sh — run cmd/e2e against a fresh screen-cam build on a virtual X
# display inside a throwaway Debian container. Nothing touches the real
# desktop: the container has its own Xvfb, and the "screen" is a tile grid
# the checker draws. The host's /dev/video10 (v4l2loopback) is passed in,
# so nothing else may be writing to it while this runs.
set -euo pipefail
cd "$(dirname "$0")/.."
DEV=${DEVICE:-/dev/video10}
OUT=$(mktemp -d)
trap 'rm -rf "$OUT"' EXIT
CGO_ENABLED=0 go build -o "$OUT/screen-cam" .
CGO_ENABLED=0 go build -tags e2e -o "$OUT/e2e" ./cmd/e2e
CGO_ENABLED=0 go test -c -o "$OUT/clipboard.test" ./internal/clipboard
if command -v fuser >/dev/null && fuser "$DEV" >/dev/null 2>&1; then
  echo "$DEV is in use (is screen-cam running?) — stop it first" >&2; exit 1
fi
sg docker -c "docker run --rm --device $DEV -v $OUT:/work debian:13 bash -c '
  apt-get update -qq >/dev/null && apt-get install -y -qq xvfb zenity xclip >/dev/null 2>&1
  Xvfb :99 -noreset -screen 0 2560x1440x24 >/dev/null 2>&1 &
  sleep 2
  DISPLAY=:99 SCREEN_CAM_TEST_DISPLAY=:99 /work/clipboard.test 2>/dev/null || { echo FAIL clipboard tests; exit 1; }
  DISPLAY=:99 /work/e2e /work/screen-cam
'"
