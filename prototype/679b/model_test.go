package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var files = []string{
	"testdata/global-tableau.json",
	"testdata/global-tableau-window0.json",
	"testdata/global-tableau-state-at-next.json",
}

func load(t *testing.T, names ...string) []Data {
	t.Helper()
	if len(names) == 0 {
		names = files
	}
	var out []Data
	for _, n := range names {
		d, err := LoadData(n)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, d)
	}
	return out
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		tm, _ := m.Update(keyMsg(k))
		m = tm.(Model)
	}
	return m
}

func size(m Model, w, h int) Model {
	tm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return tm.(Model)
}

func ids(m Model) []string {
	var out []string
	for _, i := range m.visible() {
		out = append(out, m.data().Rows[i].ID)
	}
	return out
}

// An independent width model, kept apart from Lip Gloss so the test can
// disagree with it. wide: a terminal widens a text-default base followed by
// U+FE0F to two cells. narrow: it keeps the base's one cell (and draws the
// glyph wide, which breaks the alignment).
var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

var wideSymbols = map[rune]bool{
	0x2693: true, 0x26A1: true, 0x26AA: true, 0x26D4: true, 0x2705: true, 0x274C: true, 0x2754: true,
}

var textDefault = map[rune]bool{ // narrow by default, widened by U+FE0F
	0x2699: true, 0x1F6E0: true, 0x1F5BC: true,
}

func termWidth(s string, widenVS16 bool) int {
	s = ansiRE.ReplaceAllString(s, "")
	rs := []rune(s)
	w := 0
	for i, r := range rs {
		switch {
		case r == 0xFE0F || r == 0x200D:
		case textDefault[r]:
			w++
			if widenVS16 && i+1 < len(rs) && rs[i+1] == 0xFE0F {
				w++
			}
		case wideSymbols[r], r >= 0x1F300 && r <= 0x1FAFF:
			w += 2
		default:
			w++
		}
	}
	return w
}

// tableLines are the header, rule and body lines, which must share one width.
func tableLines(m Model) []string {
	lines := strings.Split(m.View(), "\n")
	return lines[:len(lines)-3]
}

func checkAligned(t *testing.T, name string, m Model, widen bool) {
	t.Helper()
	lines := tableLines(m)
	want := termWidth(lines[0], widen)
	for i, l := range lines {
		if got := termWidth(l, widen); got != want {
			t.Errorf("%s: line %d is %d cells, header is %d: %q", name, i, got, want, l)
		}
		if got := termWidth(l, widen); got > m.width {
			t.Errorf("%s: line %d is %d cells, wider than the %d-cell window", name, i, got, m.width)
		}
	}
	if n := strings.Count(m.View(), "\n") + 1; n != m.height {
		t.Errorf("%s: view is %d lines, window is %d", name, n, m.height)
	}
}

func TestAlignmentWhereTheSelectorWidens(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor) // the selected row carries ANSI
	defer lipgloss.SetColorProfile(termenv.Ascii)
	sequences := [][]string{
		nil,
		{"down", "down", "down", "down", "down"},
		{"down", "left"},
		{"c", "down"},
		{"4", "8", "w"},
		{"w", "w", "end"},
		{"c", "e", "pgdown", "pgup"},
	}
	for _, w := range []int{120, 100, 80, 64, 50} {
		for _, h := range []int{9, 12, 24} {
			for si, seq := range sequences {
				m := size(NewModel(load(t), ""), w, h)
				m = press(m, seq...)
				checkAligned(t, fmt.Sprintf("%dx%d seq %d", w, h, si), m, true)
			}
		}
	}
}

func TestStripMakesEveryTerminalAgree(t *testing.T) {
	m := size(NewModel(load(t), ""), 100, 12)
	m.StripVS16 = true
	if strings.Contains(m.View(), "\uFE0F") {
		t.Fatal("selector survives strip")
	}
	checkAligned(t, "strip wide", m, true)
	checkAligned(t, "strip narrow", m, false)
	for _, k := range []string{"w", "down", "left", "4"} {
		m = press(m, k)
		checkAligned(t, "strip narrow after "+k, m, false)
	}
}

