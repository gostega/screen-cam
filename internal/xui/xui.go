// Package xui is screen-cam's on-screen side of X11: global shortcuts, the
// red outline, the size tag, the hint bar and the drag handles of a pinned
// frame. It owns
// one X connection and one event loop; the camera captures on its own
// connection so drawing never waits on a frame grab.
//
// Everything it draws sits OUTSIDE the viewport rectangle, so the camera
// never sees it (figure screen-live 1-2, screen-pinned 1-2, screen-hints 3).
package xui

import (
	"fmt"
	"sync"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/shape"
	"github.com/jezek/xgb/xproto"

	"github.com/gostega/screen-cam/internal/config"
	"github.com/gostega/screen-cam/internal/draw"
	"github.com/gostega/screen-cam/internal/viewport"
)

const (
	border  = 3  // outline thickness
	handleS = 15 // handle square: red, with a white centre painted on Expose
	handleW = 7  // the white centre
)

// Lock bits a user may have on that must not stop a shortcut matching.
const (
	lockCaps = 1 << 1 // Lock
	lockNum  = 1 << 4 // Mod2
)

var ignoredLocks = []uint16{0, lockCaps, lockNum, lockCaps | lockNum}

// Events is what the UI reports to the app. Callbacks run on the event loop
// goroutine and must not block.
type Events struct {
	Shortcut func(action string) // "toggle", "bigger", "preset3", …
	Scroll   func(up bool)
	// Drag reports a frame moved or resized by hand; done is true on release.
	Drag func(r viewport.Rect, done bool)
}

// UI is the connection, the windows and the grabs.
type UI struct {
	c                *xgb.Conn
	root             xproto.Window
	screenW, screenH int
	ev               Events

	mu       sync.Mutex
	edges    [4]xproto.Window
	handles  [8]xproto.Window
	tag      xproto.Window
	tagGC    xproto.Gcontext
	whiteGC  xproto.Gcontext
	shown    bool // outline visible
	pinned   bool // handles visible, tag draggable
	tagShown bool
	tagMsg   string // set by SetTag: shown for a moment after a resize
	tagText  string // what the tag currently displays
	tagBox   [4]int // the tag's x, y, w, h while shown
	rect     viewport.Rect

	hint      xproto.Window
	hints     []draw.Hint // wanted, before fitting to the frame's width
	hintsOn   bool        // "Show shortcut hints"
	hintShown bool
	hintDrawn []draw.Hint // what the bar currently displays
	flashGen  int         // bumped by each Flash, so a newer one takes over

	keys      map[keyKey]string // grabbed key -> action
	scrollMod uint16
	drag      *dragState
}

type keyKey struct {
	code xproto.Keycode
	mods uint16
}

type dragState struct {
	handle   viewport.Handle // or -1 for a move by the tag
	startX   int16
	startY   int16
	start    viewport.Rect
	lastRect viewport.Rect
}

// New connects and creates the (hidden) windows.
func New(ev Events) (*UI, error) {
	c, err := xgb.NewConn()
	if err != nil {
		return nil, fmt.Errorf("connect to X: %w", err)
	}
	if err := shape.Init(c); err != nil {
		c.Close()
		return nil, fmt.Errorf("X shape extension: %w", err)
	}
	s := xproto.Setup(c).DefaultScreen(c)
	u := &UI{c: c, root: s.Root, screenW: int(s.WidthInPixels), screenH: int(s.HeightInPixels), ev: ev, keys: map[keyKey]string{}}

	red := uint32(0xff4a3a)
	for i := range u.edges {
		u.edges[i] = u.window(red, 0, 0)
		u.clickThrough(u.edges[i])
	}
	cursors := u.handleCursors()
	// Handles are drawn, not bordered: a compositing window manager (Cinnamon's
	// Muffin) does not paint core X window borders, which left only a 7px
	// white dot that vanished against a white label.
	for i := range u.handles {
		u.handles[i] = u.window(red, 0, 0)
		xproto.ChangeWindowAttributes(c, u.handles[i], xproto.CwEventMask|xproto.CwCursor, []uint32{
			xproto.EventMaskExposure | xproto.EventMaskButtonPress | xproto.EventMaskButtonRelease | xproto.EventMaskButtonMotion,
			uint32(cursors[i]),
		})
	}
	u.tag = u.window(red, 0, 0)
	xproto.ChangeWindowAttributes(c, u.tag, xproto.CwEventMask|xproto.CwCursor, []uint32{
		xproto.EventMaskExposure | xproto.EventMaskButtonPress | xproto.EventMaskButtonRelease | xproto.EventMaskButtonMotion,
		uint32(cursors[8]),
	})
	// The hint bar is click-through like the outline: it is a reminder and
	// must never take a click meant for the page under it.
	u.hint = u.window(0x222a2e, 0, 0)
	u.clickThrough(u.hint)
	xproto.ChangeWindowAttributes(c, u.hint, xproto.CwEventMask, []uint32{xproto.EventMaskExposure})
	u.tagGC, _ = xproto.NewGcontextId(c)
	xproto.CreateGC(c, u.tagGC, xproto.Drawable(u.tag), 0, nil)
	u.whiteGC, _ = xproto.NewGcontextId(c)
	xproto.CreateGC(c, u.whiteGC, xproto.Drawable(u.tag), xproto.GcForeground, []uint32{0xffffff})

	// Mark every overlay window as a drag-and-drop icon: compositors draw no
	// shadow or animation for that type, so the outline stays a crisp line.
	all := append(append(u.edges[:0:0], u.edges[:]...), u.handles[:]...)
	u.setType(append(all, u.tag, u.hint), "_NET_WM_WINDOW_TYPE_DND")
	return u, nil
}

