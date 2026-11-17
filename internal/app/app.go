// Package app is the screen-cam tray app: it ties the camera loop, the
// on-screen outline and shortcuts (xui) and the tray menu together, and owns
// the state they share. Behaviour follows design review v2
// (docs/design.md §Design review).
package app

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/gostega/screen-cam/internal/capture"
	"github.com/gostega/screen-cam/internal/clipboard"
	"github.com/gostega/screen-cam/internal/config"
	"github.com/gostega/screen-cam/internal/v4l2"
	"github.com/gostega/screen-cam/internal/viewport"
	"github.com/gostega/screen-cam/internal/xui"
)

// App is the running tray app.
type App struct {
	Version string

	mu       sync.Mutex
	conf     config.Config
	presets  []config.Preset
	active   int // index into presets, or -1 for a size no preset has
	live     bool
	pinned   bool
	dragging bool
	w, h     int // viewport size
	rect     viewport.Rect
	screenW  int
	screenH  int
	camErr   error
	tagTimer *time.Timer
	badHints string // unknown names in "hints", last reported

	shotMu sync.Mutex // one screenshot at a time
	shotX  *capture.X // the screenshot's own X connection, made on first use
	clip   *clipboard.Owner

	configPath, presetsPath string
	ui                      *xui.UI
	tray                    *tray
	stopCam                 chan struct{}
}

// New loads the config and presets (writing defaults on first run).
func New(version string) (*App, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	a := &App{Version: version, active: 1,
		configPath: filepath.Join(dir, "config.json"), presetsPath: filepath.Join(dir, "presets.json")}
	if a.conf, err = config.LoadConfig(a.configPath); err != nil {
		return nil, fmt.Errorf("%s: %w — fix or delete it", a.configPath, err)
	}
	if a.presets, err = config.LoadPresets(a.presetsPath); err != nil {
		return nil, fmt.Errorf("%s: %w — fix or delete it", a.presetsPath, err)
	}
	a.w, a.h = 640, 480
	a.active = a.matchPreset()
	return a, nil
}

// Run starts everything and blocks until Quit. It must be called from main.
// With tray false (screen-cam --no-tray) it runs without the tray icon until
// SIGINT/SIGTERM — for checks on a virtual display, where there is no tray.
func (a *App) Run(withTray bool) error {
	release, err := singleInstance()
	if err != nil {
		return err
	}
	defer release()

	a.ui, err = xui.New(xui.Events{Shortcut: a.onShortcut, Scroll: a.onScroll, Drag: a.onDrag})
	if err != nil {
		return err
	}
	defer a.ui.Close()
	a.screenW, a.screenH = a.ui.ScreenSize()
	a.ui.SetOutline(a.conf.ShowOutline)
	a.bindKeys()
	go a.ui.Run()
	go a.watchFiles()

	a.startCamera()
	if withTray {
		runTray(a) // blocks on the main thread until Quit
	} else {
		a.refresh()
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
	}
	if a.stopCam != nil {
		close(a.stopCam)
	}
	return nil
}

func (a *App) cfg() config.Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.conf
}

// startCamera opens the device and starts the frame loop. The camera starts
// OFF (D5); the loop writes the off card until the user starts it.
func (a *App) startCamera() {
	x, err := capture.Connect()
	if err == nil {
		var cam *v4l2.Device
		cam, err = v4l2.Open(a.cfg().Device, OutW, OutH)
		if err == nil {
			a.stopCam = make(chan struct{})
			go a.runCamera(cam, x, a.stopCam)
		} else {
			x.Close()
		}
	}
	a.mu.Lock()
	a.camErr = err
	a.mu.Unlock()
	if err != nil {
		notify("Camera device unavailable", err.Error()+"\nSet up the v4l2loopback module (see the README), or close whatever else is writing to it, then restart Screen Cam.")
	}
}

func (a *App) cameraFailed(err error) {
	a.mu.Lock()
	a.live, a.camErr = false, err
	a.mu.Unlock()
	a.ui.Hide()
	notify("Screen Cam stopped", err.Error())
	a.refresh()
}

// ---- actions (tray items and shortcuts share these) ----------------------

func (a *App) toggle() {
	a.mu.Lock()
	if a.camErr != nil {
		err := a.camErr
		a.mu.Unlock()
		notify("Camera device unavailable", err.Error())
		return
	}
	a.live = !a.live
	live := a.live
	if live && a.pinned && a.rect.W == 0 {
		a.rect = viewport.CenteredOn(a.screenW/2, a.screenH/2, a.w, a.h, a.screenW, a.screenH)
	}
	a.mu.Unlock()
	if !live {
		a.ui.Hide()
	}
	a.refresh()
}

