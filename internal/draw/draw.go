// Package draw makes the small pictures screen-cam needs: the tray icons,
// the size tag's text, and the card the camera shows while it is off.
package draw

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

var (
	Red   = color.RGBA{0xff, 0x4a, 0x3a, 0xff}
	white = color.RGBA{0xff, 0xff, 0xff, 0xff}
	grey  = color.RGBA{0xc8, 0xcf, 0xcc, 0xff}
)

// TrayIcon is a 32×32 PNG camera glyph, light grey for a dark panel, with a
// red dot when live.
func TrayIcon(live bool) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	// Body: a rounded-ish rectangle outline.
	strokeRect(img, 3, 9, 21, 23, 2, grey)
	// Lens housing: a trapezoid to the right.
	for x := 21; x <= 29; x++ {
		half := 2 + (x-21)*5/8 // widens towards the front
		for y := 16 - half; y <= 16+half; y++ {
			if x >= 27 || y <= 16-half+1 || y >= 16+half-1 {
				img.Set(x, y, grey)
			}
		}
	}
	if live {
		disc(img, 25, 6, 5, Red)
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func strokeRect(img *image.RGBA, x0, y0, x1, y1, t int, c color.Color) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if x < x0+t || x > x1-t || y < y0+t || y > y1-t {
				img.Set(x, y, c)
			}
		}
	}
}

func disc(img *image.RGBA, cx, cy, r int, c color.Color) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r {
				img.Set(x, y, c)
			}
		}
	}
}

// The 7×13 font is ASCII only; these stand in for the menu's typography.
var ascii = strings.NewReplacer("×", "x", "·", "|", "…", "...", "—", "-")

// TextSize is the pixel size of s in the 7×13 font at the given scale.
func TextSize(s string, scale int) (w, h int) {
	return font.MeasureString(basicfont.Face7x13, ascii.Replace(s)).Ceil() * scale, 13 * scale
}

// Text draws s onto an RGBA picture of colour bg with padding pad (already
// scaled), each font pixel scale×scale.
func Text(s string, scale, pad int, fg, bg color.Color) *image.RGBA {
	s = ascii.Replace(s)
	tw, th := TextSize(s, 1)
	small := image.NewRGBA(image.Rect(0, 0, tw, th))
	fill(small, bg)
	d := font.Drawer{Dst: small, Src: image.NewUniform(fg), Face: basicfont.Face7x13, Dot: fixed.P(0, 11)}
	d.DrawString(s)
	out := image.NewRGBA(image.Rect(0, 0, tw*scale+2*pad, th*scale+2*pad))
	fill(out, bg)
	for y := 0; y < th*scale; y++ {
		for x := 0; x < tw*scale; x++ {
			out.Set(pad+x, pad+y, small.At(x/scale, y/scale))
		}
	}
	return out
}

// Tag is the size tag: white text on the outline's red.
func Tag(s string) *image.RGBA { return Text(s, 1, 4, white, Red) }

// OffCard is the picture the camera carries while stopped (D5): it says the
// camera is off and how to start it, so a browser never shows a stale frame
// and never sees the screen. Returned as a w×h RGBA picture.
func OffCard(w, h int, startKey string) *image.RGBA {
	bg := color.RGBA{0x22, 0x2a, 0x2e, 0xff}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill(img, bg)
	place := func(s string, scale, y int, c color.Color) {
		t := Text(s, scale, 0, c, bg)
		x := (w - t.Bounds().Dx()) / 2
		for yy := 0; yy < t.Bounds().Dy(); yy++ {
			for xx := 0; xx < t.Bounds().Dx(); xx++ {
				img.Set(x+xx, y+yy, t.At(xx, yy))
			}
		}
	}
	place("Screen Cam is off", 5, h/2-80, white)
	if startKey != "" {
		place("Press "+startKey+" to start", 3, h/2+20, grey)
	}
	return img
}

func fill(img *image.RGBA, c color.Color) {
	r, g, b, a := c.RGBA()
	px := []byte{byte(r >> 8), byte(g >> 8), byte(b >> 8), byte(a >> 8)}
	for i := 0; i < len(img.Pix); i += 4 {
		copy(img.Pix[i:i+4], px)
	}
}

// BGRA converts an RGBA picture to the byte order the X server and the
// frame scaler use.
func BGRA(img *image.RGBA) []byte {
	out := make([]byte, len(img.Pix))
	for i := 0; i < len(img.Pix); i += 4 {
		out[i], out[i+1], out[i+2], out[i+3] = img.Pix[i+2], img.Pix[i+1], img.Pix[i], img.Pix[i+3]
	}
	return out
}