// ScreenSize is the root window's size.
func (u *UI) ScreenSize() (int, int) { return u.screenW, u.screenH }

// window makes an unmapped override-redirect window: the window manager
// leaves it alone, so it has no frame, no taskbar entry and never takes focus.
func (u *UI) window(bg uint32, bw uint16, borderPx uint32) xproto.Window {
	w, _ := xproto.NewWindowId(u.c)
	mask := uint32(xproto.CwBackPixel | xproto.CwBorderPixel | xproto.CwOverrideRedirect)
	xproto.CreateWindow(u.c, xproto.WindowClassCopyFromParent, w, u.root, 0, 0, 1, 1, bw,
		xproto.WindowClassInputOutput, xproto.Setup(u.c).DefaultScreen(u.c).RootVisual, mask, []uint32{bg, borderPx, 1})
	return w
}

func (u *UI) atom(name string) xproto.Atom {
	a, err := xproto.InternAtom(u.c, false, uint16(len(name)), name).Reply()
	if err != nil {
		return 0
	}
	return a.Atom
}

func (u *UI) setType(ws []xproto.Window, typ string) {
	prop, val := u.atom("_NET_WM_WINDOW_TYPE"), u.atom(typ)
	if prop == 0 || val == 0 {
		return
	}
	b := []byte{byte(val), byte(val >> 8), byte(val >> 16), byte(val >> 24)}
	for _, w := range ws {
		xproto.ChangeProperty(u.c, xproto.PropModeReplace, w, prop, xproto.AtomAtom, 32, 1, b)
	}
}

// clickThrough gives w an empty input shape, so clicks land on whatever is
// underneath — the outline must never steal a click from the page.
func (u *UI) clickThrough(w xproto.Window) {
	shape.Rectangles(u.c, shape.SoSet, shape.SkInput, xproto.ClipOrderingUnsorted, w, 0, 0, nil)
}

func (u *UI) handleCursors() [9]xproto.Cursor {
	// X cursor font glyphs: the eight resize arrows in Handles order, then the
	// four-way move arrow for the tag.
	glyphs := [9]uint16{134, 138, 136, 96, 14, 16, 12, 70, 52}
	f, _ := xproto.NewFontId(u.c)
	xproto.OpenFont(u.c, f, uint16(len("cursor")), "cursor")
	var out [9]xproto.Cursor
	for i, g := range glyphs {
		out[i], _ = xproto.NewCursorId(u.c)
		xproto.CreateGlyphCursor(u.c, out[i], f, f, g, g+1, 0, 0, 0, 0xffff, 0xffff, 0xffff)
	}
	return out
}

// ---- outline, handles, tag ----------------------------------------------

// SetOutline shows or hides the outline (the "Show outline" menu item).
func (u *UI) SetOutline(on bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.shown = on
	u.layout()
}

// SetPinned shows the handles and makes the tag a grip (screen-pinned).
func (u *UI) SetPinned(on bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.pinned = on
	u.layout()
}

