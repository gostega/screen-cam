//go:build e2e

// e2e drives a real screen-cam on a virtual X display (Xvfb) and checks what
// reaches the camera device. It never touches a real desktop: the "screen"
// is a grid of coloured tiles this program draws, so every captured pixel
// says which tile — which screen position — the camera was looking at.
//
// Run by scripts/e2e.sh inside a throwaway container; see that file.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

const (
	W, H   = 1280, 720 // camera picture
	tile   = 160
	device = "/dev/video10"
)

var (
	c      *xgb.Conn
	root   xproto.Window
	failed int
	states = make(chan string, 100)
	logs   []string
)

func check(name string, ok bool, detail string) {
	mark := "PASS"
	if !ok {
		mark, failed = "FAIL", failed+1
	}
	fmt.Printf("%s  %-46s %s\n", mark, name, detail)
}

// tileColour is the RGB of the tile under screen point (x, y). Red stays low
// so no tile can be mistaken for the red outline.
func tileColour(x, y int) (r, g, b int) { return 0x30, (x / tile) * 15, (y / tile) * 25 }

func yuv(r, g, b int) (int, int, int) {
	return (66*r+129*g+25*b+128)>>8 + 16, (-38*r-74*g+112*b+128)>>8 + 128, (112*r-94*g-18*b+128)>>8 + 128
}

func drawTiles(sw, sh int) {
	win, _ := xproto.NewWindowId(c)
	s := xproto.Setup(c).DefaultScreen(c)
	xproto.CreateWindow(c, s.RootDepth, win, root, 0, 0, uint16(sw), uint16(sh), 0,
		xproto.WindowClassInputOutput, s.RootVisual, xproto.CwOverrideRedirect, []uint32{1})
	xproto.MapWindow(c, win)
	gc, _ := xproto.NewGcontextId(c)
	xproto.CreateGC(c, gc, xproto.Drawable(win), 0, nil)
	for ty := 0; ty < sh; ty += tile {
		for tx := 0; tx < sw; tx += tile {
			r, g, b := tileColour(tx, ty)
			xproto.ChangeGC(c, gc, xproto.GcForeground, []uint32{uint32(r<<16 | g<<8 | b)})
			xproto.PolyFillRectangle(c, xproto.Drawable(win), gc, []xproto.Rectangle{{X: int16(tx), Y: int16(ty), Width: tile, Height: tile}})
		}
	}
	xproto.GetInputFocus(c).Reply() // flush
}

// readFrame returns one I420 picture from the device, skipping a few so the
// result reflects the latest state.
func readFrame(f *os.File, skip int) []byte {
	buf := make([]byte, W*H*3/2)
	for i := 0; i <= skip; i++ {
		n := 0
		for n < len(buf) {
			m, err := f.Read(buf[n:])
			if err != nil {
				fmt.Println("read:", err)
				os.Exit(2)
			}
			n += m
		}
	}
	return buf
}

func pix(fr []byte, x, y int) (int, int, int) {
	return int(fr[y*W+x]), int(fr[W*H+(y/2)*(W/2)+x/2]), int(fr[W*H*5/4+(y/2)*(W/2)+x/2])
}

func near(a, b [3]int, tol int) bool {
	for i := range a {
		if d := a[i] - b[i]; d > tol || d < -tol {
			return false
		}
	}
	return true
}

// redPixels counts outline-red pixels in a camera picture (high V, mid Y).
func redPixels(fr []byte) int {
	n := 0
	for y := 0; y < H; y += 2 {
		for x := 0; x < W; x += 2 {
			Y, _, V := pix(fr, x, y)
			if V > 200 && Y > 60 && Y < 140 {
				n++
			}
		}
	}
	return n
}

func motion(x, y int) {
	xtest.FakeInput(c, xproto.MotionNotify, 0, 0, root, int16(x), int16(y), 0)
	xproto.GetInputFocus(c).Reply()
}

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
	panic(fmt.Sprintf("no keycode for keysym %#x", sym))
}

func press(ev byte, detail byte) { xtest.FakeInput(c, ev, detail, 0, root, 0, 0, 0) }

// chord presses Ctrl+Alt+key.
func chord(sym uint32) {
	ctrl, alt, k := keycode(0xffe3), keycode(0xffe9), keycode(sym)
	press(xproto.KeyPress, ctrl)
	press(xproto.KeyPress, alt)
	press(xproto.KeyPress, k)
	press(xproto.KeyRelease, k)
	press(xproto.KeyRelease, alt)
	press(xproto.KeyRelease, ctrl)
	xproto.GetInputFocus(c).Reply()
}

