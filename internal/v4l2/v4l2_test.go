package v4l2

import (
	"testing"
	"unsafe"
)

// The ioctl number encodes the struct size; if the mirror drifts from the
// kernel's 208 bytes the kernel rejects the call (or worse, misreads it).
func TestFormatMatchesKernelLayout(t *testing.T) {
	if got := unsafe.Sizeof(format{}); got != 208 {
		t.Fatalf("sizeof(format) = %d, want 208", got)
	}
	if got := unsafe.Offsetof(format{}.Pix); got != 8 {
		t.Fatalf("offset of Pix = %d, want 8", got)
	}
	if size := (vidiocSFmt >> 16) & 0x3fff; size != 208 {
		t.Fatalf("ioctl number encodes size %d, want 208", size)
	}
}
