// Package shot turns a captured frame area into a PNG and saves it.
package shot

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PNG encodes a w×h BGRA capture (capture.Grab's layout) as an opaque PNG.
func PNG(bgra []byte, w, h int) ([]byte, error) {
	if len(bgra) < w*h*4 {
		return nil, fmt.Errorf("shot: %d bytes for %dx%d", len(bgra), w, h)
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		p := img.Pix[i*4 : i*4+4 : i*4+4]
		p[0], p[1], p[2], p[3] = bgra[i*4+2], bgra[i*4+1], bgra[i*4], 0xff
	}
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Name is the file name for a screenshot taken at t.
func Name(t time.Time) string {
	return "screen-cam_" + t.Format("2006-01-02_15-04-05") + ".png"
}

// Save writes data into dir (created if needed) under Name(t), adding -2, -3…
// when two screenshots land in the same second. Returns the path written.
func Save(data []byte, dir string, t time.Time) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(Name(t), ".png")
	for i := 1; ; i++ {
		name := base + ".png"
		if i > 1 {
			name = fmt.Sprintf("%s-%d.png", base, i)
		}
		p := filepath.Join(dir, name)
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, err = f.Write(data)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(p)
			return "", err
		}
		return p, nil
	}
}

// Dir resolves the configured screenshot folder: a leading ~ is the home
// directory, and empty means "Screen Cam" in the user's Pictures folder.
func Dir(configured string) string {
	home, _ := os.UserHomeDir()
	switch {
	case configured == "":
		return filepath.Join(picturesDir(home), "Screen Cam")
	case configured == "~":
		return home
	case strings.HasPrefix(configured, "~/"):
		return filepath.Join(home, configured[2:])
	}
	return configured
}

func picturesDir(home string) string {
	if out, err := exec.Command("xdg-user-dir", "PICTURES").Output(); err == nil {
		// xdg-user-dir answers $HOME when the folder isn't configured; that
		// would scatter screenshots in the home directory.
		if d := strings.TrimSpace(string(out)); d != "" && filepath.Clean(d) != filepath.Clean(home) {
			return d
		}
	}
	return filepath.Join(home, "Pictures")
}
