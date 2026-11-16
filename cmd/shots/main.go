//go:build shots

// shots takes the report screenshots of a real screen-cam on a real
// display, without capturing the desktop: it first covers the whole work
// area (everything but the panel) with a test screen it draws, and it only
// ever grabs regions inside that test screen, plus the far-right end of the
// panel where the tray icon and its menu are. The window list is never
// captured.
//
//	go run -tags shots ./cmd/shots ./screen-cam OUTDIR
package main

import (
	"bufio"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"

	"github.com/gostega/screen-cam/internal/draw"
)

const panelTop = 1400 // _NET_WORKAREA height on this machine

var (
	c    *xgb.Conn
	root xproto.Window
	out  string
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// testScreen covers 0..W x 0..panelTop with a grey desk and six label cards.
func testScreen(sw int) xproto.Window {
	s := xproto.Setup(c).DefaultScreen(c)
	win, _ := xproto.NewWindowId(c)
	xproto.CreateWindow(c, s.RootDepth, win, root, 0, 0, uint16(sw), panelTop, 0,
		xproto.WindowClassInputOutput, s.RootVisual, xproto.CwBackPixel|xproto.CwOverrideRedirect, []uint32{0x3a4650, 1})
	xproto.MapWindow(c, win)
	xproto.GetInputFocus(c).Reply()
	time.Sleep(200 * time.Millisecond)
	gc, _ := xproto.NewGcontextId(c)
	xproto.CreateGC(c, gc, xproto.Drawable(win), 0, nil)
	put := func(img *image.RGBA, x, y int) {
		b := img.Bounds()
		for row := 0; row < b.Dy(); row += 64 { // PutImage request size limit
			h := min(64, b.Dy()-row)
			sub := img.SubImage(image.Rect(0, row, b.Dx(), row+h)).(*image.RGBA)
			cp := image.NewRGBA(image.Rect(0, 0, b.Dx(), h))
			for yy := 0; yy < h; yy++ {
				copy(cp.Pix[yy*cp.Stride:], sub.Pix[yy*sub.Stride:yy*sub.Stride+b.Dx()*4])
			}
			xproto.PutImage(c, xproto.ImageFormatZPixmap, xproto.Drawable(win), gc, uint16(b.Dx()), uint16(h),
				int16(x), int16(y+row), 0, s.RootDepth, draw.BGRA(cp))
		}
	}
	put(draw.Text("screen-cam test screen - the real desktop is underneath", 2, 10,
		image.White.C, image.NewUniform(colorRGB(0x3a, 0x46, 0x50)).C), 40, 30)
	// Six label cards in two rows, like a sheet of asset-label stickers.
	for i := 0; i < 6; i++ {
		x, y := 260+(i%3)*360, 320+(i/3)*420
		card := image.NewRGBA(image.Rect(0, 0, 300, 340))
		for p := 0; p < len(card.Pix); p += 4 {
			copy(card.Pix[p:p+4], []byte{0xfa, 0xfa, 0xf6, 0xff})
		}
		qrish(card, 40, 30, 220, uint32(i+1))
		put(card, x, y)
		put(draw.Text(fmt.Sprintf("UB-030%d", i+1), 3, 4, colorRGB(0x22, 0x22, 0x22), colorRGB(0xfa, 0xfa, 0xf6)), x+70, y+270)
	}
	xproto.GetInputFocus(c).Reply()
	return win
}

func colorRGB(r, g, b byte) colorT { return colorT{r, g, b} }

type colorT struct{ r, g, b byte }

func (c colorT) RGBA() (uint32, uint32, uint32, uint32) {
	return uint32(c.r) * 0x101, uint32(c.g) * 0x101, uint32(c.b) * 0x101, 0xffff
}

// qrish draws a QR-looking (not scannable) 21x21 module pattern.
func qrish(img *image.RGBA, x0, y0, size int, seed uint32) {
	m := size / 21
	s := seed*2654435761 + 7
	for my := 0; my < 21; my++ {
		for mx := 0; mx < 21; mx++ {
			s = s*1103515245 + 12345
			on := (s>>16)&1 == 1
			for _, f := range [][2]int{{0, 0}, {14, 0}, {0, 14}} {
				dx, dy := mx-f[0], my-f[1]
				if dx >= 0 && dy >= 0 && dx < 7 && dy < 7 {
					on = dx == 0 || dy == 0 || dx == 6 || dy == 6 || (dx >= 2 && dx <= 4 && dy >= 2 && dy <= 4)
				}
			}
			if !on {
				continue
			}
			for yy := 0; yy < m; yy++ {
				for xx := 0; xx < m; xx++ {
					o := img.PixOffset(x0+mx*m+xx, y0+my*m+yy)
					copy(img.Pix[o:o+4], []byte{0x11, 0x11, 0x11, 0xff})
				}
			}
		}
	}
}

// findWindow returns the first window under w whose title is name.
func findWindow(w xproto.Window, name string) xproto.Window {
	tree, err := xproto.QueryTree(c, w).Reply()
	if err != nil {
		return 0
	}
	for _, ch := range tree.Children {
		for _, prop := range []string{"_NET_WM_NAME", "WM_NAME"} {
			a, _ := xproto.InternAtom(c, true, uint16(len(prop)), prop).Reply()
			if a == nil || a.Atom == 0 {
				continue
			}
			p, err := xproto.GetProperty(c, false, ch, a.Atom, xproto.GetPropertyTypeAny, 0, 256).Reply()
			if err == nil && string(p.Value) == name {
				return ch
			}
		}
		if f := findWindow(ch, name); f != 0 {
			return f
		}
	}
	return 0
}

func grab(x, y, w, h int) *image.RGBA {
	r, err := xproto.GetImage(c, xproto.ImageFormatZPixmap, xproto.Drawable(root), int16(x), int16(y), uint16(w), uint16(h), 0xffffffff).Reply()
	must(err)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = r.Data[i*4+2], r.Data[i*4+1], r.Data[i*4], 0xff
	}
	return img
}

