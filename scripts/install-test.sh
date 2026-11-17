#!/bin/sh
# install-test.sh [--camera] [image...]: run install.sh against a local fake
# release in throwaway containers (default debian:13 and ubuntu:24.04).
#   checks.sh — install, re-run, pinned version, missing release, uninstall,
#               tampered checksum, as an unprivileged user with no sudo.
#   camera.sh — (--camera) the sudo camera-module step and the wget path;
#               downloads kernel headers and builds the DKMS module: slow.
# Needs Docker (run through `sg docker -c` if you're not in the group yet).
set -eu
cd "$(dirname "$0")/.."
suite=checks.sh
if [ "${1:-}" = --camera ]; then suite=camera.sh; shift; fi
[ $# -gt 0 ] || set -- debian:13 ubuntu:24.04
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
mkdir -p "$T/rel/latest/download" "$T/rel/download/v0.2.0-test" "$T/bad/latest/download"
for a in amd64 arm64; do
  CGO_ENABLED=0 GOARCH=$a go build -trimpath -ldflags "-s -w -X main.Version=v0.2.0-test" \
    -o "$T/rel/latest/download/screen-cam-linux-$a" .
done
(cd "$T/rel/latest/download" && sha256sum screen-cam-linux-* > SHA256SUMS)
cp "$T"/rel/latest/download/* "$T/rel/download/v0.2.0-test/"
cp "$T"/rel/latest/download/* "$T/bad/latest/download/"
echo tampered >> "$T/bad/latest/download/screen-cam-linux-amd64"
cp install.sh scripts/install-test/*.sh "$T/"
chmod -R a+rX "$T"
for img in "$@"; do
  echo "== $img ($suite)"
  docker run --rm -v "$T:/t:ro" "$img" sh "/t/$suite"
done