// Hide takes everything off screen (camera stopped).
func (u *UI) Hide() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.rect = viewport.Rect{}
	u.layout()
}

// SetTag shows text in the tag, or hides it with "". While pinned the tag
// stays up regardless, because it is the move grip.
func (u *UI) SetTag(text string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.tagMsg = text
	u.layout()
}

// SetHints sets the hint bar's reminders and whether it shows at all. The bar
// shows whenever the outline does; it never fades (figure screen-hints 1).
func (u *UI) SetHints(hints []draw.Hint, on bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.hints, u.hintsOn = hints, on
	u.layout()
}

// Flash turns the outline white and eases it back to red over about 0.6 s
// (figure screenshot-flash 1). It is on the outline, never inside the frame,
// so the camera picture doesn't flash.
func (u *UI) Flash() {
	u.mu.Lock()
	u.flashGen++
	gen := u.flashGen
	u.mu.Unlock()
	const steps = 6
	go func() {
		for i := 0; i <= steps; i++ {
			u.mu.Lock()
			if u.flashGen != gen {
				u.mu.Unlock()
				return
			}
			col := mix(0xffffff, 0xff4a3a, i, steps)
			for _, w := range u.edges {
				xproto.ChangeWindowAttributes(u.c, w, xproto.CwBackPixel, []uint32{col})
				xproto.ClearArea(u.c, false, w, 0, 0, 0, 0)
			}
			u.mu.Unlock()
			if i == 0 {
				time.Sleep(150 * time.Millisecond) // hold white long enough to see
			} else {
				time.Sleep(75 * time.Millisecond)
			}
		}
	}()
}

// mix is colour a moved i/n of the way to b, per channel.
func mix(a, b uint32, i, n int) uint32 {
	var out uint32
	for sh := 0; sh <= 16; sh += 8 {
		ca, cb := int(a>>sh&0xff), int(b>>sh&0xff)
		out |= uint32(ca+(cb-ca)*i/n) << sh
	}
	return out
}

// Move puts the outline around r.
func (u *UI) Move(r viewport.Rect) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if r == u.rect {
		return
	}
	u.rect = r
	u.layout()
}

func (u *UI) place(w xproto.Window, x, y, wd, ht int, visible bool) {
	if !visible || wd <= 0 || ht <= 0 {
		xproto.UnmapWindow(u.c, w)
		return
	}
	xproto.ConfigureWindow(u.c, w, xproto.ConfigWindowX|xproto.ConfigWindowY|xproto.ConfigWindowWidth|xproto.ConfigWindowHeight|xproto.ConfigWindowStackMode,
		[]uint32{uint32(int32(x)), uint32(int32(y)), uint32(wd), uint32(ht), xproto.StackModeAbove})
	xproto.MapWindow(u.c, w)
}

// layout positions every window around u.rect. Called with u.mu held.
func (u *UI) layout() {
	r, on := u.rect, u.rect.W > 0
	b := border
	u.place(u.edges[0], r.X-b, r.Y-b, r.W+2*b, b, on && u.shown)
	u.place(u.edges[1], r.X-b, r.Y+r.H, r.W+2*b, b, on && u.shown)
	u.place(u.edges[2], r.X-b, r.Y, b, r.H, on && u.shown)
	u.place(u.edges[3], r.X+r.W, r.Y, b, r.H, on && u.shown)

	// Handles sit outside the rectangle: their inner edge touches it, so they
	// overlap the outline and never the captured area.
	s := handleS
	pos := func(h viewport.Handle) (int, int) {
		cx, cy := r.X+r.W/2-handleS/2, r.Y+r.H/2-handleS/2
		left, right, top, bottom := r.X-handleS, r.X+r.W, r.Y-handleS, r.Y+r.H
		switch h {
		case viewport.TopLeft:
			return left, top
		case viewport.Top:
			return cx, top
		case viewport.TopRight:
			return right, top
		case viewport.Right:
			return right, cy
		case viewport.BottomRight:
			return right, bottom
		case viewport.Bottom:
			return cx, bottom
		case viewport.BottomLeft:
			return left, bottom
		}
		return left, cy
	}
	for i, h := range viewport.Handles {
		x, y := pos(h)
		u.place(u.handles[i], x, y, s, s, on && u.pinned)
	}

	text := u.tagMsg
	if u.pinned && text == "" {
		text = fmt.Sprintf("%dx%d | pinned", r.W, r.H)
	}
	u.tagShown = on && text != ""
	if u.tagShown {
		tw, th := draw.TextSize(text, 1)
		tw, th = tw+8, th+8
		x, y := r.X-b, r.Y-b-th
		if u.pinned {
			x = r.X + handleS // clear of the top-left handle
		}
		if y < 0 { // no room above: hang it below the frame
			y = r.Y + r.H + b
			if u.pinned {
				y += handleS
			}
		}
		u.place(u.tag, x, y, tw, th, true)
		u.tagBox = [4]int{x, y, tw, th}
	} else {
		u.place(u.tag, 0, 0, 0, 0, false)
	}
	if text != u.tagText {
		u.tagText = text
		u.paintTag()
	}
	u.layoutHint(r, on)
}