func save(name string, img image.Image) {
	f, err := os.Create(filepath.Join(out, name))
	must(err)
	png.Encode(f, img)
	f.Close()
	fmt.Println("wrote", name, img.Bounds().Dx(), "x", img.Bounds().Dy())
}

func sync() { xproto.GetInputFocus(c).Reply() }

func motion(x, y int) { xtest.FakeInput(c, xproto.MotionNotify, 0, 0, root, int16(x), int16(y), 0); sync() }

func keycode(sym uint32) byte {
	setup := xproto.Setup(c)
	n := byte(setup.MaxKeycode - setup.MinKeycode + 1)
	m, _ := xproto.GetKeyboardMapping(c, setup.MinKeycode, n).Reply()
	per := int(m.KeysymsPerKeycode)
	for i := 0; i < int(n); i++ {
		if uint32(m.Keysyms[i*per]) == sym {
			return byte(setup.MinKeycode) + byte(i)
		}
	}
	panic(fmt.Sprintf("no keycode for %#x", sym))
}

func tap(syms ...uint32) {
	for _, s := range syms {
		xtest.FakeInput(c, xproto.KeyPress, keycode(s), 0, root, 0, 0, 0)
	}
	for i := len(syms) - 1; i >= 0; i-- {
		xtest.FakeInput(c, xproto.KeyRelease, keycode(syms[i]), 0, root, 0, 0, 0)
	}
	sync()
}

func chord(sym uint32) { tap(0xffe3, 0xffe9, sym) } // Ctrl+Alt+sym

func click(btn byte, x, y int) {
	motion(x, y)
	xtest.FakeInput(c, xproto.ButtonPress, btn, 0, root, 0, 0, 0)
	xtest.FakeInput(c, xproto.ButtonRelease, btn, 0, root, 0, 0, 0)
	sync()
}