// Finding: Lip Gloss counts a narrow base plus U+FE0F as two cells, so the
// grid assumes a terminal that widens it; an older terminal misaligns.
func TestWithoutStripANarrowTerminalMisaligns(t *testing.T) {
	m := size(NewModel(load(t), ""), 100, 12)
	lines := tableLines(m)
	if termWidth(lines[0], false) == termWidth(lines[1], false) {
		t.Error("expected the header, which holds two selector symbols, to measure short on a narrow terminal")
	}
	if lipgloss.Width("⚙️") != 2 {
		t.Errorf("lipgloss.Width of a selector symbol is %d, the grid assumes 2", lipgloss.Width("⚙️"))
	}
}

func TestDeepTreeStaysAligned(t *testing.T) {
	d := load(t, files[0])[0]
	var rows []Row
	for i := 0; i < 12; i++ {
		r := d.Rows[i%len(d.Rows)]
		r.ID = fmt.Sprintf("d%03d", i)
		r.Depth = i % 7
		r.Parent = i%7 < 6
		r.Title = "A task with a long title to force truncation " + r.Title
		rows = append(rows, r)
	}
	d.Rows = rows
	for _, w := range []int{100, 70, 50} {
		m := size(NewModel([]Data{d}, ""), w, 10)
		for i := 0; i < 12; i++ {
			m = press(m, "down")
			checkAligned(t, fmt.Sprintf("deep %d step %d", w, i), m, true)
		}
	}
}

func TestExpandAndCollapse(t *testing.T) {
	m := size(NewModel(load(t), ""), 100, 14)
	if got := strings.Join(ids(m), " "); got != "a1c0 4e2b 9f31 c07d 7b2e 3c5d" {
		t.Fatalf("opening rows %s", got)
	}
	m = press(m, "down", "left") // collapse 4e2b
	if got := strings.Join(ids(m), " "); got != "a1c0 4e2b 7b2e 3c5d" {
		t.Fatalf("after collapse %s", got)
	}
	if !strings.Contains(m.View(), "▸ Sensor node (2)") {
		t.Errorf("collapsed row lacks the hidden count:\n%s", m.View())
	}
	m = press(m, "right")
	if len(ids(m)) != 6 || m.selected != "4e2b" {
		t.Fatalf("after expand: %v sel %s", ids(m), m.selected)
	}
	m = press(m, "right") // into the first child
	if m.selected != "9f31" {
		t.Fatalf("right on an expanded parent selects %s", m.selected)
	}
	m = press(m, "left") // a leaf: move to its parent
	if m.selected != "4e2b" {
		t.Fatalf("left on a leaf selects %s", m.selected)
	}
	m = press(m, "up", "enter") // collapse the root
	if got := strings.Join(ids(m), " "); got != "a1c0" {
		t.Fatalf("root collapsed: %s", got)
	}
	m = press(m, "e")
	if len(ids(m)) != 6 {
		t.Fatal("e does not expand all")
	}
	m = press(m, "c")
	if got := strings.Join(ids(m), " "); got != "a1c0 4e2b 7b2e 3c5d" {
		t.Fatalf("glance: %s", got)
	}
}

func TestScrollInAFixedWindow(t *testing.T) {
	m := size(NewModel(load(t), ""), 100, chromeLines+3) // three body rows of six
	bh := m.bodyHeight()
	if bh != 3 {
		t.Fatalf("body height %d", bh)
	}
	if !strings.Contains(m.View(), "a1c0") || strings.Contains(m.View(), "c07d") {
		t.Fatalf("opening window:\n%s", m.View())
	}
	for i := 1; i <= 5; i++ {
		m = press(m, "down")
		vis := m.visible()
		p := m.selIndex(vis)
		if p != i || p < m.top || p >= m.top+bh {
			t.Fatalf("step %d: selection %d outside %d..%d", i, p, m.top, m.top+bh)
		}
		if strings.Count(m.View(), "\n")+1 != m.height {
			t.Fatalf("step %d: height changed", i)
		}
	}
	if m.top != 3 || !strings.Contains(m.View(), "3c5d") || strings.Contains(m.View(), "a1c0") {
		t.Fatalf("scrolled to the end:\n%s", m.View())
	}
	m = press(m, "home")
	if m.top != 0 || m.selected != "a1c0" {
		t.Fatalf("home: top %d sel %s", m.top, m.selected)
	}
	m = press(m, "pgdown")
	if m.selected != "c07d" {
		t.Fatalf("pgdown selects %s", m.selected)
	}
	m = press(m, "end")
	if m.selected != "3c5d" || m.top != 3 {
		t.Fatalf("end: top %d sel %s", m.top, m.selected)
	}
	// A shorter window keeps the selection in view and the lines whole.
	m = size(m, 100, chromeLines+2)
	if p := m.selIndex(m.visible()); p < m.top || p >= m.top+2 {
		t.Fatalf("resize lost the selection: %d in %d..%d", p, m.top, m.top+2)
	}
}