func (a *App) selectPreset(i int) {
	a.mu.Lock()
	if i < 0 || i >= len(a.presets) {
		a.mu.Unlock()
		return
	}
	p := a.presets[i]
	a.w, a.h, a.active = p.W, p.H, i
	a.resizePinnedLocked()
	startLive := !a.live && a.camErr == nil // a preset key also starts the camera
	a.live = a.live || startLive
	a.mu.Unlock()
	a.showTag(p.Label())
	a.refresh()
}

// resizePinnedLocked fits a pinned rect to the current size, keeping its centre.
func (a *App) resizePinnedLocked() {
	if a.pinned && a.rect.W > 0 {
		a.rect = viewport.CenteredOn(a.rect.X+a.rect.W/2, a.rect.Y+a.rect.H/2, a.w, a.h, a.screenW, a.screenH)
		a.ui.Move(a.rect)
	}
}

// currentRectLocked is the viewport resizes start from. Pinned, that is the
// frame itself. Following, the size is a.w×a.h and a.rect is only where the
// last frame was drawn — it can lag a resize by up to one frame, so two quick
// resizes must not compound from it.
func (a *App) currentRectLocked() viewport.Rect {
	if a.pinned && a.rect.W > 0 {
		return a.rect
	}
	cx, cy := a.screenW/2, a.screenH/2
	if a.rect.W > 0 {
		cx, cy = a.rect.X+a.rect.W/2, a.rect.Y+a.rect.H/2
	}
	return viewport.CenteredOn(cx, cy, a.w, a.h, a.screenW, a.screenH)
}

func (a *App) scale(f float64) {
	a.mu.Lock()
	r := viewport.Scaled(a.currentRectLocked(), f, a.screenW, a.screenH)
	a.applyRectLocked(r)
	a.mu.Unlock()
	a.showSizeTag()
	a.refresh()
}

func (a *App) nextAspect() {
	a.mu.Lock()
	r := viewport.NextAspect(a.currentRectLocked(), a.screenW, a.screenH)
	a.applyRectLocked(r)
	a.mu.Unlock()
	a.showSizeTag()
	a.refresh()
}

func (a *App) applyRectLocked(r viewport.Rect) {
	a.w, a.h = r.W, r.H
	if a.pinned {
		a.rect = r
		a.ui.Move(r)
	}
	a.active = a.matchPreset()
}

func (a *App) togglePin() {
	a.mu.Lock()
	a.pinned = !a.pinned
	if a.pinned && a.rect.W == 0 {
		a.rect = a.currentRectLocked()
	}
	pinned := a.pinned
	a.mu.Unlock()
	a.ui.SetPinned(pinned)
	a.refresh()
}

func (a *App) setOutline(on bool) {
	a.mu.Lock()
	a.conf.ShowOutline = on
	c := a.conf
	a.mu.Unlock()
	a.ui.SetOutline(on)
	if err := config.SaveConfig(a.configPath, c); err != nil {
		notify("Could not save the outline setting", err.Error())
	}
	a.refresh()
}

func (a *App) setAutostart(on bool) {
	if err := setAutostart(on); err != nil {
		notify("Could not change Start at login", err.Error())
	}
	a.refresh()
}

// savePreset asks for a name and appends the current size (figure save-preset).
func (a *App) savePreset() {
	a.mu.Lock()
	w, h := a.w, a.h
	a.mu.Unlock()
	size := fmt.Sprintf("%d×%d", w, h)
	go func() {
		name, ok, err := askName(size, size)
		if err != nil {
			notify("Could not ask for a name", err.Error())
			return
		}
		if !ok {
			return
		}
		a.mu.Lock()
		ps := append(append([]config.Preset{}, a.presets...), config.Preset{Name: name, W: w, H: h})
		a.mu.Unlock()
		if err := config.SavePresets(a.presetsPath, ps); err != nil {
			notify("Could not save the preset", err.Error())
			return
		}
		a.mu.Lock()
		a.presets, a.active = ps, len(ps)-1
		n, mods := len(ps), a.conf.Shortcuts.PresetMods
		a.mu.Unlock()
		key := "no shortcut (only the first nine get one)"
		if n <= 9 {
			key = fmt.Sprintf("%s+%d", mods, n)
		}
		notify("Preset saved", fmt.Sprintf("\"%s\" · %s · %s", name, size, key))
		a.refresh()
	}()
}

func (a *App) editFile(path string) {
	if err := openFile(path); err != nil {
		notify("Could not open "+filepath.Base(path), err.Error())
	}
}

