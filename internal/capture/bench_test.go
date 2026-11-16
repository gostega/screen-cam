package capture

import (
	"os"
	"testing"

	"github.com/jezek/xgb/xproto"
)

// BenchmarkGrab1200x900 measures GetImage on the running X server against an
// off-screen pixmap: the same request path as a screen grab, no desktop pixels.
func BenchmarkGrab1200x900(b *testing.B) {
	if os.Getenv("DISPLAY") == "" {
		b.Skip("no X display")
	}
	x, err := Connect()
	if err != nil {
		b.Skip(err)
	}
	defer x.Close()
	c := x.Conn()
	s := xproto.Setup(c).DefaultScreen(c)
	pm, _ := xproto.NewPixmapId(c)
	xproto.CreatePixmap(c, s.RootDepth, pm, xproto.Drawable(s.Root), 1200, 900)
	defer xproto.FreePixmap(c, pm)
	for b.Loop() {
		if _, err := x.Grab(xproto.Drawable(pm), 0, 0, 1200, 900); err != nil {
			b.Fatal(err)
		}
	}
}
