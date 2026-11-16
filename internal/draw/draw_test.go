package draw

import (
	"bytes"
	"image/png"
	"testing"
)

func TestTrayIconsDiffer(t *testing.T) {
	off, live := TrayIcon(false), TrayIcon(true)
	for _, b := range [][]byte{off, live} {
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil || img.Bounds().Dx() != 32 {
			t.Fatalf("icon: %v %v", err, img)
		}
	}
	if bytes.Equal(off, live) {
		t.Error("live icon looks the same as the idle one")
	}
	img, _ := png.Decode(bytes.NewReader(live))
	if r, g, _, _ := img.At(25, 6).RGBA(); r>>8 != 0xff || g>>8 != 0x4a {
		t.Errorf("live dot not red at its centre")
	}
}

func TestOffCardHasTextOnBackground(t *testing.T) {
	img := OffCard(1280, 720, "Ctrl+Alt+C")
	if img.Bounds().Dx() != 1280 || img.Bounds().Dy() != 720 {
		t.Fatal("wrong size")
	}
	bg := img.At(5, 5)
	other := 0
	for x := 300; x < 980; x++ {
		if img.At(x, 720/2-55) != bg {
			other++
		}
	}
	if other == 0 {
		t.Error("no text drawn across the middle of the card")
	}
}

func TestTagSize(t *testing.T) {
	tag := Tag("640×480 · Single label")
	w, h := TextSize("640×480 · Single label", 1)
	if tag.Bounds().Dx() != w+8 || tag.Bounds().Dy() != h+8 {
		t.Errorf("tag %v for text %dx%d", tag.Bounds(), w, h)
	}
}
