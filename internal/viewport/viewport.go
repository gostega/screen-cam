// Package viewport is the on-screen rectangle the camera looks at: where it
// sits, how big it is, and keeping it on the screen.
package viewport

// Min is the smallest side a viewport may have; below this a QR code cannot
// fit and the scaled-up picture is mostly blur.
const Min = 64

// Rect is a screen rectangle in pixels.
type Rect struct{ X, Y, W, H int }

// Clamp limits w and h to [Min, screen] so the rectangle can always fit.
func clampSize(w, h, screenW, screenH int) (int, int) {
	w = max(Min, min(w, screenW))
	h = max(Min, min(h, screenH))
	return w, h
}

// CenteredOn returns a w×h rectangle centred on (px, py), pushed back inside
// the screen when the point is near an edge, so the camera never looks at
// off-screen pixels.
func CenteredOn(px, py, w, h, screenW, screenH int) Rect {
	w, h = clampSize(w, h, screenW, screenH)
	x := min(max(px-w/2, 0), screenW-w)
	y := min(max(py-h/2, 0), screenH-h)
	return Rect{x, y, w, h}
}

// Scaled grows or shrinks r by factor around its centre, keeping its aspect
// ratio and keeping it on screen.
func Scaled(r Rect, factor float64, screenW, screenH int) Rect {
	w := int(float64(r.W)*factor + 0.5)
	h := int(float64(r.H)*factor + 0.5)
	// Keep the aspect ratio when one side hits a limit.
	if w > screenW || h > screenH {
		s := min(float64(screenW)/float64(r.W), float64(screenH)/float64(r.H))
		w, h = int(float64(r.W)*s), int(float64(r.H)*s)
	}
	if w < Min || h < Min {
		s := max(float64(Min)/float64(r.W), float64(Min)/float64(r.H))
		w, h = int(float64(r.W)*s+0.5), int(float64(r.H)*s+0.5)
	}
	return CenteredOn(r.X+r.W/2, r.Y+r.H/2, w, h, screenW, screenH)
}

// Aspects is the order Ctrl+Alt+A cycles through, as width:height.
var Aspects = [][2]int{{4, 3}, {16, 9}, {1, 1}, {9, 16}}

// AspectIndex is the entry in Aspects closest to r's shape.
func AspectIndex(r Rect) int {
	best, bestDiff := 0, 1e9
	shape := float64(r.W) / float64(r.H)
	for i, a := range Aspects {
		d := shape/(float64(a[0])/float64(a[1])) - 1
		if d < 0 {
			d = -d
		}
		if d < bestDiff {
			best, bestDiff = i, d
		}
	}
	return best
}

// NextAspect switches r to the next shape in Aspects, keeping its width and
// centre (the height gives way first when the screen is too short).
func NextAspect(r Rect, screenW, screenH int) Rect {
	a := Aspects[(AspectIndex(r)+1)%len(Aspects)]
	w, h := r.W, r.W*a[1]/a[0]
	if h > screenH { // too tall for the screen: shrink both, keep the shape
		h = screenH
		w = h * a[0] / a[1]
	}
	if h < Min {
		h = Min
		w = h * a[0] / a[1]
	}
	return CenteredOn(r.X+r.W/2, r.Y+r.H/2, w, h, screenW, screenH)
}

// Moved shifts r by (dx, dy), stopping at the screen edges.
func Moved(r Rect, dx, dy, screenW, screenH int) Rect {
	r.X = min(max(r.X+dx, 0), screenW-r.W)
	r.Y = min(max(r.Y+dy, 0), screenH-r.H)
	return r
}

// Handle is one of the eight drag handles on a pinned frame.
type Handle int

const (
	TopLeft Handle = iota
	Top
	TopRight
	Right
	BottomRight
	Bottom
	BottomLeft
	Left
)

// Handles lists all eight, in drawing order.
var Handles = []Handle{TopLeft, Top, TopRight, Right, BottomRight, Bottom, BottomLeft, Left}

// edges reports which sides a handle moves: -1 the left/top side, +1 the
// right/bottom side, 0 neither.
func (h Handle) edges() (sx, sy int) {
	switch h {
	case TopLeft:
		return -1, -1
	case Top:
		return 0, -1
	case TopRight:
		return 1, -1
	case Right:
		return 1, 0
	case BottomRight:
		return 1, 1
	case Bottom:
		return 0, 1
	case BottomLeft:
		return -1, 1
	}
	return -1, 0 // Left
}

// Dragged is the frame after dragging handle hd by (dx, dy) from start. The
// opposite side stays put. keepAspect (Shift held) keeps start's shape, led
// by whichever axis moved further. The result stays on screen and at least
// Min on each side.
func Dragged(start Rect, hd Handle, dx, dy int, keepAspect bool, screenW, screenH int) Rect {
	sx, sy := hd.edges()
	left, top, right, bottom := start.X, start.Y, start.X+start.W, start.Y+start.H
	if sx < 0 {
		left = min(max(left+dx, 0), right-Min)
	} else if sx > 0 {
		right = max(min(right+dx, screenW), left+Min)
	}
	if sy < 0 {
		top = min(max(top+dy, 0), bottom-Min)
	} else if sy > 0 {
		bottom = max(min(bottom+dy, screenH), top+Min)
	}
	r := Rect{left, top, right - left, bottom - top}
	if !keepAspect {
		return r
	}
	// Scale the start shape by the larger relative change, then re-anchor on
	// the side(s) that did not move.
	f := 1.0
	fx, fy := float64(r.W)/float64(start.W), float64(r.H)/float64(start.H)
	switch {
	case sx == 0:
		f = fy
	case sy == 0:
		f = fx
	case abs(fx-1) >= abs(fy-1):
		f = fx
	default:
		f = fy
	}
	w, h := int(float64(start.W)*f+0.5), int(float64(start.H)*f+0.5)
	// Never grow past the screen.
	if w > screenW || h > screenH {
		s := min(float64(screenW)/float64(w), float64(screenH)/float64(h))
		w, h = int(float64(w)*s), int(float64(h)*s)
	}
	if w < Min || h < Min {
		s := max(float64(Min)/float64(w), float64(Min)/float64(h))
		w, h = int(float64(w)*s+0.5), int(float64(h)*s+0.5)
	}
	x, y := start.X, start.Y
	if sx < 0 {
		x = start.X + start.W - w
	} else if sx == 0 {
		x = start.X + (start.W-w)/2
	}
	if sy < 0 {
		y = start.Y + start.H - h
	} else if sy == 0 {
		y = start.Y + (start.H-h)/2
	}
	return Moved(Rect{x, y, w, h}, 0, 0, screenW, screenH)
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
