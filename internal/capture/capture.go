// Package capture reads pixels and the pointer position from the X server.
package capture

import (
	"fmt"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// X is a connection to the display.
type X struct {
	conn             *xgb.Conn
	root             xproto.Window
	ScreenW, ScreenH int
}

// Connect opens the display named by $DISPLAY.
func Connect() (*X, error) {
	c, err := xgb.NewConn()
	if err != nil {
		return nil, fmt.Errorf("connect to X (this needs an X11 session): %w", err)
	}
	s := xproto.Setup(c).DefaultScreen(c)
	return &X{conn: c, root: s.Root, ScreenW: int(s.WidthInPixels), ScreenH: int(s.HeightInPixels)}, nil
}

// Close drops the connection.
func (x *X) Close() { x.conn.Close() }

// Conn exposes the connection for callers that draw (tests, the outline).
func (x *X) Conn() *xgb.Conn { return x.conn }

// Root is the root window, the drawable for whole-screen capture.
func (x *X) Root() xproto.Drawable { return xproto.Drawable(x.root) }

// Pointer returns the pointer's position on the screen.
func (x *X) Pointer() (int, int, error) {
	p, err := xproto.QueryPointer(x.conn, x.root).Reply()
	if err != nil {
		return 0, 0, err
	}
	return int(p.RootX), int(p.RootY), nil
}

// Grab returns the w×h region at (px, py) of d as BGRA, 4 bytes a pixel, w*4
// bytes a row. The cursor is never part of it: X keeps the cursor out of
// GetImage, which is how the pointer stays out of the picture.
func (x *X) Grab(d xproto.Drawable, px, py, w, h int) ([]byte, error) {
	img, err := xproto.GetImage(x.conn, xproto.ImageFormatZPixmap, d,
		int16(px), int16(py), uint16(w), uint16(h), 0xffffffff).Reply()
	if err != nil {
		return nil, err
	}
	if img.Depth != 24 && img.Depth != 32 {
		return nil, fmt.Errorf("display depth %d not supported (want 24 or 32)", img.Depth)
	}
	if len(img.Data) < w*h*4 {
		return nil, fmt.Errorf("short image: %d bytes for %dx%d", len(img.Data), w, h)
	}
	return img.Data, nil
}
