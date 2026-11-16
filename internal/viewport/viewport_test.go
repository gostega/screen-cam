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
