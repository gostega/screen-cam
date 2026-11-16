package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseKey(t *testing.T) {
	cases := map[string]Key{
		"Ctrl+Alt+C":     {Ctrl | Alt, 'c', "Ctrl+Alt+C"},
		"ctrl+alt+=":     {Ctrl | Alt, 0x3d, "ctrl+alt+="},
		"Ctrl+Alt+-":     {Ctrl | Alt, 0x2d, "Ctrl+Alt+-"},
		"Ctrl+Alt+minus": {Ctrl | Alt, 0x2d, "Ctrl+Alt+minus"},
		"Super+Shift+F5": {Super | Shift, 0xffc2, "Super+Shift+F5"},
		"Ctrl+Alt+7":     {Ctrl | Alt, '7', "Ctrl+Alt+7"},
	}
	for in, want := range cases {
		got, err := ParseKey(in)
		if err != nil || got != want {
			t.Errorf("ParseKey(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, bad := range []string{"C", "Ctrl+", "Hyper+C", "Ctrl+Alt+Banana", "+C"} {
		if _, err := ParseKey(bad); err == nil {
			t.Errorf("ParseKey(%q) accepted a bad shortcut", bad)
		}
	}
}

func TestDefaultShortcutsAllParse(t *testing.T) {
	if err := Default().validate(); err != nil {
		t.Fatal(err)
	}
	for n := 1; n <= 9; n++ {
		if _, err := PresetKey(Default().Shortcuts.PresetMods, n); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFirstRunWritesDefaultsAndReadsThemBack(t *testing.T) {
	dir := t.TempDir()
	cp, pp := filepath.Join(dir, "config.json"), filepath.Join(dir, "presets.json")
	c, err := LoadConfig(cp)
	if err != nil || c != Default() {
		t.Fatalf("first load: %+v %v", c, err)
	}
	ps, err := LoadPresets(pp)
	if err != nil || len(ps) != 3 || ps[1].Name != "Single label" {
		t.Fatalf("first presets: %+v %v", ps, err)
	}
	for _, p := range []string{cp, pp} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s not written: %v", p, err)
		}
	}
}

func TestPartialConfigKeepsDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(p, []byte(`{"fps": 15, "shortcuts": {"toggle": "Super+F9"}}`), 0o644)
	c, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.FPS != 15 || c.Shortcuts.Toggle != "Super+F9" || c.Device != "/dev/video10" || c.Shortcuts.Pin != "Ctrl+Alt+P" {
		t.Errorf("got %+v", c)
	}
}

func TestBadEditsAreReportedNotFatal(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"config.json":  `{"shortcuts": {"pin": "Ctrl+Alt+Banana"}}`,
		"broken.json":  `{"fps": 30,`,
		"presets.json": `[{"name": "", "width": 640, "height": 480}]`,
	}
	for name, body := range cases {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(body), 0o644)
		var err error
		if strings.HasPrefix(name, "presets") {
			_, err = LoadPresets(p)
		} else {
			_, err = LoadConfig(p)
		}
		if err == nil {
			t.Errorf("%s: bad content accepted", name)
		}
	}
}

func TestSavePresetsRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "presets.json")
	want := append(DefaultPresets(), Preset{"Shelf close-up", 800, 600})
	if err := SavePresets(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPresets(p)
	if err != nil || len(got) != 4 || got[3] != want[3] {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	if got[3].Label() != "Shelf close-up · 800×600" {
		t.Errorf("label = %q", got[3].Label())
	}
}
