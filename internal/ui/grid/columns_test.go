package grid

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/settings"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view"
)

// T6: windows and choices give the expected columns, headers and counts.
func TestColumns(t *testing.T) {
	base := load(t, "tableau.json")
	counts := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	for i := range base.Gates {
		base.Gates[i].Count = counts[i]
	}
	window := func(on ...int) []bool {
		w := make([]bool, 10)
		for _, i := range on {
			w[i] = true
		}
		return w
	}
	type col struct {
		header string
		folded bool
	}
	cases := []struct {
		name    string
		window  []bool
		choices map[string]settings.Choice
		want    []col
	}{
		{"none folded", window(0, 1, 2, 3, 4, 5, 6, 7, 8, 9), nil, []col{
			{"❔", false}, {"📝", false}, {"📌", false}, {"🔧", false}, {"📐", false},
			{"🧱", false}, {"📏", false}, {"🔗", false}, {"🌍", false}, {"🚀", false}}},
		{"all folded", window(), nil, []col{{"❔…🚀 ×55", true}}},
		{"a single gate folded", window(1, 2, 3, 4, 5, 6, 7, 8, 9), nil, []col{
			{"❔ ×1", true}, {"📝", false}, {"📌", false}, {"🔧", false}, {"📐", false},
			{"🧱", false}, {"📏", false}, {"🔗", false}, {"🌍", false}, {"🚀", false}}},
		{"a run in the middle", window(0, 1, 2, 7, 8, 9), nil, []col{
			{"❔", false}, {"📝", false}, {"📌", false}, {"🔧…📏 ×22", true}, {"🔗", false}, {"🌍", false}, {"🚀", false}}},
		{"a run at each end", window(3, 4, 5), nil, []col{
			{"❔…📌 ×6", true}, {"🔧", false}, {"📐", false}, {"🧱", false}, {"📏…🚀 ×34", true}}},
		{"a show outside the window", window(2, 3), map[string]settings.Choice{"undefined": settings.Show, "unit": settings.Show}, []col{
			{"❔", false}, {"📝 ×2", true}, {"📌", false}, {"🔧", false}, {"📐…🧱 ×11", true}, {"📏", false}, {"🔗…🚀 ×27", true}}},
		{"a hide inside the window", window(0, 1, 2, 3, 4, 5, 6, 7, 8, 9), map[string]settings.Choice{"mockup": settings.Hide, "function": settings.Hide, "release": settings.Hide}, []col{
			{"❔", false}, {"📝", false}, {"📌…🔧 ×7", true}, {"📐", false}, {"🧱", false}, {"📏", false}, {"🔗", false}, {"🌍", false}, {"🚀 ×10", true}}},
		{"a hide beside a folded run joins it", window(2, 3, 4), map[string]settings.Choice{"mockup": settings.Hide}, []col{
			{"❔…📌 ×6", true}, {"🔧", false}, {"📐", false}, {"🧱…🚀 ×40", true}}},
		{"a choice for another project", window(1, 2), map[string]settings.Choice{"performance": settings.Hide}, []col{
			{"❔ ×1", true}, {"📝", false}, {"📌", false}, {"🔧…🚀 ×49", true}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tb := base
			tb.Gates = append([]view.Gate(nil), base.Gates...)
			for i := range tb.Gates {
				tb.Gates[i].Window = c.window[i]
			}
			mt := buildMetrics(&tb, c.choices, ui.Measure{})
			if len(mt.cols) != len(c.want) {
				t.Fatalf("%d columns, want %d: %+v", len(mt.cols), len(c.want), mt.cols)
			}
			for i, w := range c.want {
				if got := mt.cols[i]; got.header != w.header || got.folded != w.folded {
					t.Errorf("column %d = %q folded %v, want %q folded %v", i, got.header, got.folded, w.header, w.folded)
				}
			}
		})
	}
}

func TestColumnWidths(t *testing.T) {
	tb := load(t, "tableau.json")
	mt := buildMetrics(&tb, nil, ui.Measure{})
	// Widths come from every row, folded or not: 🔧 holds only 🤖, 📌 holds 🤖👀.
	want := []int{5, 2, 4, 2, 4, 4, 2, 2, 4, 2}
	if len(mt.cols) != len(want) {
		t.Fatalf("%d columns", len(mt.cols))
	}
	for i, w := range want {
		if mt.cols[i].w != w {
			t.Errorf("column %d (%s) is %d wide, want %d", i, mt.cols[i].header, mt.cols[i].w, w)
		}
	}
	if mt.idW != 4 {
		t.Errorf("id column = %d, want 4", mt.idW)
	}
	// The widest label any row can have, with its count: depth 4, marker, the longest title.
	longest := 0
	for i, r := range tb.Rows {
		longest = max(longest, ansi.StringWidth(label(r, r.Parent, mt.below[i])))
	}
	if mt.natural != longest || longest != 49 {
		t.Errorf("natural width = %d, longest label %d, want 49", mt.natural, longest)
	}
}