func scrollWithChord(up bool) {
	ctrl, alt := keycode(0xffe3), keycode(0xffe9)
	btn := byte(5)
	if up {
		btn = 4
	}
	press(xproto.KeyPress, ctrl)
	press(xproto.KeyPress, alt)
	press(xproto.ButtonPress, btn)
	press(xproto.ButtonRelease, btn)
	press(xproto.KeyRelease, alt)
	press(xproto.KeyRelease, ctrl)
	xproto.GetInputFocus(c).Reply()
}

func drag(fromX, fromY, dx, dy int) {
	motion(fromX, fromY)
	press(xproto.ButtonPress, 1)
	xproto.GetInputFocus(c).Reply()
	for i := 1; i <= 5; i++ {
		motion(fromX+dx*i/5, fromY+dy*i/5)
		time.Sleep(30 * time.Millisecond)
	}
	press(xproto.ButtonRelease, 1)
	xproto.GetInputFocus(c).Reply()
}

// findWindow returns the first viewable window under w titled name.
func findWindow(w xproto.Window, name string) xproto.Window {
	tree, err := xproto.QueryTree(c, w).Reply()
	if err != nil {
		return 0
	}
	for _, ch := range tree.Children {
		for _, prop := range []string{"_NET_WM_NAME", "WM_NAME"} {
			atom, _ := xproto.InternAtom(c, true, uint16(len(prop)), prop).Reply()
			if atom == nil || atom.Atom == 0 {
				continue
			}
			p, err := xproto.GetProperty(c, false, ch, atom.Atom, xproto.GetPropertyTypeAny, 0, 256).Reply()
			if err == nil && string(p.Value) == name {
				if a, _ := xproto.GetWindowAttributes(c, ch).Reply(); a != nil && a.MapState == xproto.MapStateViewable {
					return ch
				}
			}
		}
		if found := findWindow(ch, name); found != 0 {
			return found
		}
	}
	return 0
}

type state struct {
	live, pinned       bool
	w, h, x, y, rw, rh int
	preset             int
	raw                string
}

var stateRE = regexp.MustCompile(`live=(\w+) pinned=(\w+) size=(\d+)x(\d+) rect=(-?\d+),(-?\d+),(\d+)x(\d+) preset=(-?\d+)`)

func parse(line string) state {
	m := stateRE.FindStringSubmatch(line)
	if m == nil {
		return state{raw: line}
	}
	n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
	return state{m[1] == "true", m[2] == "true", n(3), n(4), n(5), n(6), n(7), n(8), n(9), line}
}

// waitState returns the first state line, after the action, that satisfies ok.
func waitState(ok func(state) bool) (state, bool) {
	deadline := time.After(3 * time.Second)
	var last state
	for {
		select {
		case l := <-states:
			last = parse(l)
			if ok(last) {
				return last, true
			}
		case <-deadline:
			return last, false
		}
	}
}

func drain() {
	for {
		select {
		case <-states:
		default:
			return
		}
	}
}

