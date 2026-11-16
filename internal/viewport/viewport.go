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
