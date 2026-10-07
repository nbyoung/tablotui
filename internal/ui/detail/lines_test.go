package detail

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view/list"
)

var methods = []struct {
	name string
	m    ui.Measure
}{
	{"wcwidth", ui.Measure{Method: ansi.WcWidth}},
	{"grapheme", ui.Measure{Method: ansi.GraphemeWidth}},
}

// T5: the symbols the panes add measure one cell under both methods.
func TestNewSymbolsMeasureOneCell(t *testing.T) {
	for _, me := range methods {
		for _, s := range []string{"┌", "│", "─", "›", "→", "–", "…", "·", ">"} {
			if got := me.m.Width(s); got != 1 {
				t.Errorf("%s: %q measures %d cells", me.name, s, got)
			}
		}
	}
}

// T6: the wrap.
func TestWrapLine(t *testing.T) {
	m := ui.Measure{}
	cases := []struct {
		name string
		l    Line
		w    int
		want []string
	}{
		{"a label and a hang", Line{Label: "note", Text: "alpha beta gamma delta", Hang: 8}, 20,
			[]string{"note    alpha beta", "        gamma delta"}},
		{"an indent and a hang", Line{Text: "aaa bbb ccc ddd", Indent: 2, Hang: 2}, 10,
			[]string{"  aaa bbb", "    ccc", "    ddd"}},
		{"the indent takes a quarter and the hang a half", Line{Text: "abcdefghij klmnopqrstu", Indent: 30, Hang: 30}, 20,
			[]string{"     abcdefghij", "               klmno", "               pqrst", "               u"}},
		{"a word wider than the row", Line{Text: "x abcdefghijklmnop y"}, 8,
			[]string{"x", "abcdefgh", "ijklmnop", "y"}},
		{"a hash in a narrow pane", Line{Text: "Commit: 879b4471c0ffee00d15ea5e0fabc0de0feedbeef", Indent: 2, Hang: 2}, 38,
			[]string{"  Commit:", "    879b4471c0ffee00d15ea5e0fabc0de0fe", "    edbeef"}},
		{"an empty text", Line{}, 10, []string{""}},
		{"an empty text with an indent", Line{Indent: 2}, 10, []string{""}},
		{"a label alone", Line{Label: "ab", Hang: 4}, 10, []string{"ab"}},
		{"a zero width", Line{Text: "abc"}, 0, []string{""}},
		{"a symbol that does not fit beside a word", Line{Text: "ab 🟢"}, 4, []string{"ab", "🟢"}},
		{"a symbol at the edge", Line{Text: "ab 🟢"}, 5, []string{"ab 🟢"}},
		{"runs of spaces", Line{Text: "a    b\n\tc"}, 20, []string{"a b c"}},
		{"marked", Line{Text: "alpha", Indent: 2, Marked: true}, 20, []string{"> alpha"}},
		{"marked with no indent", Line{Text: "alpha", Marked: true}, 20, []string{"alpha"}},
		{"marked with a label", Line{Label: "id", Text: "x", Indent: 2, Hang: 4, Marked: true}, 20, []string{"> id  x"}},
		{"marked and wrapped", Line{Text: "alpha beta", Indent: 2, Hang: 2, Marked: true}, 8, []string{"> alpha", "    beta"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := wrapLine(c.l, c.w, m)
			if fmt.Sprintf("%q", got) != fmt.Sprintf("%q", c.want) {
				t.Errorf("wrapLine = %q, want %q", got, c.want)
			}
			for i, r := range got {
				if w := m.Width(r); c.w > 0 && w > c.w && !strings.Contains(c.name, "symbol") {
					t.Errorf("row %d is %d cells wide in %d", i, w, c.w)
				}
			}
		})
	}
}

func TestWrapKeepsRowsOfItsLines(t *testing.T) {
	rows := wrap([]Line{{Text: "one two three", Hang: 2}, {Item: -1}, {Text: "four"}}, 8, ui.Measure{})
	var got []string
	for _, r := range rows {
		got = append(got, fmt.Sprintf("%d.%d:%s", r.line, r.sub, r.text))
	}
	want := "[0.0:one two 0.1:  three 1.0: 2.0:four]"
	if s := fmt.Sprint(got); s != want {
		t.Errorf("rows = %s, want %s", s, want)
	}
}