func main() {
	var err error
	if c, err = xgb.NewConn(); err != nil {
		fmt.Println("no X display:", err)
		os.Exit(2)
	}
	if err := xtest.Init(c); err != nil {
		fmt.Println("XTEST:", err)
		os.Exit(2)
	}
	s := xproto.Setup(c).DefaultScreen(c)
	root = s.Root
	sw, sh := int(s.WidthInPixels), int(s.HeightInPixels)
	drawTiles(sw, sh)

	// Another program holds Ctrl+Alt+S before screen-cam starts, so the app
	// must report it and carry on (TASK-5 acceptance).
	if err := xproto.GrabKeyChecked(c, true, root, 0x4|0x8, xproto.Keycode(keycode('s')),
		xproto.GrabModeAsync, xproto.GrabModeAsync).Check(); err != nil {
		fmt.Println("could not pre-grab Ctrl+Alt+S:", err)
	}

	cfgDir, _ := os.MkdirTemp("", "sc-cfg")
	app := exec.Command(os.Args[1], "--no-tray")
	app.Env = append(os.Environ(), "SCREEN_CAM_DEBUG=1", "XDG_CONFIG_HOME="+cfgDir, "XDG_RUNTIME_DIR="+cfgDir)
	stderr, _ := app.StderrPipe()
	if err := app.Start(); err != nil {
		fmt.Println("start:", err)
		os.Exit(2)
	}
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			logs = append(logs, sc.Text())
			if strings.HasPrefix(sc.Text(), "state ") {
				states <- sc.Text()
			}
		}
	}()
	defer app.Process.Kill()

	st, ok := waitState(func(s state) bool { return true })
	check("app starts with the camera off", ok && !st.live, st.raw)
	time.Sleep(300 * time.Millisecond)
	joined := strings.Join(logs, "\n")
	check("held shortcut reported, not fatal", strings.Contains(joined, "Ctrl+Alt+S is taken by another program"), "")

	cam, err := os.Open(device)
	if err != nil {
		fmt.Println("open device:", err)
		os.Exit(2)
	}
	fr := readFrame(cam, 2)
	y, _, _ := pix(fr, 5, 5)
	wy, _, _ := yuv(0x22, 0x2a, 0x2e)
	check("off card on the device, not the screen", y >= wy-3 && y <= wy+3, fmt.Sprintf("corner Y=%d want≈%d", y, wy))

	sample := func(name string, px, py int) {
		fr := readFrame(cam, 4)
		ry, ru, rv := pix(fr, W/2, H/2)
		ty, tu, tv := yuv(tileColour(px, py))
		check(name, near([3]int{ry, ru, rv}, [3]int{ty, tu, tv}, 4),
			fmt.Sprintf("centre YUV %d,%d,%d; tile at %d,%d is %d,%d,%d", ry, ru, rv, px, py, ty, tu, tv))
	}

	// Start, following the pointer.
	motion(1000, 600)
	drain()
	chord('c')
	st, ok = waitState(func(s state) bool { return s.live })
	check("Ctrl+Alt+C starts the camera", ok, st.raw)
	sample("frame shows the tile under the pointer", 1000, 600)
	fr = readFrame(cam, 2)
	check("outline never in the camera picture", redPixels(fr) == 0, fmt.Sprintf("%d red pixels", redPixels(fr)))

	// The outline is on screen, just outside the viewport (640x480 around 1000,600).
	img, _ := xproto.GetImage(c, xproto.ImageFormatZPixmap, xproto.Drawable(root), 1000-320-2, 600, 1, 1, 0xffffffff).Reply()
	inside, _ := xproto.GetImage(c, xproto.ImageFormatZPixmap, xproto.Drawable(root), 1000-320+2, 600, 1, 1, 0xffffffff).Reply()
	isRed := func(d []byte) bool { return len(d) >= 3 && d[2] == 0xff && d[1] == 0x4a && d[0] == 0x3a }
	check("outline drawn on screen outside the viewport", isRed(img.Data) && !isRed(inside.Data),
		fmt.Sprintf("2px outside=%x 2px inside=%x", img.Data[:3], inside.Data[:3]))

	motion(1800, 900)
	sample("viewport follows the pointer", 1800, 900)

	drain()
	chord('=')
	st, ok = waitState(func(s state) bool { return s.w == 704 && s.h == 528 })
	check("Ctrl+Alt+= grows 10%", ok, st.raw)
	drain()
	chord('-')
	st, ok = waitState(func(s state) bool { return s.w == 640 && s.h == 480 })
	check("Ctrl+Alt+- shrinks back", ok, st.raw)
	drain()
	scrollWithChord(true)
	st, ok = waitState(func(s state) bool { return s.w == 704 })
	check("Ctrl+Alt+scroll resizes", ok, st.raw)
	drain()
	chord('1')
	st, ok = waitState(func(s state) bool { return s.preset == 0 && s.w == 360 && s.h == 640 })
	check("Ctrl+Alt+1 selects Phone screen 360x640", ok, st.raw)
	drain()
	chord('2')
	st, ok = waitState(func(s state) bool { return s.preset == 1 && s.w == 640 && s.h == 480 })
	check("Ctrl+Alt+2 selects Single label 640x480", ok, st.raw)
	drain()
	chord('a')
	st, ok = waitState(func(s state) bool { return s.w == 640 && s.h == 360 })
	check("Ctrl+Alt+A cycles 4:3 -> 16:9", ok, st.raw)

	// Frames keep arriving through every resize: count them over a second.
	start, n := time.Now(), 0
	for time.Since(start) < time.Second {
		readFrame(cam, 0)
		n++
	}
	check("frames keep flowing after resizes", n >= 20, fmt.Sprintf("%d fps", n))

	// Pin, then move the pointer away: the camera stays where it was.
	motion(1200, 700)
	time.Sleep(200 * time.Millisecond)
	drain()
	chord('p')
	st, ok = waitState(func(s state) bool { return s.pinned })
	check("Ctrl+Alt+P pins", ok, st.raw)
	motion(200, 200)
	sample("pinned frame ignores the pointer", 1200, 700)

	// Drag the bottom-right handle by +100,+60.
	r := st
	drain()
	drag(r.x+r.rw+5, r.y+r.rh+5, 100, 60)
	st, ok = waitState(func(s state) bool { return s.rw == r.rw+100 && s.rh == r.rh+60 })
	check("bottom-right handle resizes", ok, fmt.Sprintf("from %dx%d: %s", r.rw, r.rh, st.raw))

	// Drag the tag (the move grip, above the frame's left side) by -200,-100.
	r = st
	drain()
	drag(r.x+11+12, r.y-12, -200, -100)
	st, ok = waitState(func(s state) bool { return s.x == r.x-200 && s.y == r.y-100 && s.rw == r.rw })
	check("dragging the tag moves the frame", ok, fmt.Sprintf("from %d,%d: %s", r.x, r.y, st.raw))
	sample("camera follows the moved frame", st.x+st.rw/2, st.y+st.rh/2)

	// Edit the shortcuts file: toggle moves to Ctrl+Alt+T, reloaded live.
	cfgPath := filepath.Join(cfgDir, "screen-cam", "config.json")
	b, _ := os.ReadFile(cfgPath)
	os.WriteFile(cfgPath, []byte(strings.Replace(string(b), `"toggle": "Ctrl+Alt+C"`, `"toggle": "Ctrl+Alt+T"`, 1)), 0o644)
	time.Sleep(1500 * time.Millisecond)
	drain()
	chord('t')
	st, ok = waitState(func(s state) bool { return !s.live })
	check("edited shortcut works without a restart", ok, st.raw)
	fr = readFrame(cam, 3)
	y, _, _ = pix(fr, 5, 5)
	check("stopping shows the off card again", y >= wy-3 && y <= wy+3, fmt.Sprintf("corner Y=%d", y))

	// A broken presets file is reported and the app keeps running.
	os.WriteFile(filepath.Join(cfgDir, "screen-cam", "presets.json"), []byte(`[{"name": "x", `), 0o644)
	time.Sleep(1500 * time.Millisecond)
	check("broken presets.json reported, app alive", strings.Contains(strings.Join(logs, "\n"), "presets.json not loaded") && app.ProcessState == nil, "")
	drain()
	chord('2')
	st, ok = waitState(func(s state) bool { return s.live && s.w == 640 && s.h == 480 })
	check("old presets still work after the bad edit", ok, st.raw)

	// Save a preset through the zenity prompt (figure save-preset). Release
	// the Ctrl+Alt+S grab taken at the start and make the app re-bind by
	// touching its config, then accept the pre-filled name with Return.
	xproto.UngrabKey(c, xproto.Keycode(keycode('s')), root, 0x4|0x8)
	now := time.Now()
	os.Chtimes(cfgPath, now, now)
	time.Sleep(1500 * time.Millisecond)
	drain()
	chord('s')
	time.Sleep(2500 * time.Millisecond) // zenity (GTK) is slow to start
	// No window manager on Xvfb, so nothing gives the dialog focus: find it
	// by title and focus it, as a window manager would.
	dlg := findWindow(root, "Save Screen Cam preset")
	check("name prompt appears", dlg != 0, "")
	if dlg != 0 {
		xproto.SetInputFocus(c, xproto.InputFocusPointerRoot, dlg, xproto.TimeCurrentTime)
		xproto.GetInputFocus(c).Reply()
		time.Sleep(300 * time.Millisecond)
	}
	ret := keycode(0xff0d)
	press(xproto.KeyPress, ret)
	press(xproto.KeyRelease, ret)
	xproto.GetInputFocus(c).Reply()
	time.Sleep(1500 * time.Millisecond)
	saved, _ := os.ReadFile(filepath.Join(cfgDir, "screen-cam", "presets.json"))
	check("Ctrl+Alt+S saves the size via the name prompt", strings.Contains(string(saved), `"name": "640×480"`),
		fmt.Sprintf("presets.json has %d bytes", len(saved)))
	check("saved preset is confirmed with its shortcut", strings.Contains(strings.Join(logs, "\n"), "640×480 · Ctrl+Alt+4"), "")

	// Throughput at a large viewport (informational for TASK-2).
	chord('=')
	chord('=')
	chord('=')
	time.Sleep(300 * time.Millisecond)
	start, n = time.Now(), 0
	for time.Since(start) < 2*time.Second {
		readFrame(cam, 0)
		n++
	}
	fmt.Printf("INFO  fps at about 850x640 viewport: %.1f\n", float64(n)/2)

	if failed > 0 {
		fmt.Println("\napp log (state lines omitted):")
		for _, l := range logs {
			if !strings.HasPrefix(l, "state ") {
				fmt.Println("  ", l)
			}
		}
		fmt.Printf("\n%d check(s) failed\n", failed)
		os.Exit(1)
	}
	fmt.Println("\nall checks passed")
}