// layoutHint places the hint bar under the frame (below the handles when
// pinned), keeping as many hints as fit the frame's width, at least one. With
// no room below it goes above, right-aligned so it clears the size tag.
func (u *UI) layoutHint(r viewport.Rect, on bool) {
	hs := u.hints
	for len(hs) > 1 && draw.HintBarWidth(hs) > r.W+2*border {
		hs = hs[:len(hs)-1]
	}
	visible := on && u.shown && u.hintsOn && len(hs) > 0
	var x, y, w, h int
	if visible {
		w, h = draw.HintBarWidth(hs), 13+8
		gap := border
		if u.pinned {
			gap = handleS
		}
		x, y = r.X-border, r.Y+r.H+gap
		if y+h > u.screenH { // no room below
			x, y = r.X+r.W+border-w, r.Y-gap-h
			if t := u.tagBox; u.tagShown && t[1] < r.Y && x < t[0]+t[2] {
				y = t[1] - h // stack above the tag rather than over it
			}
			visible = y >= 0
		}
	}
	u.hintShown = visible
	if !visible {
		u.place(u.hint, 0, 0, 0, 0, false)
		return
	}
	u.place(u.hint, x, y, w, h, true)
	if !sameHints(hs, u.hintDrawn) {
		u.hintDrawn = append(u.hintDrawn[:0], hs...)
		u.paintHint()
	}
}

func sameHints(a, b []draw.Hint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (u *UI) paintHint() {
	if !u.hintShown || len(u.hintDrawn) == 0 {
		return
	}
	img := draw.HintBar(u.hintDrawn)
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	xproto.PutImage(u.c, xproto.ImageFormatZPixmap, xproto.Drawable(u.hint), u.tagGC,
		uint16(w), uint16(h), 0, 0, 0, xproto.Setup(u.c).DefaultScreen(u.c).RootDepth, draw.BGRA(img))
}

func (u *UI) paintTag() {
	if !u.tagShown {
		return
	}
	img := draw.Tag(u.tagText)
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	xproto.PutImage(u.c, xproto.ImageFormatZPixmap, xproto.Drawable(u.tag), u.tagGC,
		uint16(w), uint16(h), 0, 0, 0, xproto.Setup(u.c).DefaultScreen(u.c).RootDepth, draw.BGRA(img))
}

// ---- shortcuts ----------------------------------------------------------

// Bind replaces every grab with keys (action -> key) and the scroll
// modifiers. It returns one message per shortcut that could not be grabbed,
// typically because another program holds it; the others still work.
func (u *UI) Bind(keys map[string]config.Key, scrollMods uint16) []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	xproto.UngrabKey(u.c, xproto.GrabAny, u.root, xproto.ModMaskAny)
	xproto.UngrabButton(u.c, xproto.ButtonIndexAny, u.root, xproto.ModMaskAny)
	u.keys = map[keyKey]string{}

	codes := u.keycodes()
	var problems []string
	for action, k := range keys {
		code, ok := codes[k.Keysym]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: no key on this keyboard", k))
			continue
		}
		var err error
		for _, lock := range ignoredLocks {
			if err = xproto.GrabKeyChecked(u.c, true, u.root, k.Mods|lock, code,
				xproto.GrabModeAsync, xproto.GrabModeAsync).Check(); err != nil {
				break
			}
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s is taken by another program", k))
			continue
		}
		u.keys[keyKey{code, k.Mods}] = action
	}
	u.scrollMod = scrollMods
	for _, btn := range []xproto.Button{4, 5} {
		for _, lock := range ignoredLocks {
			if err := xproto.GrabButtonChecked(u.c, false, u.root, xproto.EventMaskButtonPress,
				xproto.GrabModeAsync, xproto.GrabModeAsync, xproto.WindowNone, xproto.CursorNone,
				byte(btn), scrollMods|lock).Check(); err != nil {
				problems = append(problems, "scroll-to-resize is taken by another program")
				break
			}
		}
	}
	return problems
}

