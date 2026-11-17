package shot

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPNGColours(t *testing.T) {
	// Two pixels, BGRA: pure blue, then pure red.
	data, err := PNG([]byte{0xff, 0, 0, 0, 0, 0, 0xff, 0}, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, a := img.At(0, 0).RGBA()
	if r != 0 || g != 0 || b != 0xffff || a != 0xffff {
		t.Fatalf("pixel 0 = %x %x %x %x, want opaque blue", r, g, b, a)
	}
	r, _, b, _ = img.At(1, 0).RGBA()
	if r != 0xffff || b != 0 {
		t.Fatalf("pixel 1 = r%x b%x, want red", r, b)
	}
	if _, err := PNG([]byte{1, 2, 3}, 2, 1); err == nil {
		t.Fatal("short capture accepted")
	}
}

func TestSaveNamesAndCollisions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ScreenCam")
	at := time.Date(2026, 11, 17, 9, 41, 7, 0, time.Local)
	p1, err := Save([]byte("a"), dir, at)
	if err != nil {
		t.Fatal(err)
	}
	p2, _ := Save([]byte("b"), dir, at)
	if filepath.Base(p1) != "screen-cam_2026-11-17_09-41-07.png" || filepath.Base(p2) != "screen-cam_2026-11-17_09-41-07-2.png" {
		t.Fatalf("names %s, %s", filepath.Base(p1), filepath.Base(p2))
	}
	if b, _ := os.ReadFile(p1); string(b) != "a" {
		t.Fatal("second save overwrote the first")
	}
}

func TestDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := Dir("~/Shots"); got != filepath.Join(home, "Shots") {
		t.Fatalf("Dir(~/Shots) = %s", got)
	}
	if got := Dir("/tmp/x"); got != "/tmp/x" {
		t.Fatalf("Dir(/tmp/x) = %s", got)
	}
	if got := Dir(""); !strings.HasSuffix(got, "/ScreenCam") {
		t.Fatalf("Dir(\"\") = %s", got)
	}
}
