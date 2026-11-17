# screen-cam Makefile — the suite's standard targets.
#
# No dev-publish / backlog-publish targets: a tray tool has nothing to serve
# (docs/design.md §No dev port).

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"
PREFIX  ?= $(HOME)/.local
DEVICE  ?= /dev/video10

.PHONY: build test run pattern release install uninstall changelog

build:
	go build $(LDFLAGS) -o screen-cam .

test:
	go vet ./...
	go test ./...

# The engine until the tray exists: a 640x480 viewport following the mouse.
run: build
	./screen-cam follow -device $(DEVICE)

# Colour bars on the device, no screen capture. For checks.
pattern: build
	./screen-cam pattern -device $(DEVICE)

# release: the binaries a GitHub release carries (.github/workflows/release.yml
# runs exactly this), one per architecture plus SHA256SUMS. Static, so they run
# on any Linux with X11. Asset names are what install.sh asks for.
ARCHES ?= amd64 arm64

release:
	@git describe --tags --exact-match >/dev/null 2>&1 || { echo "release: HEAD is not tagged"; exit 1; }
	@case "$(VERSION)" in *-dirty) echo "release: working tree has uncommitted changes"; exit 1;; esac
	rm -rf dist && mkdir -p dist
	for a in $(ARCHES); do \
	  CGO_ENABLED=0 GOOS=linux GOARCH=$$a go build -trimpath -ldflags "-s -w -X main.Version=$(VERSION)" \
	    -o dist/screen-cam-linux-$$a . || exit 1; \
	done
	cd dist && sha256sum screen-cam-linux-* > SHA256SUMS
	scripts/release-notes.sh $(VERSION) > dist/NOTES.md

# install: the binary, an Applications-menu entry, and start at login (D6;
# the tray's "Start at login" item turns that off again).
APPS      ?= $(HOME)/.local/share/applications
AUTOSTART ?= $(HOME)/.config/autostart

install: build
	install -Dm755 screen-cam $(PREFIX)/bin/screen-cam
	mkdir -p $(APPS) $(AUTOSTART)
	$(PREFIX)/bin/screen-cam desktop-entry > $(APPS)/screen-cam.desktop
	$(PREFIX)/bin/screen-cam desktop-entry > $(AUTOSTART)/screen-cam.desktop
	@echo "installed $(PREFIX)/bin/screen-cam ($(VERSION)), menu entry, start at login"
	@echo "start it now from the Applications menu (Screen Cam), or: setsid $(PREFIX)/bin/screen-cam &"

uninstall:
	-pkill -x screen-cam
	rm -f $(PREFIX)/bin/screen-cam $(APPS)/screen-cam.desktop $(AUTOSTART)/screen-cam.desktop

changelog:
	@$(HOME)/.claude/skills/james-dev-conventions/helpers/changelog.sh