// keycodes maps each keysym to the first keycode that produces it.
func (u *UI) keycodes() map[uint32]xproto.Keycode {
	setup := xproto.Setup(u.c)
	n := byte(setup.MaxKeycode - setup.MinKeycode + 1)
	m, err := xproto.GetKeyboardMapping(u.c, setup.MinKeycode, n).Reply()
	out := map[uint32]xproto.Keycode{}
	if err != nil {
		return out
	}
	per := int(m.KeysymsPerKeycode)
	for i := 0; i < int(n); i++ {
		for j := 0; j < per && j < 2; j++ { // unshifted and shifted
			sym := uint32(m.Keysyms[i*per+j])
			if sym == 0 {
				continue
			}
			if _, seen := out[sym]; !seen {
				out[sym] = setup.MinKeycode + xproto.Keycode(i)
			}
		}
	}
	return out
}

// ---- event loop ---------------------------------------------------------

// Run handles X events until the connection closes.
func (u *UI) Run() {
	for {
		e, err := u.c.WaitForEvent()
		if e == nil && err == nil {
			return // connection closed
		}
		if err != nil {
			continue // a failed request (e.g. a grab another program won); Bind reports those
		}
		switch e := e.(type) {
		case xproto.KeyPressEvent:
			mods := uint16(e.State) &^ (lockCaps | lockNum)
			u.mu.Lock()
			action := u.keys[keyKey{e.Detail, mods}]
			u.mu.Unlock()
			if action != "" && u.ev.Shortcut != nil {
				u.ev.Shortcut(action)
			}
		case xproto.ButtonPressEvent:
			if e.Event == u.root && (e.Detail == 4 || e.Detail == 5) {
				if u.ev.Scroll != nil {
					u.ev.Scroll(e.Detail == 4)
				}
				continue
			}
			u.startDrag(e)
		case xproto.MotionNotifyEvent:
			u.moveDrag(e.RootX, e.RootY, uint16(e.State)&xproto.ModMaskShift != 0, false)
		case xproto.ButtonReleaseEvent:
			u.moveDrag(e.RootX, e.RootY, uint16(e.State)&xproto.ModMaskShift != 0, true)
		case xproto.ExposeEvent:
			u.mu.Lock()
			if e.Window == u.tag {
				u.paintTag()
			} else if e.Window == u.hint {
				u.paintHint()
			} else {
				for _, h := range u.handles {
					if h == e.Window {
						o := int16((handleS - handleW) / 2)
						xproto.PolyFillRectangle(u.c, xproto.Drawable(h), u.whiteGC,
							[]xproto.Rectangle{{X: o, Y: o, Width: handleW, Height: handleW}})
					}
				}
			}
			u.mu.Unlock()
		}
	}
}

func (u *UI) startDrag(e xproto.ButtonPressEvent) {
	if e.Detail != 1 {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.pinned {
		return
	}
	h := viewport.Handle(-1)
	if e.Event != u.tag {
		found := false
		for i, w := range u.handles {
			if w == e.Event {
				h, found = viewport.Handles[i], true
			}
		}
		if !found {
			return
		}
	}
	u.drag = &dragState{handle: h, startX: e.RootX, startY: e.RootY, start: u.rect, lastRect: u.rect}
}

func (u *UI) moveDrag(x, y int16, keepAspect, done bool) {
	u.mu.Lock()
	d := u.drag
	if d == nil {
		u.mu.Unlock()
		return
	}
	dx, dy := int(x-d.startX), int(y-d.startY)
	var r viewport.Rect
	if d.handle < 0 {
		r = viewport.Moved(d.start, dx, dy, u.screenW, u.screenH)
	} else {
		r = viewport.Dragged(d.start, d.handle, dx, dy, keepAspect, u.screenW, u.screenH)
	}
	d.lastRect = r
	if done {
		u.drag = nil
	}
	u.mu.Unlock()
	if u.ev.Drag != nil {
		u.ev.Drag(r, done)
	}
}

// Close drops the connection, which also removes every window and grab.
func (u *UI) Close() { u.c.Close() }
