package app

import (
	"fmt"

	"fyne.io/systray"

	"github.com/gostega/screen-cam/internal/draw"
	"github.com/gostega/screen-cam/internal/viewport"
)

// maxPresetSlots is how many preset items the menu can show. Menu items are
// created once and shown or hidden, because the tray protocol appends new
// items at the end of the menu rather than where the presets are.
const maxPresetSlots = 20

// tray is the menu in figure tray-menu (design review v2). Shortcut hints sit
// in the item titles: the tray protocol has no accelerator column.
type tray struct {
	status, toggle                        *systray.MenuItem
	presets                               [maxPresetSlots]*systray.MenuItem
	save, editPresets, editShortcuts      *systray.MenuItem
	bigger, smaller, aspect               *systray.MenuItem
	follow, outline, login, version, quit *systray.MenuItem
	iconLive                              bool
	iconSet                               bool
}

func runTray(a *App) {
	systray.Run(func() { a.buildTray() }, func() {})
}

func (a *App) buildTray() {
	t := &tray{}
	a.tray = t
	systray.SetIcon(draw.TrayIcon(false))
	systray.SetTitle("Screen Cam")
	systray.SetTooltip("Screen Cam")
	systray.SetOnTapped(a.toggle) // left-click toggles (tray-menu 1)

	t.status = systray.AddMenuItem("", "")
	t.status.Disable()
	t.toggle = systray.AddMenuItem("", "")
	systray.AddSeparator()
	head := systray.AddMenuItem("Presets", "")
	head.Disable()
	for i := range t.presets {
		t.presets[i] = systray.AddMenuItemCheckbox("", "", false)
		t.presets[i].Hide()
	}
	t.save = systray.AddMenuItem("", "")
	t.editPresets = systray.AddMenuItem("Edit presets file…", a.presetsPath)
	t.editShortcuts = systray.AddMenuItem("Edit shortcuts file…", a.configPath)
	systray.AddSeparator()
	t.bigger = systray.AddMenuItem("", "")
	t.smaller = systray.AddMenuItem("", "")
	t.aspect = systray.AddMenuItem("", "")
	t.follow = systray.AddMenuItemCheckbox("", "", true)
	t.outline = systray.AddMenuItemCheckbox("Show outline", "", true)
	t.login = systray.AddMenuItemCheckbox("Start at login", "", false)
	systray.AddSeparator()
	t.version = systray.AddMenuItem("", "")
	t.version.Disable()
	t.quit = systray.AddMenuItem("Quit", "")

	go a.trayClicks()
	a.refresh()
}

func (a *App) trayClicks() {
	t := a.tray
	presetClicks := make(chan int)
	for i, it := range t.presets {
		go func() {
			for range it.ClickedCh {
				presetClicks <- i
			}
		}()
	}
	for {
		select {
		case <-t.toggle.ClickedCh:
			a.toggle()
		case i := <-presetClicks:
			a.selectPreset(i)
		case <-t.save.ClickedCh:
			a.savePreset()
		case <-t.editPresets.ClickedCh:
			a.editFile(a.presetsPath)
		case <-t.editShortcuts.ClickedCh:
			a.editFile(a.configPath)
		case <-t.bigger.ClickedCh:
			a.scale(1.1)
		case <-t.smaller.ClickedCh:
			a.scale(1 / 1.1)
		case <-t.aspect.ClickedCh:
			a.nextAspect()
		case <-t.follow.ClickedCh:
			a.togglePin()
		case <-t.outline.ClickedCh:
			a.setOutline(!t.outline.Checked())
		case <-t.login.ClickedCh:
			a.setAutostart(!t.login.Checked())
		case <-t.quit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func hint(title, key string) string {
	if key == "" {
		return title
	}
	return title + "    " + key
}

// refresh brings every menu item and the icon in line with the state.
func (a *App) refresh() {
	a.logState()
	t := a.tray
	if t == nil {
		return
	}
	a.mu.Lock()
	live, pinned, camErr := a.live, a.pinned, a.camErr
	w, h, active := a.w, a.h, a.active
	presets := append(a.presets[:0:0], a.presets...)
	sc := a.conf.Shortcuts
	outlineOn := a.conf.ShowOutline
	aspect := viewport.Aspects[viewport.AspectIndex(viewport.Rect{W: w, H: h})]
	a.mu.Unlock()

	mode := "following mouse"
	if pinned {
		mode = "pinned"
	}
	switch {
	case camErr != nil:
		t.status.SetTitle("Camera unavailable: " + camErr.Error())
	case live:
		t.status.SetTitle(fmt.Sprintf("Live · %d×%d · %s", w, h, mode))
	default:
		t.status.SetTitle(fmt.Sprintf("Off · %d×%d · %s", w, h, mode))
	}
	if live {
		t.toggle.SetTitle(hint("Stop camera", sc.Toggle))
	} else {
		t.toggle.SetTitle(hint("Start camera", sc.Toggle))
	}
	for i, it := range t.presets {
		if i >= len(presets) {
			it.Hide()
			continue
		}
		key := ""
		if i < 9 {
			key = fmt.Sprintf("%s+%d", sc.PresetMods, i+1)
		}
		it.SetTitle(hint(presets[i].Label(), key))
		if i == active {
			it.Check()
		} else {
			it.Uncheck()
		}
		it.Show()
	}
	t.save.SetTitle(hint("Save current size as preset…", sc.Save))
	t.bigger.SetTitle(hint("Bigger", sc.Bigger))
	t.smaller.SetTitle(hint("Smaller", sc.Smaller))
	next := viewport.Aspects[(viewport.AspectIndex(viewport.Rect{W: w, H: h})+1)%len(viewport.Aspects)]
	t.aspect.SetTitle(hint(fmt.Sprintf("Aspect: %d:%d  ▸ %d:%d", aspect[0], aspect[1], next[0], next[1]), sc.Aspect))
	t.follow.SetTitle(hint("Follow mouse", sc.Pin+" pins"))
	setChecked(t.follow, !pinned)
	setChecked(t.outline, outlineOn)
	setChecked(t.login, autostartEnabled())
	t.version.SetTitle(fmt.Sprintf("screen-cam %s · %s", a.Version, a.cfg().Device))

	if !t.iconSet || t.iconLive != live {
		systray.SetIcon(draw.TrayIcon(live))
		t.iconLive, t.iconSet = live, true
	}
}

func setChecked(it *systray.MenuItem, on bool) {
	if on {
		it.Check()
	} else {
		it.Uncheck()
	}
}
