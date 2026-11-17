package app

import (
	"reflect"
	"testing"

	"github.com/gostega/screen-cam/internal/config"
	"github.com/gostega/screen-cam/internal/draw"
)

func TestHintsFor(t *testing.T) {
	c := config.Default()
	got, unknown := hintsFor(c, true, false)
	want := []draw.Hint{{Key: "Ctrl+Alt+P", Label: "pin & resize"}, {Key: "Ctrl+Alt+X", Label: "screenshot"}}
	if !reflect.DeepEqual(got, want) || unknown != nil {
		t.Fatalf("defaults: %v %v", got, unknown)
	}
	// Pinned, the same key's hint says what it does next.
	if got, _ := hintsFor(c, true, true); got[0].Label != "follow mouse" {
		t.Fatalf("pinned pin hint = %q", got[0].Label)
	}
	// A rebound key shows its new combination; an unknown name is reported;
	// a switched-off shortcut is left out.
	c.Shortcuts.Pin = "Ctrl+Alt+L"
	c.Shortcuts.Save = ""
	c.Hints = []string{"pin", "nope", "save", "toggle"}
	got, unknown = hintsFor(c, false, false)
	want = []draw.Hint{{Key: "Ctrl+Alt+L", Label: "pin & resize"}, {Key: "Ctrl+Alt+C", Label: "start camera"}}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(unknown, []string{"nope"}) {
		t.Fatalf("edited: %v unknown %v", got, unknown)
	}
}

func TestTildePath(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	for in, want := range map[string]string{"/home/u/Pictures/x": "~/Pictures/x", "/home/u": "~", "/data/shots": "/data/shots", "/home/user2": "/home/user2"} {
		if got := tildePath(in); got != want {
			t.Errorf("tildePath(%s) = %s, want %s", in, got, want)
		}
	}
}
