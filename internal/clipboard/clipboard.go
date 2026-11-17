// Package clipboard puts a PNG on the X11 clipboard and serves it for as long
// as this process owns the selection, in pure Go (ADR-0001: no xclip).
//
// X has no clipboard store: the owner answers each paste itself. A paste of
// more than one request's worth of data uses the ICCCM INCR protocol: the
// owner announces the size, then writes a chunk each time the requestor
// deletes the previous one, and ends with an empty chunk.
package clipboard

import (
	"fmt"
	"sync"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// Chunk is the largest single property write; bigger data goes by INCR.
// 64 KiB stays well inside the core protocol's 256 KiB request limit.
// A variable so tests can force INCR with small data.
var Chunk = 64 << 10

// Owner holds the clipboard on its own X connection, so serving a paste never
// waits behind the overlay's event loop.
type Owner struct {
	c   *xgb.Conn
	win xproto.Window

	clipboard, targets, png, incr, atomType xproto.Atom

	mu   sync.Mutex
	data []byte
	// INCR transfers in flight, by requestor window and property.
	sends map[sendKey]*send
}

type sendKey struct {
	w xproto.Window
	p xproto.Atom
}

type send struct {
	data []byte
	off  int
}

// New connects to the display and starts answering paste requests.
func New() (*Owner, error) {
	c, err := xgb.NewConn()
	if err != nil {
		return nil, fmt.Errorf("clipboard: connect to X: %w", err)
	}
	o := &Owner{c: c, sends: map[sendKey]*send{}}
	for _, a := range []struct {
		name string
		dst  *xproto.Atom
	}{{"CLIPBOARD", &o.clipboard}, {"TARGETS", &o.targets}, {"image/png", &o.png}, {"INCR", &o.incr}, {"ATOM", &o.atomType}} {
		r, err := xproto.InternAtom(c, false, uint16(len(a.name)), a.name).Reply()
		if err != nil {
			c.Close()
			return nil, err
		}
		*a.dst = r.Atom
	}
	s := xproto.Setup(c).DefaultScreen(c)
	if o.win, err = xproto.NewWindowId(c); err != nil {
		c.Close()
		return nil, err
	}
	xproto.CreateWindow(c, 0, o.win, s.Root, -1, -1, 1, 1, 0, xproto.WindowClassInputOnly, s.RootVisual, 0, nil)
	go o.loop()
	return o, nil
}

// SetPNG takes the clipboard and serves png on it until another program copies.
func (o *Owner) SetPNG(png []byte) error {
	o.mu.Lock()
	o.data = png
	o.mu.Unlock()
	if err := xproto.SetSelectionOwnerChecked(o.c, o.win, o.clipboard, xproto.TimeCurrentTime).Check(); err != nil {
		return err
	}
	r, err := xproto.GetSelectionOwner(o.c, o.clipboard).Reply()
	if err != nil {
		return err
	}
	if r.Owner != o.win {
		return fmt.Errorf("clipboard: another program kept the selection")
	}
	return nil
}

// Close gives up the clipboard and the connection.
func (o *Owner) Close() { o.c.Close() }

func (o *Owner) loop() {
	for {
		ev, err := o.c.WaitForEvent()
		if ev == nil && err == nil {
			return // connection closed
		}
		switch e := ev.(type) {
		case xproto.SelectionRequestEvent:
			o.answer(e)
		case xproto.SelectionClearEvent:
			o.mu.Lock()
			o.data = nil
			o.mu.Unlock()
		case xproto.PropertyNotifyEvent:
			if e.State == xproto.PropertyDelete {
				o.next(sendKey{e.Window, e.Atom})
			}
		}
	}
}

func (o *Owner) answer(e xproto.SelectionRequestEvent) {
	prop := e.Property
	if prop == xproto.AtomNone { // pre-ICCCM requestors
		prop = e.Target
	}
	o.mu.Lock()
	data := o.data
	o.mu.Unlock()

	ok := true
	switch {
	case data == nil || e.Selection != o.clipboard:
		ok = false
	case e.Target == o.targets:
		list := make([]byte, 8)
		xgb.Put32(list, uint32(o.targets))
		xgb.Put32(list[4:], uint32(o.png))
		xproto.ChangeProperty(o.c, xproto.PropModeReplace, e.Requestor, prop, o.atomType, 32, 2, list)
	case e.Target == o.png && len(data) <= Chunk:
		xproto.ChangeProperty(o.c, xproto.PropModeReplace, e.Requestor, prop, o.png, 8, uint32(len(data)), data)
	case e.Target == o.png:
		// INCR: watch the requestor's properties, announce the size; each
		// deletion of the property asks for the next chunk (next).
		xproto.ChangeWindowAttributes(o.c, e.Requestor, xproto.CwEventMask, []uint32{xproto.EventMaskPropertyChange})
		o.mu.Lock()
		o.sends[sendKey{e.Requestor, prop}] = &send{data: data}
		o.mu.Unlock()
		size := make([]byte, 4)
		xgb.Put32(size, uint32(len(data)))
		xproto.ChangeProperty(o.c, xproto.PropModeReplace, e.Requestor, prop, o.incr, 32, 1, size)
	default:
		ok = false
	}
	if !ok {
		prop = xproto.AtomNone
	}
	n := xproto.SelectionNotifyEvent{Time: e.Time, Requestor: e.Requestor, Selection: e.Selection, Target: e.Target, Property: prop}
	xproto.SendEvent(o.c, false, e.Requestor, 0, string(n.Bytes()))
}

// next writes the next INCR chunk, or the empty chunk that ends the transfer.
func (o *Owner) next(k sendKey) {
	o.mu.Lock()
	s := o.sends[k]
	if s == nil {
		o.mu.Unlock()
		return
	}
	end := min(s.off+Chunk, len(s.data))
	chunk := s.data[s.off:end]
	s.off = end
	if len(chunk) == 0 {
		delete(o.sends, k)
	}
	o.mu.Unlock()
	xproto.ChangeProperty(o.c, xproto.PropModeReplace, k.w, k.p, o.png, 8, uint32(len(chunk)), chunk)
	if len(chunk) == 0 {
		xproto.ChangeWindowAttributes(o.c, k.w, xproto.CwEventMask, []uint32{0})
	}
}
