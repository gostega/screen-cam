package clipboard

import (
	"bytes"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// These tests take the clipboard, so they run only on a display set aside for
// them: SCREEN_CAM_TEST_DISPLAY must equal DISPLAY (scripts/e2e.sh runs them
// on its Xvfb). xclip is the reader, so the protocol is checked by a client
// that doesn't share this package's assumptions.
func display(t *testing.T) {
	t.Helper()
	d := os.Getenv("SCREEN_CAM_TEST_DISPLAY")
	if d == "" || d != os.Getenv("DISPLAY") {
		t.Skip("needs SCREEN_CAM_TEST_DISPLAY=$DISPLAY on a throwaway display")
	}
	if _, err := exec.LookPath("xclip"); err != nil {
		t.Skip("needs xclip")
	}
}

func paste(t *testing.T, target string) []byte {
	t.Helper()
	out, err := exec.Command("xclip", "-o", "-selection", "clipboard", "-t", target).Output()
	if err != nil {
		t.Fatalf("xclip -t %s: %v", target, err)
	}
	return out
}

func TestServesPNG(t *testing.T) {
	display(t)
	o, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	for _, n := range []int{100, Chunk, Chunk + 1, 5*Chunk + 123} { // direct, edge, INCR
		data := make([]byte, n)
		rand.New(rand.NewSource(int64(n))).Read(data)
		if err := o.SetPNG(data); err != nil {
			t.Fatal(err)
		}
		if got := paste(t, "image/png"); !bytes.Equal(got, data) {
			t.Fatalf("%d bytes: pasted %d bytes, not the same data", n, len(got))
		}
	}
	targets := string(paste(t, "TARGETS"))
	if !strings.Contains(targets, "image/png") {
		t.Fatalf("TARGETS = %q, want image/png listed", targets)
	}
}

func TestRefusesOtherTargets(t *testing.T) {
	display(t)
	o, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if err := o.SetPNG([]byte("png")); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("xclip", "-o", "-selection", "clipboard", "-t", "UTF8_STRING").Output(); err == nil && len(out) > 0 {
		t.Fatalf("UTF8_STRING pasted %q; want a refusal", out)
	}
}
