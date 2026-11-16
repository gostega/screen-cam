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

release:
	@git describe --tags --exact-match >/dev/null 2>&1 || { echo "release: HEAD is not tagged"; exit 1; }
	mkdir -p dist
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o dist/screen-cam .

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
	@git shortlog --group=trailer:changelog --format=reference $$(git describe --tags --abbrev=0)..HEAD
