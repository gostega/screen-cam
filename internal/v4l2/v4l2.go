// Package v4l2 writes pictures to a v4l2loopback device, which browsers then
// list as a webcam. The format is set once, when the device is opened; it
// never changes while the device is open, so a browser reading it is never
// asked to renegotiate (ADR-0001).
package v4l2

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// pixFormat mirrors struct v4l2_pix_format.
type pixFormat struct {
	Width, Height, PixelFormat, Field, BytesPerLine, SizeImage,
	Colorspace, Priv, Flags, YcbcrEnc, Quantization, XferFunc uint32
}

// format mirrors struct v4l2_format: a u32 type, then a 200-byte union that
// the kernel aligns to 8 bytes on 64-bit builds.
type format struct {
	Type uint32
	_    uint32
	Pix  pixFormat
	_    [200 - unsafe.Sizeof(pixFormat{})]byte
}

const (
	bufTypeVideoOutput = 2
	fieldNone          = 1
	pixFmtYU12         = 0x32315559 // 'YU12', planar I420
	colorspaceSRGB     = 8
	// _IOWR('V', 5, struct v4l2_format) with sizeof == 208.
	vidiocSFmt = 0xc0d05605
)

// Device is an open loopback output.
type Device struct {
	f    *os.File
	size int
}

// Open opens path and sets it to w×h I420.
func Open(path string, w, h int) (*Device, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	size := w * h * 3 / 2
	fm := format{Type: bufTypeVideoOutput, Pix: pixFormat{
		Width: uint32(w), Height: uint32(h), PixelFormat: pixFmtYU12, Field: fieldNone,
		BytesPerLine: uint32(w), SizeImage: uint32(size), Colorspace: colorspaceSRGB,
	}}
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, f.Fd(), vidiocSFmt, uintptr(unsafe.Pointer(&fm))); e != 0 {
		f.Close()
		return nil, fmt.Errorf("%s: set format %dx%d YU12: %w (is it a v4l2loopback device?)", path, w, h, e)
	}
	return &Device{f: f, size: size}, nil
}

// Write sends one whole picture.
func (d *Device) Write(pic []byte) error {
	if len(pic) != d.size {
		return fmt.Errorf("picture is %d bytes, device expects %d", len(pic), d.size)
	}
	_, err := d.f.Write(pic)
	return err
}

// Close releases the device; readers stop receiving frames.
func (d *Device) Close() error { return d.f.Close() }