// T7: the run fold.
func alike(n int, rest string) []entry {
	var es []entry
	for i := range n {
		es = append(es, entry{task: list.TaskRef{ID: fmt.Sprintf("t%03d", i), Title: "Task"}, rest: rest, item: i})
	}
	return es
}

func TestFoldRuns(t *testing.T) {
	if got := foldRuns(alike(8, "a"), false); len(got) != 8 {
		t.Errorf("eight alike fold to %d entries", len(got))
	}
	got := foldRuns(alike(9, "a"), false)
	if len(got) != 4 || got[2].task.ID != "t002" {
		t.Fatalf("nine alike fold to %d entries: %+v", len(got), got)
	}
	f := got[3]
	if f.task.ID != "" || f.task.Title != "… and 6 more: t003 t004 t005 t006 t007 t008" || f.rest != "a" || f.item != 3 {
		t.Errorf("the fold line = %+v", f)
	}
	if !strings.HasPrefix(f.first(), "… and 6 more") || len(f.ids) != 6 {
		t.Errorf("the fold line reads %q with %d ids", f.first(), len(f.ids))
	}
	// A run that another entry breaks stays whole.
	mixed := append(append(alike(4, "a"), entry{task: list.TaskRef{ID: "odd"}, rest: "b"}), alike(4, "a")...)
	if got := foldRuns(mixed, false); len(got) != 9 {
		t.Errorf("nine with one that differs fold to %d entries", len(got))
	}
	// Two runs fold on their own.
	two := append(alike(10, "a"), alike(12, "b")...)
	if got := foldRuns(two, false); len(got) != 8 {
		t.Errorf("two runs fold to %d entries", len(got))
	}
	// Unfolded, the entries stand as they came.
	if got := foldRuns(alike(30, "a"), true); len(got) != 30 {
		t.Errorf("unfolded entries number %d", len(got))
	}
}

func TestFoldLineCarriesTheMarkWhenItsIdsHoldTheSelection(t *testing.T) {
	var q list.Queue
	q.Level = list.Glance
	for i := range 9 {
		q.Items = append(q.Items, list.QueueItem{
			Kind: "ready", Gate: "design", Since: "2026-09-30",
			Task: list.TaskRef{ID: fmt.Sprintf("t%03d", i), Title: "Task"},
		})
	}
	text := func(sel string, unfold bool) []string {
		_, _, lines := queueLines(q, list.Glance, unfold, sel)
		var out []string
		for _, r := range wrap(lines, 60, ui.Measure{}) {
			out = append(out, r.text)
		}
		return out
	}
	has := func(rows []string, prefix string) bool {
		for _, r := range rows {
			if strings.HasPrefix(r, prefix) {
				return true
			}
		}
		return false
	}
	if rows := text("t005", false); !has(rows, "> … and 6 more: t003 t004") || has(rows, "  … and 6 more") {
		t.Errorf("the fold line carries no mark:\n%s", strings.Join(rows, "\n"))
	}
	if rows := text("t001", false); !has(rows, "> t001") || !has(rows, "  … and 6 more") {
		t.Errorf("an entry shown, or a fold line marked in error:\n%s", strings.Join(rows, "\n"))
	}
	rows := text("t005", true)
	marks := 0
	for _, r := range rows {
		if strings.HasPrefix(r, ">") {
			marks++
		}
	}
	if !has(rows, "> t005") || marks != 1 {
		t.Errorf("an unfolded queue marks one entry:\n%s", strings.Join(rows, "\n"))
	}
	for _, r := range text("", false) {
		if strings.HasPrefix(r, ">") {
			t.Errorf("no selection marks %q", r)
		}
	}
}

// decode reads a fixture of testdata into v.
func decode(t *testing.T, name string, v any) {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := list.Decode(f, v); err != nil {
		t.Fatal(err)
	}
}