// diffBox is the bounding box of pixels that differ between a and b.
func diffBox(a, b *image.RGBA) image.Rectangle {
	box := image.Rectangle{}
	for y := 0; y < a.Bounds().Dy(); y++ {
		for x := 0; x < a.Bounds().Dx(); x++ {
			o := a.PixOffset(x, y)
			if a.Pix[o] != b.Pix[o] || a.Pix[o+1] != b.Pix[o+1] || a.Pix[o+2] != b.Pix[o+2] {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

func main() {
	exe, outDir := os.Args[1], os.Args[2]
	out = outDir
	os.MkdirAll(out, 0o755)
	var err error
	c, err = xgb.NewConn()
	must(err)
	must(xtest.Init(c))
	s := xproto.Setup(c).DefaultScreen(c)
	root = s.Root
	sw := int(s.WidthInPixels)
	cover := testScreen(sw)
	defer xproto.DestroyWindow(c, cover)

	// The tray strip: the right 360px of the panel only (applets, never the window list).
	strip := func() *image.RGBA { return grab(sw-360, panelTop, 360, 40) }
	before := strip()

	cfg, _ := os.MkdirTemp("", "sc-shots")
	app := exec.Command(exe)
	app.Env = append(os.Environ(), "SCREEN_CAM_DEBUG=1", "XDG_CONFIG_HOME="+cfg)
	stderr, _ := app.StderrPipe()
	must(app.Start())
	defer app.Process.Kill()
	lines := make(chan string, 100)
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	time.Sleep(2500 * time.Millisecond)
	idle := strip()
	_ = before
	save("tray-strip-idle.png", idle)

	// Live, following the mouse over label UB-0302.
	motion(260+360+150, 320+170)
	chord('c')
	time.Sleep(800 * time.Millisecond)
	live := strip()
	save("tray-strip-live.png", live)
	// Only our icon changes between idle and live (it gains the red dot), so
	// the difference locates it even though other applets shift around it.
	icon := diffBox(idle, live)
	fmt.Println("tray icon's live dot at", icon)
	chord('2') // re-select the preset so the size tag shows
	time.Sleep(300 * time.Millisecond)
	save("screen-live.png", grab(380, 120, 800, 700))

	// The tray menu: right-click the icon; grab the panel's right end and the menu above it.
	cx, cy := sw-360+(icon.Min.X+icon.Max.X)/2, panelTop+20
	if icon.Empty() {
		fmt.Println("could not find the tray icon; skipping the menu shot")
	} else {
		click(3, cx, cy)
		time.Sleep(900 * time.Millisecond)
		save("tray-menu.png", grab(sw-760, panelTop-760, 760, 800))
		tap(0xff1b) // Escape closes the menu
		time.Sleep(300 * time.Millisecond)
	}
	// Back over UB-0302 before pinning: the frame followed the pointer to the tray.
	motion(260+360+150, 320+170)
	time.Sleep(400 * time.Millisecond)

	// Pinned, with handles.
	chord('p')
	time.Sleep(300 * time.Millisecond)
	motion(1900, 300) // pointer away: the frame stays
	time.Sleep(300 * time.Millisecond)
	save("screen-pinned.png", grab(380, 120, 800, 700))

	// Save a preset: the zenity prompt, then the notification.
	// The dialog is a managed window, so it opens beneath the override-redirect
	// test screen; grab the dialog's own window rather than the screen. The
	// notification is drawn by Cinnamon itself, so record the D-Bus call that
	// asked for it instead of photographing the screen.
	mon := exec.Command("dbus-monitor", "--session", "interface='org.freedesktop.Notifications',member='Notify'")
	monOut, _ := mon.StdoutPipe()
	mon.Start()
	chord('s')
	time.Sleep(2500 * time.Millisecond)
	if dlg := findWindow(root, "Save Screen Cam preset"); dlg != 0 {
		g, _ := xproto.GetGeometry(c, xproto.Drawable(dlg)).Reply()
		r, err := xproto.GetImage(c, xproto.ImageFormatZPixmap, xproto.Drawable(dlg), 0, 0, g.Width, g.Height, 0xffffffff).Reply()
		if err == nil {
			w, h := int(g.Width), int(g.Height)
			img := image.NewRGBA(image.Rect(0, 0, w, h))
			for i := 0; i < w*h && i*4+3 < len(r.Data); i++ {
				img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = r.Data[i*4+2], r.Data[i*4+1], r.Data[i*4], 0xff
			}
			save("save-preset-dialog.png", img)
		}
	} else {
		fmt.Println("dialog not found")
	}
	tap(0xff0d) // Return accepts the pre-filled name
	time.Sleep(1500 * time.Millisecond)
	mon.Process.Kill()
	b := make([]byte, 8192)
	n, _ := monOut.Read(b)
	os.WriteFile(filepath.Join(out, "save-preset-notification.txt"), b[:n], 0o644)
	fmt.Println("notification call:", strings.Join(strings.Fields(string(b[:n])), " "))

	chord('c') // stop
	time.Sleep(500 * time.Millisecond)
	app.Process.Signal(os.Interrupt)
	time.Sleep(300 * time.Millisecond)
	var log []string
	for {
		select {
		case l := <-lines:
			log = append(log, l)
			continue
		default:
		}
		break
	}
	fmt.Println(strings.Join(log, "\n"))
}
