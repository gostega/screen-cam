package viewport

import "testing"

func TestCenteredOnStaysOnScreen(t *testing.T) {
	cases := []struct {
		name   string
		px, py int
		want   Rect
	}{
		{"middle", 1280, 720, Rect{960, 480, 640, 480}},
		{"top-left corner", 5, 5, Rect{0, 0, 640, 480}},
		{"bottom-right corner", 2559, 1439, Rect{1920, 960, 640, 480}},
	}
	for _, c := range cases {
		if got := CenteredOn(c.px, c.py, 640, 480, 2560, 1440); got != c.want {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
}

func TestCenteredOnClampsSize(t *testing.T) {
	if got := CenteredOn(100, 100, 9000, 10, 2560, 1440); got.W != 2560 || got.H != Min {
		t.Errorf("got %+v, want width clamped to screen and height to Min", got)
	}
}

func TestScaledKeepsCentreAndAspect(t *testing.T) {
	r := Rect{960, 480, 640, 480}
	got := Scaled(r, 1.1, 2560, 1440)
	if got.W != 704 || got.H != 528 {
		t.Fatalf("size: got %dx%d want 704x528", got.W, got.H)
	}
	if cx, cy := got.X+got.W/2, got.Y+got.H/2; cx != 1280 || cy != 720 {
		t.Errorf("centre moved to %d,%d", cx, cy)
	}
}

func TestScaledLimitsKeepAspect(t *testing.T) {
	big := Scaled(Rect{0, 0, 1600, 1200}, 2, 2560, 1440)
	if big.H != 1440 || big.W != 1920 {
		t.Errorf("upper limit: got %dx%d want 1920x1440", big.W, big.H)
	}
	small := Scaled(Rect{0, 0, 80, 60}, 0.5, 2560, 1440)
	if small.H != Min || small.W != 85 {
		t.Errorf("lower limit: got %dx%d want 85x%d", small.W, small.H, Min)
	}
}

func TestNextAspectCyclesKeepingWidth(t *testing.T) {
	r := Rect{960, 480, 640, 480} // 4:3
	want := [][2]int{{640, 360}, {640, 640}, {640, 1137}, {640, 480}}
	for i, w := range want {
		r = NextAspect(r, 2560, 1440)
		if r.W != w[0] || r.H != w[1] {
			// 9:16 at width 640 is 1137 tall, which fits a 1440 screen.
			t.Fatalf("step %d: got %dx%d want %dx%d", i+1, r.W, r.H, w[0], w[1])
		}
	}
}

func TestNextAspectShrinksWhenTooTall(t *testing.T) {
	r := NextAspect(Rect{0, 0, 1200, 1200}, 2560, 1440) // 1:1 -> 9:16 would be 2133 tall
	if r.H != 1440 || r.W != 810 {
		t.Errorf("got %dx%d want 810x1440", r.W, r.H)
	}
}

func TestMovedStopsAtEdges(t *testing.T) {
	r := Moved(Rect{100, 100, 640, 480}, -500, 5000, 2560, 1440)
	if r != (Rect{0, 960, 640, 480}) {
		t.Errorf("got %+v", r)
	}
}

func TestDraggedFreeResize(t *testing.T) {
	s := Rect{1000, 500, 640, 480}
	cases := []struct {
		h      Handle
		dx, dy int
		want   Rect
	}{
		{BottomRight, 100, 50, Rect{1000, 500, 740, 530}},
		{TopLeft, -100, -50, Rect{900, 450, 740, 530}},
		{Right, 60, 999, Rect{1000, 500, 700, 480}},           // side handle ignores dy
		{Top, 999, -20, Rect{1000, 480, 640, 500}},            // and dx
		{Left, 5000, 0, Rect{1576, 500, Min, 480}},            // can't cross the other side
		{BottomRight, 9000, 9000, Rect{1000, 500, 1560, 940}}, // stops at the screen
	}
	for _, c := range cases {
		if got := Dragged(s, c.h, c.dx, c.dy, false, 2560, 1440); got != c.want {
			t.Errorf("handle %d by %d,%d: got %+v want %+v", c.h, c.dx, c.dy, got, c.want)
		}
	}
}

func TestDraggedKeepAspect(t *testing.T) {
	s := Rect{1000, 500, 640, 480}
	got := Dragged(s, BottomRight, 160, 10, true, 2560, 1440) // x led: 800 wide
	if got != (Rect{1000, 500, 800, 600}) {
		t.Errorf("corner: got %+v", got)
	}
	got = Dragged(s, Left, -64, 0, true, 2560, 1440) // 704 wide, centred vertically
	if got.W != 704 || got.H != 528 || got.X+got.W != 1640 || got.Y != 476 {
		t.Errorf("side: got %+v", got)
	}
}