// ---- events from xui -----------------------------------------------------

func (a *App) onShortcut(action string) {
	switch action {
	case "toggle":
		a.toggle()
	case "bigger":
		a.scale(1.1)
	case "smaller":
		a.scale(1 / 1.1)
	case "aspect":
		a.nextAspect()
	case "pin":
		a.togglePin()
	case "save":
		a.savePreset()
	case "screenshot":
		a.screenshot()
	default:
		var n int
		if _, err := fmt.Sscanf(action, "preset%d", &n); err == nil {
			a.selectPreset(n - 1)
		}
	}
}

func (a *App) onScroll(up bool) {
	if up {
		a.scale(1.1)
	} else {
		a.scale(1 / 1.1)
	}
}

func (a *App) onDrag(r viewport.Rect, done bool) {
	a.mu.Lock()
	a.dragging = !done
	a.rect, a.w, a.h = r, r.W, r.H
	a.active = a.matchPreset()
	a.mu.Unlock()
	a.ui.Move(r)
	if done {
		a.refresh()
	}
}

// ---- helpers -------------------------------------------------------------

// matchPreset is the preset with the current size, or -1. Called with a.mu held.
func (a *App) matchPreset() int {
	for i, p := range a.presets {
		if p.W == a.w && p.H == a.h {
			return i
		}
	}
	return -1
}

func (a *App) showSizeTag() {
	a.mu.Lock()
	text := fmt.Sprintf("%d×%d", a.w, a.h)
	if a.active >= 0 {
		text = a.presets[a.active].Label()
	}
	a.mu.Unlock()
	a.showTag(text)
}

// showTag shows text on the frame for 2 seconds (figure screen-live 2).
func (a *App) showTag(text string) {
	debugf("tag %s", text)
	a.ui.SetTag(text)
	a.mu.Lock()
	if a.tagTimer != nil {
		a.tagTimer.Stop()
	}
	a.tagTimer = time.AfterFunc(2*time.Second, func() { a.ui.SetTag("") })
	a.mu.Unlock()
}

// bindKeys (re)grabs every shortcut from the config and reports any that
// another program holds (TASK-5).
func (a *App) bindKeys() {
	c := a.cfg()
	keys := map[string]config.Key{}
	add := func(action, s string) {
		if s == "" {
			return
		}
		if k, err := config.ParseKey(s); err == nil {
			keys[action] = k
		}
	}
	add("toggle", c.Shortcuts.Toggle)
	add("bigger", c.Shortcuts.Bigger)
	add("smaller", c.Shortcuts.Smaller)
	add("aspect", c.Shortcuts.Aspect)
	add("pin", c.Shortcuts.Pin)
	add("save", c.Shortcuts.Save)
	add("screenshot", c.Shortcuts.Screenshot)
	for n := 1; n <= 9; n++ {
		if k, err := config.PresetKey(c.Shortcuts.PresetMods, n); err == nil {
			keys[fmt.Sprintf("preset%d", n)] = k
		}
	}
	scroll, _ := config.ParseMods(c.Shortcuts.ScrollMods)
	if problems := a.ui.Bind(keys, scroll); len(problems) > 0 {
		body := ""
		for _, p := range problems {
			body += p + "\n"
		}
		notify("Some Screen Cam shortcuts are unavailable", body+"Change them with Edit shortcuts file… in the tray menu.")
	}
}

// watchFiles reloads config.json and presets.json when they change. A bad
// edit is reported and the last good copy kept.
func (a *App) watchFiles() {
	mtime := func(p string) time.Time {
		if fi, err := os.Stat(p); err == nil {
			return fi.ModTime()
		}
		return time.Time{}
	}
	lastC, lastP := mtime(a.configPath), mtime(a.presetsPath)
	for range time.Tick(time.Second) {
		if m := mtime(a.configPath); !m.Equal(lastC) {
			lastC = m
			if c, err := config.LoadConfig(a.configPath); err != nil {
				notify("config.json not loaded", err.Error()+"\nKept the previous settings.")
			} else {
				a.mu.Lock()
				a.conf = c
				a.mu.Unlock()
				a.ui.SetOutline(c.ShowOutline)
				a.bindKeys()
				a.refresh()
			}
		}
		if m := mtime(a.presetsPath); !m.Equal(lastP) {
			lastP = m
			if ps, err := config.LoadPresets(a.presetsPath); err != nil {
				notify("presets.json not loaded", err.Error()+"\nKept the previous presets.")
			} else {
				a.mu.Lock()
				a.presets = ps
				a.active = a.matchPreset()
				a.mu.Unlock()
				a.refresh()
			}
		}
	}
}
