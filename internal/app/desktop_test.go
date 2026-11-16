package app

import "testing"

func TestUTF8Locale(t *testing.T) {
	cases := []struct {
		all, ctype, lang string
		want             bool
	}{
		{"", "", "en_AU.UTF-8", true},
		{"", "", "", false},
		{"C", "", "en_AU.UTF-8", false}, // LC_ALL wins
		{"", "en_US.utf8", "C", true},   // then LC_CTYPE
		{"", "", "POSIX", false},
	}
	for _, c := range cases {
		t.Setenv("LC_ALL", c.all)
		t.Setenv("LC_CTYPE", c.ctype)
		t.Setenv("LANG", c.lang)
		if got := utf8Locale(); got != c.want {
			t.Errorf("LC_ALL=%q LC_CTYPE=%q LANG=%q: got %v", c.all, c.ctype, c.lang, got)
		}
	}
}

func TestDesktopEntryExec(t *testing.T) {
	e := DesktopEntry("/home/u/.local/bin/screen-cam")
	if want := "Exec=/home/u/.local/bin/screen-cam\n"; !contains(e, want) || !contains(e, "[Desktop Entry]\n") {
		t.Errorf("entry:\n%s", e)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
