// screen-cam turns part of the screen into a webcam: it captures a viewport
// that follows the mouse and writes it to a v4l2loopback device.
//
// The tray app (spec.md) is pending design review. Until then two commands
// drive the engine directly:
//
//	screen-cam follow [-size 640x480] [-fps 30]   viewport follows the pointer
//	screen-cam pattern [-seconds 0]               colour bars, no screen capture
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gostega/screen-cam/internal/capture"
	"github.com/gostega/screen-cam/internal/frame"
	"github.com/gostega/screen-cam/internal/v4l2"
	"github.com/gostega/screen-cam/internal/viewport"
)

// Version is stamped at build time from `git describe` (see Makefile).
var Version = "dev"

// The camera's picture size never changes while it is open (ADR-0001).
const outW, outH = 1280, 720

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-version") {
		fmt.Println("screen-cam", Version)
		return
	}
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: screen-cam follow|pattern [flags]   (the tray app is not built yet)")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "follow":
		err = follow(os.Args[2:])
	case "pattern":
		err = pattern(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "screen-cam:", err)
		os.Exit(1)
	}
}

func commonFlags(fs *flag.FlagSet) (dev *string, fps *int) {
	return fs.String("device", "/dev/video10", "v4l2loopback device to write to"),
		fs.Int("fps", 30, "frames per second")
}

// stopOnSignal returns a channel closed on Ctrl-C or SIGTERM.
func stopOnSignal() <-chan os.Signal {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	return c
}

func follow(args []string) error {
	fs := flag.NewFlagSet("follow", flag.ExitOnError)
	dev, fps := commonFlags(fs)
	size := fs.String("size", "640x480", "viewport size, WxH")
	fs.Parse(args)
	var w, h int
	if _, err := fmt.Sscanf(*size, "%dx%d", &w, &h); err != nil {
		return fmt.Errorf("-size %q: want WxH, e.g. 640x480", *size)
	}

	x, err := capture.Connect()
	if err != nil {
		return err
	}
	defer x.Close()
	cam, err := v4l2.Open(*dev, outW, outH)
	if err != nil {
		return err
	}
	defer cam.Close()

	out := frame.NewI420(outW, outH)
	var sc *frame.Scaler
	stop := stopOnSignal()
	tick := time.NewTicker(time.Second / time.Duration(*fps))
	defer tick.Stop()
	fmt.Printf("screen-cam %s: %dx%d following the mouse -> %s (Ctrl-C to stop)\n", Version, w, h, *dev)
	for {
		select {
		case <-stop:
			return nil
		case <-tick.C:
		}
		px, py, err := x.Pointer()
		if err != nil {
			return err
		}
		r := viewport.CenteredOn(px, py, w, h, x.ScreenW, x.ScreenH)
		if sc == nil || !sc.Matches(r.W, r.H) {
			sc = frame.NewScaler(r.W, r.H, out)
		}
		img, err := x.Grab(x.Root(), r.X, r.Y, r.W, r.H)
		if err != nil {
			return err
		}
		sc.Render(img, r.W*4)
		if err := cam.Write(out.Buf); err != nil {
			return err
		}
	}
}

// Bars are the eight colours of a standard test card, left to right.
var Bars = [8][3]byte{
	{255, 255, 255}, {255, 255, 0}, {0, 255, 255}, {0, 255, 0},
	{255, 0, 255}, {255, 0, 0}, {0, 0, 255}, {0, 0, 0},
}

// barsImage is a w×h BGRA picture of the eight bars.
func barsImage(w, h int) []byte {
	p := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := Bars[x*8/w]
			o := (y*w + x) * 4
			p[o], p[o+1], p[o+2], p[o+3] = c[2], c[1], c[0], 255
		}
	}
	return p
}

func pattern(args []string) error {
	fs := flag.NewFlagSet("pattern", flag.ExitOnError)
	dev, fps := commonFlags(fs)
	secs := fs.Int("seconds", 0, "stop after this many seconds (0 = until Ctrl-C)")
	fs.Parse(args)

	cam, err := v4l2.Open(*dev, outW, outH)
	if err != nil {
		return err
	}
	defer cam.Close()
	out := frame.NewI420(outW, outH)
	frame.NewScaler(outW, outH, out).Render(barsImage(outW, outH), outW*4)

	stop := stopOnSignal()
	var deadline <-chan time.Time
	if *secs > 0 {
		deadline = time.After(time.Duration(*secs) * time.Second)
	}
	tick := time.NewTicker(time.Second / time.Duration(*fps))
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return nil
		case <-deadline:
			return nil
		case <-tick.C:
			if err := cam.Write(out.Buf); err != nil {
				if errors.Is(err, syscall.EPIPE) {
					return nil
				}
				return err
			}
		}
	}
}
