package capture

import (
	"os"
	"testing"

	"github.com/jezek/xgb/xproto"
)

// Grab is tested against an off-screen pixmap the test paints itself, so the
// test never reads what is on the real screen.
func TestGrabReadsBGRA(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	x, err := Connect()
	if err != nil {
		t.Skip(err)
	}
	defer x.Close()
	c := x.Conn()
	s := xproto.Setup(c).DefaultScreen(c)

	pm, _ := xproto.NewPixmapId(c)
	xproto.CreatePixmap(c, s.RootDepth, pm, xproto.Drawable(s.Root), 64, 64)
	defer xproto.FreePixmap(c, pm)
	gc, _ := xproto.NewGcontextId(c)
	// Pure red on a 24-bit TrueColor visual is 0xff0000.
	xproto.CreateGC(c, gc, xproto.Drawable(pm), xproto.GcForeground, []uint32{0xff0000})
	defer xproto.FreeGC(c, gc)
	xproto.PolyFillRectangle(c, xproto.Drawable(pm), gc, []xproto.Rectangle{{X: 0, Y: 0, Width: 64, Height: 64}})

	px, err := x.Grab(xproto.Drawable(pm), 10, 10, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	if b, g, r := px[0], px[1], px[2]; r != 255 || g != 0 || b != 0 {
		t.Fatalf("pixel = r%d g%d b%d, want pure red", r, g, b)
	}
}