func TestLabel(t *testing.T) {
	cases := []struct {
		row    view.Row
		folded bool
		below  int
		want   string
	}{
		{view.Row{Title: "Leaf", Depth: 2}, false, 0, "      Leaf"},
		{view.Row{Title: "Open", Depth: 1, Parent: true}, false, 4, "  ▾ Open"},
		{view.Row{Title: "Shut", Depth: 1, Parent: true}, true, 31, "  ▸ Shut (31)"},
		{view.Row{Title: "Spine", Depth: 0, Parent: true, Label: "spine"}, true, 2, "▸ Spine · spine (2)"},
		{view.Row{Title: "Kin", Depth: 1, Label: "sibling"}, false, 0, "    Kin · sibling"},
	}
	for _, c := range cases {
		if got := label(c.row, c.folded, c.below); got != c.want {
			t.Errorf("label = %q, want %q", got, c.want)
		}
	}
}

// T7: x, s and X.
func TestHideShowWindow(t *testing.T) {
	t.Run("x hides a column the window shows and the file says so", func(t *testing.T) {
		path := settingsFile(t, "")
		r := newRig(t, fixtures(), path, GlanceStart(), 100, 14).keys("right right right right x")
		if got := loadSettings(t, path); len(got) != 1 || got["function"] != settings.Hide {
			t.Errorf("settings = %v", got)
		}
		if !strings.Contains(r.line(1), "🔧 ×0") {
			t.Errorf("header = %q", r.line(1))
		}
		if !strings.HasPrefix(r.status(), "437e × 🔧 ×0 ·") {
			t.Errorf("status = %q", r.status())
		}
	})
	t.Run("s at the first gate column takes the last gate of the run", func(t *testing.T) {
		path := settingsFile(t, "")
		r := newRig(t, fixtures(), path, GlanceStart(), 100, 14).keys("down enter right")
		_ = r
		// Window 0 folds ❔…📝: the run starts the grid.
		start := GlanceStart()
		start.Request = reqWindow0
		r = newRig(t, fixtures(), path, start, 100, 14).keys("right s")
		if got := loadSettings(t, path); len(got) != 1 || got["defined"] != settings.Show {
			t.Errorf("settings = %v, want defined shown", got)
		}
		if !strings.Contains(r.line(1), "❔ ×0 📝") {
			t.Errorf("header = %q", r.line(1))
		}
	})
	t.Run("s elsewhere takes the first gate of the run and forgets its hide", func(t *testing.T) {
		path := settingsFile(t, "")
		r := newRig(t, fixtures(), path, GlanceStart(), 100, 14)
		r.keys("right right right right x right x") // hides function and design: 🔧…📐 is column 4
		if got := loadSettings(t, path); len(got) != 2 {
			t.Fatalf("settings = %v", got)
		}
		r.keys("left s")
		if got := loadSettings(t, path); len(got) != 1 || got["design"] != settings.Hide {
			t.Errorf("settings = %v, want design hidden alone", got)
		}
		if !strings.Contains(r.line(1), "🔧 📐 ×4") && !strings.Contains(r.line(1), "🔧  📐 ×4") {
			t.Errorf("header = %q", r.line(1))
		}
	})
	t.Run("x on a column shown only by a choice forgets the show", func(t *testing.T) {
		path := settingsFile(t, "")
		r := newRig(t, fixtures(), path, GlanceStart(), 100, 14).keys("right s")
		if got := loadSettings(t, path); got["undefined"] != settings.Show {
			t.Fatalf("settings = %v", got)
		}
		r.keys("x")
		if got := loadSettings(t, path); len(got) != 0 {
			t.Errorf("settings = %v, want none", got)
		}
		if !strings.Contains(r.line(1), "❔ ×0") {
			t.Errorf("header = %q", r.line(1))
		}
	})
	t.Run("X returns to the window and keeps the keys of another project", func(t *testing.T) {
		path := settingsFile(t, fourHidden)
		r := newRig(t, fixtures(), path, GlanceStart(), 100, 14).keys("right right s X")
		got := loadSettings(t, path)
		if len(got) != 1 || got["performance"] != settings.Hide {
			t.Errorf("settings = %v, want performance alone", got)
		}
		if strings.Contains(r.line(1), "🔧…📐") || !strings.Contains(r.line(1), "🚀") {
			t.Errorf("header = %q", r.line(1))
		}
		if strings.Contains(r.status(), "changed") {
			t.Errorf("status = %q", r.status())
		}
	})
	t.Run("the two notices", func(t *testing.T) {
		path := settingsFile(t, "")
		r := newRig(t, fixtures(), path, GlanceStart(), 100, 14).keys("x")
		if r.status() != "the cursor is not on a gate column" {
			t.Errorf("status = %q", r.status())
		}
		r.keys("right x") // the folded column
		if r.status() != "the cursor is not on a gate column" {
			t.Errorf("status = %q", r.status())
		}
		r.keys("right s") // a shown column
		if r.status() != "the cursor is not on a folded column" {
			t.Errorf("status = %q", r.status())
		}
		r.keys("left left s")
		if r.status() != "the cursor is not on a folded column" {
			t.Errorf("task column: status = %q", r.status())
		}
		if _, err := os.Stat(path); err == nil {
			t.Error("a refused key wrote the settings")
		}
	})
	t.Run("the cursor stays within the columns that remain", func(t *testing.T) {
		r := newRig(t, fixtures(), "", GlanceStart(), 100, 14)
		r.keys("right right right right right right right right right x") // 🌍
		r.keys("right x")                                                 // 🚀 joins the run
		if !strings.HasPrefix(r.status(), "437e × 🌍…🚀 ×0") {
			t.Errorf("status = %q", r.status())
		}
		// Hiding the first shown column merges it into the run before it.
		r.keys("X left left left left left left left x")
		if !strings.Contains(r.line(1), "❔…📝 ×29") || !strings.HasPrefix(r.status(), "437e × 📌 mockup") {
			t.Errorf("header %q, status %q", r.line(1), r.status())
		}
	})
}

