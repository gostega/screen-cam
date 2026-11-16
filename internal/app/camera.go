package app

import (
	"time"

	"github.com/gostega/screen-cam/internal/capture"
	"github.com/gostega/screen-cam/internal/draw"
	"github.com/gostega/screen-cam/internal/frame"
	"github.com/gostega/screen-cam/internal/v4l2"
	"github.com/gostega/screen-cam/internal/viewport"
)

// OutW, OutH is the camera picture size, fixed for the device's lifetime (ADR-0001).
const OutW, OutH = 1280, 720

// runCamera writes a frame to the device every tick until stop closes:
// the viewport while live, the off card while stopped (D5). It keeps the
// device open the whole time so readers never see the stream end.
func (a *App) runCamera(cam *v4l2.Device, x *capture.X, stop <-chan struct{}) {
	out := frame.NewI420(OutW, OutH)
	off := frame.NewI420(OutW, OutH)
	frame.NewScaler(OutW, OutH, off).Render(draw.BGRA(draw.OffCard(OutW, OutH, a.cfg().Shortcuts.Toggle)), OutW*4)
	var sc *frame.Scaler

	tick := time.NewTicker(time.Second / time.Duration(a.cfg().FPS))
	defer tick.Stop()
	fps := a.cfg().FPS
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
		}
		if f := a.cfg().FPS; f != fps { // config.json edited
			fps = f
			tick.Reset(time.Second / time.Duration(fps))
		}
		r, live := a.frameRect(x)
		if !live {
			if err := cam.Write(off.Buf); err != nil {
				a.cameraFailed(err)
				return
			}
			continue
		}
		if sc == nil || !sc.Matches(r.W, r.H) {
			sc = frame.NewScaler(r.W, r.H, out)
		}
		img, err := x.Grab(x.Root(), r.X, r.Y, r.W, r.H)
		if err != nil {
			continue // a transient X error; try again next frame
		}
		sc.Render(img, r.W*4)
		if err := cam.Write(out.Buf); err != nil {
			a.cameraFailed(err)
			return
		}
	}
}

// frameRect is this frame's viewport: centred on the pointer when following,
// where it was left when pinned. It also moves the outline to match.
func (a *App) frameRect(x *capture.X) (viewport.Rect, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.live {
		return viewport.Rect{}, false
	}
	if !a.pinned && !a.dragging {
		if px, py, err := x.Pointer(); err == nil {
			a.rect = viewport.CenteredOn(px, py, a.w, a.h, a.screenW, a.screenH)
		}
	}
	a.ui.Move(a.rect)
	return a.rect, true
}
