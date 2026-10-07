package detail

import (
	"strings"
	"testing"
)

// T2: the screens. Each runs the grid on the grid's fixtures and the panes on
// the list fixtures, and compares the stripped screen.
func TestScreens(t *testing.T) {
	cases := []struct {
		file string
		w, h int
		keys string
	}{
		{"stack-100x30.txt", 100, 30, "E down down down down 1"},
		{"pair-80x24.txt", 80, 24, "E down down down down 2 4"},
		{"side-160x30.txt", 160, 30, "E down down down down 1 2 esc"},
		{"four-160x45.txt", 160, 45, "E down down down down 1 2 3 4"},
		{"floor-40x8.txt", 40, 8, "E down down down down 1 down down down"},
		{"zoom-100x30.txt", 100, 30, "3 z p"},
		{"goto-120x24.txt", 120, 24, "2 d down down down enter"},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			r := newRig(t, "testdata", c.w, c.h).keys(c.keys)
			want := strings.TrimRight(golden(t, c.file), "\n")
			if got := r.screen(); got != want {
				t.Errorf("screen differs from %s:\n%s\nwant:\n%s", c.file, got, want)
			}
		})
	}
}