func TestGateWindowFoldsToACount(t *testing.T) {
	m := size(NewModel(load(t), ""), 100, 14)
	v := m.View()
	if !strings.Contains(v, "0›") || strings.Contains(v, "‹") {
		t.Errorf("default window: want only a count after:\n%s", v)
	}
	if !strings.Contains(v, "folded after integrate..release 0") {
		t.Errorf("status line lacks the fold:\n%s", v)
	}
	if got := len(m.shown()); got != 9 {
		t.Errorf("window 1 shows %d columns", got)
	}
	m = press(m, "w")
	v = m.View()
	if !strings.Contains(v, "‹1") || !strings.Contains(v, "0›") {
		t.Errorf("window 0: want counts before and after:\n%s", v)
	}
	if !strings.Contains(v, "folded before undefined..undefined 1") {
		t.Errorf("status line lacks the fold before:\n%s", v)
	}
	if got := len(m.shown()); got != 7 {
		t.Errorf("window 0 shows %d columns", got)
	}
	if m.selected != "a1c0" {
		t.Errorf("selection moved to %s", m.selected)
	}
}

func TestHideAndShowAColumnKeepsTheChoice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "settings.json")
	m := size(NewModel(load(t), path), 100, 14)
	if !strings.Contains(m.View(), "⚙️") {
		t.Fatal("function column missing")
	}
	m = press(m, "4") // function
	v := m.View()
	if strings.Contains(strings.Split(v, "\n")[0], "⚙") || !strings.Contains(v, "(4⚙️)") {
		t.Errorf("function column still shown or not marked hidden:\n%s", v)
	}
	if !strings.Contains(strings.Split(v, "\n")[0], "⊘1") {
		t.Errorf("hidden column lacks its count (9f31 stands at function):\n%s", v)
	}
	if strings.Contains(v, "🔴⛔") {
		t.Errorf("the state cell of a hidden column shows:\n%s", v)
	}
	b, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(b), `"function"`) {
		t.Fatalf("settings file: %q %v", b, err)
	}
	// A new session reads the choice back, also in another window variant.
	m2 := size(NewModel(load(t), path), 100, 14)
	if m2.View() != v {
		t.Errorf("reloaded view differs:\n%s\nwant\n%s", m2.View(), v)
	}
	m2 = press(m2, "w")
	if strings.Contains(strings.Split(m2.View(), "\n")[0], "⚙") {
		t.Error("choice lost across the window variant")
	}
	m2 = press(m2, "3") // show function again; window 0 has no undefined column, so the digit shifts
	if b, _ := os.ReadFile(path); strings.Contains(string(b), "function") {
		t.Errorf("settings still name function: %s", b)
	}
	m2 = press(m2, "1", "2", "0")
	if len(m2.shown()) != len(m2.data().Columns) {
		t.Error("0 does not show every column")
	}
	checkAligned(t, "hidden", press(m, "5", "7"), true)
}

func TestSettingsFaults(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := size(NewModel(load(t), bad), 100, 14)
	if !strings.Contains(m.View(), "settings:") {
		t.Errorf("malformed settings not reported:\n%s", m.View())
	}
	// A directory where the file should be: the save fails and the model reports it.
	m = size(NewModel(load(t), dir), 100, 14)
	m = press(m, "4")
	if !strings.Contains(m.View(), "settings:") {
		t.Errorf("save failure not reported")
	}
}

func TestMarksAndSymbolsComeFromTheData(t *testing.T) {
	m := size(NewModel(load(t), ""), 120, 14)
	v := m.View()
	for _, s := range []string{"🟢", "🔴⛔", "🧑👀", "🤖👀", "🪆", "—", "⚪", "⚙️", "🛠️"} {
		if !strings.Contains(v, s) {
			t.Errorf("missing %s:\n%s", s, v)
		}
	}
}
