package config

import (
	"fmt"
	"strings"
)

// Modifier bits, matching X11's state mask so the X side can use them as is.
const (
	Shift = 1 << 0
	Ctrl  = 1 << 2
	Alt   = 1 << 3 // Mod1
	Super = 1 << 6 // Mod4
)

// Key is a parsed combination: modifier bits and an X keysym.
type Key struct {
	Mods   uint16
	Keysym uint32
	Text   string // as written, for messages
}

var modNames = map[string]uint16{
	"ctrl": Ctrl, "control": Ctrl, "alt": Alt, "shift": Shift, "super": Super, "win": Super,
}

// named keys beyond letters and digits, as people write them.
var keyNames = map[string]uint32{
	"=": 0x3d, "equal": 0x3d, "plus": 0x3d, "-": 0x2d, "minus": 0x2d,
	"space": 0x20, "[": 0x5b, "]": 0x5d, ",": 0x2c, ".": 0x2e, "/": 0x2f,
	"home": 0xff50, "end": 0xff57, "pageup": 0xff55, "pagedown": 0xff56,
	"insert": 0xff63, "pause": 0xff13, "print": 0xff61,
}

// ParseMods parses "Ctrl+Alt" (modifiers only).
func ParseMods(s string) (uint16, error) {
	var m uint16
	for _, part := range strings.Split(s, "+") {
		bit, ok := modNames[strings.ToLower(strings.TrimSpace(part))]
		if !ok {
			return 0, fmt.Errorf("%q: %q is not a modifier (Ctrl, Alt, Shift, Super)", s, part)
		}
		m |= bit
	}
	if m == 0 {
		return 0, fmt.Errorf("%q: needs at least one modifier", s)
	}
	return m, nil
}

// ParseKey parses "Ctrl+Alt+C". At least one modifier is required: a bare
// key grabbed globally would stop that key working everywhere else.
func ParseKey(s string) (Key, error) {
	// Split on "+" but allow the key itself to be "+" written as "plus".
	i := strings.LastIndex(s, "+")
	if i <= 0 || i == len(s)-1 {
		return Key{}, fmt.Errorf("%q: want modifiers and a key, e.g. Ctrl+Alt+C", s)
	}
	mods, err := ParseMods(s[:i])
	if err != nil {
		return Key{}, err
	}
	name := strings.TrimSpace(s[i+1:])
	lower := strings.ToLower(name)
	var sym uint32
	switch {
	case len(lower) == 1 && lower[0] >= 'a' && lower[0] <= 'z':
		sym = uint32(lower[0]) // X keysyms for letters are their lowercase ASCII
	case len(lower) == 1 && lower[0] >= '0' && lower[0] <= '9':
		sym = uint32(lower[0])
	case len(lower) >= 2 && lower[0] == 'f':
		var n int
		if _, err := fmt.Sscanf(lower[1:], "%d", &n); err == nil && n >= 1 && n <= 12 {
			sym = 0xffbe + uint32(n-1)
		}
	}
	if sym == 0 {
		sym = keyNames[lower]
	}
	if sym == 0 {
		return Key{}, fmt.Errorf("%q: unknown key %q", s, name)
	}
	return Key{Mods: mods, Keysym: sym, Text: s}, nil
}

// PresetKey is the key that selects preset n (1-9).
func PresetKey(mods string, n int) (Key, error) {
	return ParseKey(fmt.Sprintf("%s+%d", mods, n))
}

// ShortText is "Ctrl+Alt+C" for menu labels: the combination as written.
func (k Key) String() string { return k.Text }