// T8: the settings through the grid.
func TestSettingsFiles(t *testing.T) {
	t.Run("a malformed file gives the notice and writes nothing", func(t *testing.T) {
		body := "{not json"
		path := settingsFile(t, body)
		r := newRig(t, fixtures(), path, GlanceStart(), 100, 14)
		if n := r.notices; len(n) != 1 || !n[0].Err || !strings.HasPrefix(n[0].Text, "settings: ") || !strings.HasSuffix(n[0].Text, "; this session keeps no column choice") {
			t.Errorf("notices = %+v", n)
		}
		if s := r.status(); !strings.HasPrefix(s, "error: settings: ") {
			t.Errorf("status = %q", s)
		}
		r.keys("right right right right x")
		if got := readFile(t, path); got != body {
			t.Errorf("the file became %q", got)
		}
	})
	t.Run("version 2 gives the notice and writes nothing", func(t *testing.T) {
		body := `{"version": 2, "columns": {"design": "hide", "future": "x"}}`
		path := settingsFile(t, body)
		r := newRig(t, fixtures(), path, GlanceStart(), 100, 14)
		if n := r.notices; len(n) != 1 || !strings.Contains(n[0].Text, "version 2") || !strings.HasSuffix(n[0].Text, "; this session keeps no column choice") {
			t.Errorf("notices = %+v", n)
		}
		r.keys("right right right right x X")
		if got := readFile(t, path); got != body {
			t.Errorf("the file became %q", got)
		}
		// The window still rules: the file's choice is not applied.
		if !strings.Contains(r.line(1), "📐") {
			t.Errorf("header = %q", r.line(1))
		}
	})
	t.Run("a missing file means no choice", func(t *testing.T) {
		path := settingsFile(t, "")
		r := newRig(t, fixtures(), path, GlanceStart(), 100, 14)
		if strings.Contains(r.status(), "settings") {
			t.Errorf("status = %q", r.status())
		}
	})
	t.Run("an unwritable directory gives an error and keeps the choice in memory", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("a read-only directory stops no write here")
		}
		dir := t.TempDir()
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
		path := filepath.Join(dir, "settings.json")
		r := newRig(t, fixtures(), path, GlanceStart(), 100, 14).keys("right right right right x")
		if s := r.status(); !strings.HasPrefix(s, "error: settings: ") {
			t.Errorf("status = %q", s)
		}
		if !strings.Contains(r.line(1), "🔧 ×0") {
			t.Errorf("header = %q", r.line(1))
		}
	})
	t.Run("a choice survives a second session", func(t *testing.T) {
		path := settingsFile(t, "")
		newRig(t, fixtures(), path, GlanceStart(), 100, 14).keys("right right right right x")
		r := newRig(t, fixtures(), path, GlanceStart(), 100, 14)
		if !strings.Contains(r.line(1), "🔧 ×0") || !strings.Contains(r.status(), "1 changed") {
			t.Errorf("header %q, status %q", r.line(1), r.status())
		}
	})
	t.Run("an empty path keeps no choice", func(t *testing.T) {
		r := newRig(t, fixtures(), "", GlanceStart(), 100, 14).keys("right right right right x")
		if !strings.Contains(r.line(1), "🔧 ×0") {
			t.Errorf("header = %q", r.line(1))
		}
	})
}
