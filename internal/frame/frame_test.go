package frame

import "testing"

func TestFit(t *testing.T) {
	cases := []struct {
		name       string
		sw, sh     int
		x, y, w, h int
	}{
		{"same shape fills", 640, 360, 0, 0, 1280, 720},
		{"4:3 gets side bars", 640, 480, 160, 0, 960, 720},
		{"wide strip gets top/bottom bars", 1200, 400, 0, 146, 1280, 426},
		{"portrait phone", 360, 640, 438, 0, 404, 720},
	}
	for _, c := range cases {
		x, y, w, h := Fit(c.sw, c.sh, 1280, 720)
		if x != c.x || y != c.y || w != c.w || h != c.h {
			t.Errorf("%s: got %d,%d %dx%d want %d,%d %dx%d", c.name, x, y, w, h, c.x, c.y, c.w, c.h)
		}
	}
}

// solid returns a w×h BGRA picture of one colour.
func solid(w, h int, r, g, b byte) []byte {
	p := make([]byte, w*h*4)
	for i := 0; i < len(p); i += 4 {
		p[i], p[i+1], p[i+2], p[i+3] = b, g, r, 255
	}
	return p
}

func TestRenderLetterboxesAndConverts(t *testing.T) {
	out := NewI420(1280, 720)
	NewScaler(640, 480, out).Render(solid(640, 480, 255, 255, 255), 640*4)
	Y, U, V := out.planes()
	// Inside the picture: white is Y=235, neutral chroma.
	if got := Y[360*1280+640]; got != 235 {
		t.Errorf("centre luma = %d, want 235 (white)", got)
	}
	// The side bars are black: Y=16.
	if got := Y[360*1280+10]; got != 16 {
		t.Errorf("left bar luma = %d, want 16 (black)", got)
	}
	if got := Y[360*1280+1270]; got != 16 {
		t.Errorf("right bar luma = %d, want 16 (black)", got)
	}
	c := 180*640 + 320
	if U[c] != 128 || V[c] != 128 {
		t.Errorf("white chroma = %d,%d, want 128,128", U[c], V[c])
	}
}

func TestRenderRed(t *testing.T) {
	out := NewI420(1280, 720)
	NewScaler(1280, 720, out).Render(solid(1280, 720, 255, 0, 0), 1280*4)
	Y, _, V := out.planes()
	if Y[0] != 82 || V[0] != 240 {
		t.Errorf("red = Y%d V%d, want Y82 V240 (BT.601)", Y[0], V[0])
	}
}

func BenchmarkRender640x480(b *testing.B) {
	out := NewI420(1280, 720)
	s := NewScaler(640, 480, out)
	src := solid(640, 480, 10, 200, 30)
	for b.Loop() {
		s.Render(src, 640*4)
	}
}
