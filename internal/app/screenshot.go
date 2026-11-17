package app

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gostega/screen-cam/internal/capture"
	"github.com/gostega/screen-cam/internal/clipboard"
	"github.com/gostega/screen-cam/internal/config"
	"github.com/gostega/screen-cam/internal/draw"
	"github.com/gostega/screen-cam/internal/shot"
	"github.com/gostega/screen-cam/internal/viewport"
)

// screenshot captures the frame area to the clipboard and a file (figure
// screenshot-flash). It runs off the caller's goroutine: the shortcut
// arrives on the X event loop, which must not wait for a PNG encode.
func (a *App) screenshot() {
	go func() {
		a.shotMu.Lock()
		defer a.shotMu.Unlock()
		a.takeScreenshot()
	}()
}

func (a *App) takeScreenshot() {
	if a.shotX == nil {
		x, err := capture.Connect()
		if err != nil {
			a.shotFailed("Screenshot failed", err)
			return
		}
		a.shotX = x
	}
	x := a.shotX

	// The frame as it stands: live, where the camera last drew it; off, where
	// it would be — the pinned spot, or around the pointer (S5).
	a.mu.Lock()
	live := a.live
	var r viewport.Rect
	if (live || a.pinned) && a.rect.W > 0 {
		r = a.rect
	}
	w, h := a.w, a.h
	dir := shot.Dir(a.conf.ScreenshotDir)
	a.mu.Unlock()
	if r.W == 0 {
		px, py, err := x.Pointer()
		if err != nil {
			a.shotFailed("Screenshot failed", err)
			return
		}
		r = viewport.CenteredOn(px, py, w, h, a.screenW, a.screenH)
	}

	// Capture before any feedback is drawn, so the flash can't be in it.
	bgra, err := x.Grab(x.Root(), r.X, r.Y, r.W, r.H)
	if err != nil {
		a.shotFailed("Screenshot failed", err)
		return
	}
	png, err := shot.PNG(bgra, r.W, r.H)
	if err != nil {
		a.shotFailed("Screenshot failed", err)
		return
	}

	clipErr := a.toClipboard(png)
	path, saveErr := shot.Save(png, dir, time.Now())
	debugf("screenshot rect=%d,%d,%dx%d path=%s clipboard=%v save=%v", r.X, r.Y, r.W, r.H, path, clipErr, saveErr)

	if !live {
		a.ui.Move(r) // show where it was taken, then take the outline away again
		time.AfterFunc(2*time.Second, func() {
			a.mu.Lock()
			still := !a.live
			a.mu.Unlock()
			if still {
				a.ui.Hide()
			}
		})
	}
	a.ui.Flash()
	switch {
	case clipErr == nil && saveErr == nil:
		a.showTag("Screenshot copied to clipboard")
	case saveErr == nil:
		a.showTag("Screenshot saved; clipboard unavailable")
		notify("Screenshot not copied", clipErr.Error()+"\nSaved to "+path)
	case clipErr == nil:
		a.showTag("Copied to clipboard; not saved")
		notify("Screenshot not saved", saveErr.Error())
	default:
		a.shotFailed("Screenshot failed", fmt.Errorf("%v; %v", clipErr, saveErr))
	}
}

func (a *App) toClipboard(png []byte) error {
	if a.clip == nil {
		c, err := clipboard.New()
		if err != nil {
			return err
		}
		a.clip = c
	}
	return a.clip.SetPNG(png)
}

func (a *App) shotFailed(title string, err error) {
	debugf("screenshot failed: %v", err)
	a.showTag(title)
	notify(title, err.Error())
}

// openShots opens the screenshot folder, creating it first so the file
// manager has something to show (figure tray-menu-m3 1).
func (a *App) openShots() {
	dir := shot.Dir(a.cfg().ScreenshotDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		notify("Could not open the screenshot folder", err.Error())
		return
	}
	a.editFile(dir)
}

// chooseShotsDir picks the screenshot folder with a folder picker and writes
// it to config.json as screenshot_dir.
func (a *App) chooseShotsDir() {
	go func() {
		cur := shot.Dir(a.cfg().ScreenshotDir)
		os.MkdirAll(cur, 0o755) // so the picker can open in it
		dir, ok, err := askFolder(cur)
		if err != nil {
			notify("Could not show the folder picker", err.Error())
			return
		}
		if !ok {
			return
		}
		a.mu.Lock()
		a.conf.ScreenshotDir = tildePath(dir)
		c := a.conf
		a.mu.Unlock()
		if err := config.SaveConfig(a.configPath, c); err != nil {
			notify("Could not save the screenshot folder", err.Error())
			return
		}
		notify("Screenshot folder changed", "Screenshots now go to "+tildePath(dir))
		a.refresh()
	}()
}

func (a *App) setHintsShown(on bool) {
	a.mu.Lock()
	a.conf.ShowHints = on
	c := a.conf
	a.mu.Unlock()
	if err := config.SaveConfig(a.configPath, c); err != nil {
		notify("Could not save the hints setting", err.Error())
	}
	a.refresh()
}

// hintLabel is what each shortcut does, as the hint bar says it. The pin
// hint names what the key will do next (figure screen-hints-pinned 1).
func hintLabel(name string, live, pinned bool) string {
	switch name {
	case "toggle":
		if live {
			return "stop camera"
		}
		return "start camera"
	case "pin":
		if pinned {
			return "follow mouse"
		}
		return "pin & resize"
	case "save":
		return "save preset"
	}
	return name // bigger, smaller, aspect, screenshot
}

// hintsFor turns config.json's "hints" into the bar's reminders. Names that
// aren't shortcuts come back in unknown; a switched-off shortcut is skipped.
func hintsFor(c config.Config, live, pinned bool) (hints []draw.Hint, unknown []string) {
	keys := c.Shortcuts.ByName()
	for _, n := range c.Hints {
		k, ok := keys[n]
		if !ok {
			unknown = append(unknown, n)
			continue
		}
		if k != "" {
			hints = append(hints, draw.Hint{Key: k, Label: hintLabel(n, live, pinned)})
		}
	}
	return hints, unknown
}

// updateHints refreshes the hint bar, and says once per change when
// "hints" names something that isn't a shortcut.
func (a *App) updateHints() {
	if a.ui == nil {
		return
	}
	a.mu.Lock()
	c, live, pinned := a.conf, a.live, a.pinned
	a.mu.Unlock()
	hints, unknown := hintsFor(c, live, pinned)
	a.ui.SetHints(hints, c.ShowHints)

	bad := strings.Join(unknown, ", ")
	a.mu.Lock()
	changed := bad != a.badHints
	a.badHints = bad
	a.mu.Unlock()
	if changed && bad != "" {
		names := make([]string, 0, 7)
		for n := range c.Shortcuts.ByName() {
			names = append(names, n)
		}
		sort.Strings(names)
		notify("Unknown hint in config.json", fmt.Sprintf("%q is not a shortcut name. Use one of: %s.", bad, strings.Join(names, ", ")))
	}
}
