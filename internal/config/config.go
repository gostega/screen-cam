// Package config holds screen-cam's two user-editable files:
//
//	~/.config/screen-cam/config.json   device, frame rate, shortcuts, hints,
//	                                   screenshot folder
//	~/.config/screen-cam/presets.json  named viewport sizes
//
// Both are plain JSON that the user edits by hand ("Edit … file…" in the tray);
// the app reloads them when they change, so a bad edit must never crash it —
// Load reports the problem and the caller keeps the last good copy.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Shortcuts maps each action to a key combination such as "Ctrl+Alt+C".
// PresetMods + 1..9 selects presets; ScrollMods + wheel resizes.
type Shortcuts struct {
	Toggle     string `json:"toggle"`
	Bigger     string `json:"bigger"`
	Smaller    string `json:"smaller"`
	Aspect     string `json:"aspect"`
	Pin        string `json:"pin"`
	Save       string `json:"save"`
	Screenshot string `json:"screenshot"`
	PresetMods string `json:"preset_modifiers"`
	ScrollMods string `json:"scroll_modifiers"`
}

// Config is config.json.
type Config struct {
	Device      string `json:"device"`
	FPS         int    `json:"fps"`
	ShowOutline bool   `json:"show_outline"`
	// ShowHints and Hints are the shortcut reminders under the frame
	// (figure screen-hints): Hints names entries of Shortcuts, in order.
	ShowHints bool     `json:"show_hints"`
	Hints     []string `json:"hints"`
	// ScreenshotDir is where screenshots are saved; a leading ~ is home.
	ScreenshotDir string    `json:"screenshot_dir"`
	Shortcuts     Shortcuts `json:"shortcuts"`
}

// Default is what a first run writes.
func Default() Config {
	return Config{
		Device: "/dev/video10", FPS: 30, ShowOutline: true,
		ShowHints: true, Hints: []string{"pin", "screenshot"},
		ScreenshotDir: "~/Pictures/ScreenCam",
		Shortcuts: Shortcuts{
			Toggle: "Ctrl+Alt+C", Bigger: "Ctrl+Alt+=", Smaller: "Ctrl+Alt+-",
			Aspect: "Ctrl+Alt+A", Pin: "Ctrl+Alt+P", Save: "Ctrl+Alt+S",
			Screenshot: "Ctrl+Alt+X",
			PresetMods: "Ctrl+Alt", ScrollMods: "Ctrl+Alt",
		},
	}
}

// Preset is a named viewport size. Position is deliberately not saved (D4).
type Preset struct {
	Name string `json:"name"`
	W    int    `json:"width"`
	H    int    `json:"height"`
}

// Label is how the tray and notifications show a preset.
func (p Preset) Label() string { return fmt.Sprintf("%s · %d×%d", p.Name, p.W, p.H) }

// DefaultPresets are the three sizes from the design review.
func DefaultPresets() []Preset {
	return []Preset{
		{"Phone screen", 360, 640},
		{"Single label", 640, 480},
		{"Label sheet row", 1200, 400},
	}
}

// Dir is ~/.config/screen-cam (or $XDG_CONFIG_HOME/screen-cam).
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "screen-cam"), nil
}

// ByName is each shortcut by its JSON name — the names "hints" refers to.
func (s Shortcuts) ByName() map[string]string {
	return map[string]string{
		"toggle": s.Toggle, "bigger": s.Bigger, "smaller": s.Smaller, "aspect": s.Aspect,
		"pin": s.Pin, "save": s.Save, "screenshot": s.Screenshot,
	}
}

// LoadConfig reads path, writing the defaults there first if it is missing.
// Fields left out of the file keep their defaults, and are written into it,
// so a setting added by an upgrade shows up in the file for editing.
func LoadConfig(path string) (Config, error) {
	c := Default()
	if err := loadOrCreate(path, &c); err != nil {
		return c, err
	}
	if err := c.validate(); err != nil {
		return c, err
	}
	if b, err := os.ReadFile(path); err == nil && missingKeys(b, c) {
		if err := writeJSON(path, c); err != nil {
			return c, err
		}
	}
	return c, nil
}

// missingKeys reports whether file lacks a key that c would write.
func missingKeys(file []byte, c Config) bool {
	var have, want map[string]json.RawMessage
	full, _ := json.Marshal(c)
	if json.Unmarshal(file, &have) != nil || json.Unmarshal(full, &want) != nil {
		return false
	}
	for k := range want {
		if _, ok := have[k]; !ok {
			return true
		}
	}
	var haveS, wantS map[string]json.RawMessage
	json.Unmarshal(have["shortcuts"], &haveS)
	json.Unmarshal(want["shortcuts"], &wantS)
	for k := range wantS {
		if _, ok := haveS[k]; !ok {
			return true
		}
	}
	return false
}

func (c Config) validate() error {
	if c.FPS < 1 || c.FPS > 60 {
		return fmt.Errorf("fps %d: want 1-60", c.FPS)
	}
	for name, s := range c.Shortcuts.ByName() {
		if s == "" {
			continue // an empty string switches that shortcut off
		}
		if _, err := ParseKey(s); err != nil {
			return fmt.Errorf("shortcut %s: %w", name, err)
		}
	}
	for name, s := range map[string]string{"preset_modifiers": c.Shortcuts.PresetMods, "scroll_modifiers": c.Shortcuts.ScrollMods} {
		if _, err := ParseMods(s); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

// LoadPresets reads path, writing DefaultPresets there first if it is missing.
func LoadPresets(path string) ([]Preset, error) {
	ps := DefaultPresets()
	if err := loadOrCreate(path, &ps); err != nil {
		return nil, err
	}
	for i, p := range ps {
		if strings.TrimSpace(p.Name) == "" || p.W < 64 || p.H < 64 {
			return nil, fmt.Errorf("preset %d (%q %dx%d): needs a name and both sides at least 64", i+1, p.Name, p.W, p.H)
		}
	}
	return ps, nil
}

// SavePresets writes ps to path, replacing it atomically so a reader never
// sees half a file.
func SavePresets(path string, ps []Preset) error { return writeJSON(path, ps) }

func loadOrCreate(path string, v any) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return writeJSON(path, v)
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SaveConfig writes c to path (used when a tray toggle changes a setting).
func SaveConfig(path string, c Config) error { return writeJSON(path, c) }
