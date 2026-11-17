package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/godbus/dbus/v5"
)

// notify shows a desktop notification. Failures are ignored: a missing
// notification daemon must not stop the camera.
func notify(summary, body string) {
	conn, err := dbus.SessionBus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "screen-cam:", summary+":", body)
		return
	}
	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	obj.Call("org.freedesktop.Notifications.Notify", 0, "Screen Cam", uint32(0), "camera-web",
		summary, body, []string{}, map[string]dbus.Variant{}, int32(5000))
}

// debug turns on a one-line state log after every change
// (SCREEN_CAM_DEBUG=1), which the virtual-display checks read.
var debug = os.Getenv("SCREEN_CAM_DEBUG") == "1"

// debugf writes one line to the debug log (the e2e checks read it).
func debugf(format string, args ...any) {
	if debug {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func (a *App) logState() {
	if !debug {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	fmt.Fprintf(os.Stderr, "state live=%v pinned=%v size=%dx%d rect=%d,%d,%dx%d preset=%d presets=%d outline=%v\n",
		a.live, a.pinned, a.w, a.h, a.rect.X, a.rect.Y, a.rect.W, a.rect.H, a.active, len(a.presets), a.conf.ShowOutline)
}

// askName asks for a preset name with zenity, pre-filled with def. ok is
// false when the user cancels (figure save-preset 1: Cancel saves nothing).
func askName(size, def string) (name string, ok bool, err error) {
	cmd := exec.Command("zenity", "--entry", "--title=Save Screen Cam preset",
		"--text=Name for this "+size+" view", "--entry-text="+def)
	// zenity rejects a non-ASCII argument ("640×480") as an unknown option
	// when the locale is not UTF-8 — e.g. a session started with no LANG.
	if !utf8Locale() {
		cmd.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var exit *exec.ExitError
	// zenity exits 1 for Cancel or a closed dialog. Any other failure (it
	// could not start, no display) is an error to report, not a quiet Cancel.
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("zenity: %w %s", err, strings.TrimSpace(stderr.String()))
	}
	name = strings.TrimSpace(string(out))
	if name == "" {
		name = def
	}
	return name, true, nil
}

// utf8Locale reports whether the effective locale is UTF-8, using the
// POSIX precedence LC_ALL > LC_CTYPE > LANG.
func utf8Locale() bool {
	for _, v := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if l := os.Getenv(v); l != "" {
			l = strings.ToLower(l)
			return strings.Contains(l, "utf-8") || strings.Contains(l, "utf8")
		}
	}
	return false
}

// askFolder asks for a folder with zenity's folder picker, starting in dir.
// ok is false on Cancel; other failures are errors, as in askName.
func askFolder(dir string) (folder string, ok bool, err error) {
	cmd := exec.Command("zenity", "--file-selection", "--directory",
		"--title=Screen Cam screenshot folder", "--filename="+strings.TrimSuffix(dir, "/")+"/")
	if !utf8Locale() {
		cmd.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("zenity: %w %s", err, strings.TrimSpace(stderr.String()))
	}
	folder = strings.TrimSpace(string(out))
	return folder, folder != "", nil
}

// tildePath writes a path under the home directory as ~/…, the way the
// config file shows it.
func tildePath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+"/") {
		return "~" + p[len(home):]
	}
	return p
}

// openFile opens path in the user's default application for its type.
func openFile(path string) error {
	cmd := exec.Command("xdg-open", path)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// autostartPath is the login-start entry (figure tray-menu 8, D6).
func autostartPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "autostart", "screen-cam.desktop"), nil
}

func autostartEnabled() bool {
	p, err := autostartPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// DesktopEntry is the .desktop text for exe, used both for the menu entry
// that `make install` writes and for the autostart entry.
func DesktopEntry(exe string) string {
	return "[Desktop Entry]\nType=Application\nName=Screen Cam\n" +
		"Comment=Turn part of the screen into a webcam\n" +
		"Exec=" + exe + "\nIcon=camera-web\nTerminal=false\nCategories=Utility;Video;\n" +
		"X-GNOME-Autostart-enabled=true\n"
}

func setAutostart(on bool) error {
	p, err := autostartPath()
	if err != nil {
		return err
	}
	if !on {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(DesktopEntry(exe)), 0o644)
}

// singleInstance takes a lock so a second copy (say, from autostart while
// one is already running) exits instead of fighting over the shortcuts.
func singleInstance() (release func(), err error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	f, err := os.OpenFile(filepath.Join(dir, "screen-cam.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("screen-cam is already running (look for its tray icon)")
	}
	return func() { f.Close() }, nil
}
