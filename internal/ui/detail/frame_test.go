package detail

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/ui/grid"
	"github.com/nbyoung/tablotui/internal/view/list"
)

// T3: every line fits.
func TestEveryLineFits(t *testing.T) {
	sizes := [][2]int{
		{40, 8}, {80, 24}, {100, 30}, {119, 20}, {120, 12}, {120, 24}, {160, 30}, {170, 46},
		{60, 12}, {79, 11}, {99, 12}, {121, 11}, {200, 8},
	}
	for w := 40; w < 44; w++ {
		for h := 8; h < 12; h++ {
			sizes = append(sizes, [2]int{w, h})
		}
	}
	scripts := []string{
		"1",
		"1 2 3 4",
		"E down down 1 2 3 4 z",
		"2 d down down enter",
		"3 p end",
		"4 pgup up home G",
	}
	for _, grapheme := range []bool{false, true} {
		method := ansi.WcWidth
		if grapheme {
			method = ansi.GraphemeWidth
		}
		m := ui.Measure{Method: method}
		for _, sz := range sizes {
			for _, script := range scripts {
				r := newRig(t, "testdata", sz[0], sz[1])
				if grapheme {
					r.grapheme()
				}
				r.keys(script)
				lines := r.content()
				tag := fmt.Sprintf("%dx%d, grapheme %v, keys %q", sz[0], sz[1], grapheme, script)
				if len(lines) != sz[1] {
					t.Fatalf("%s: %d lines", tag, len(lines))
				}
				for i, l := range lines {
					if got := m.Width(ansi.Strip(l)); got != sz[0] {
						t.Fatalf("%s: line %d is %d cells wide: %q", tag, i, got, ansi.Strip(l))
					}
				}
			}
		}
	}
}

// T3, for the pane alone: View returns h lines of w cells at any size.
func TestPaneViewFitsItsRectangle(t *testing.T) {
	for _, id := range allPanes {
		for _, grapheme := range []bool{false, true} {
			method := ansi.WcWidth
			if grapheme {
				method = ansi.GraphemeWidth
			}
			x := pane(t, id, "testdata", 80, 20).open().sel("e9c6", "")
			x.c.Measure = ui.Measure{Method: method}
			for w := 1; w <= 90; w += 3 {
				for _, h := range []int{1, 2, 5, 12, 40} {
					lines := strings.Split(x.p.View(w, h, x.c), "\n")
					if len(lines) != h {
						t.Fatalf("%s %dx%d: %d lines", id, w, h, len(lines))
					}
					for i, l := range lines {
						if got := x.c.Measure.Width(ansi.Strip(l)); got != w {
							t.Fatalf("%s %dx%d: line %d is %d cells wide: %q", id, w, h, i, got, l)
						}
					}
				}
			}
		}
	}
}

// T12: 1 to 4 and z.
func TestDigitsOpenFocusAndClosePanes(t *testing.T) {
	r := newRig(t, "testdata", 100, 30).keys("E down down down down")
	if strings.Contains(r.screen(), "┌─") {
		t.Fatal("a pane stands open on arrival")
	}
	focused := func() string { return strings.Fields(r.status())[0] }
	r.keys("1")
	if !strings.Contains(r.screen(), "┌─ 1 Task e9c6 · detail") || focused() != "task" {
		t.Errorf("1 did not open the task pane with the focus: %q\n%s", r.status(), r.screen())
	}
	r.keys("esc")
	if !strings.Contains(r.screen(), "┌─ 1 Task e9c6") || focused() == "task" {
		t.Errorf("esc did not return the focus to the grid with the pane open: %q", r.status())
	}
	// The pane follows the grid while it has no focus: it asks for the next
	// task, whose fixture the directory lacks.
	r.keys("down")
	if !strings.Contains(r.screen(), "task-bc86.json") {
		t.Errorf("the pane does not follow:\n%s", r.screen())
	}
	r.keys("1")
	if focused() != "task" || !strings.Contains(r.screen(), "┌─ 1 Task") {
		t.Errorf("1 did not return to the open pane: %q", r.status())
	}
	r.keys("1")
	if strings.Contains(r.screen(), "┌─") || focused() == "task" {
		t.Errorf("1 did not close the focused pane: %q\n%s", r.status(), r.screen())
	}
	// Each digit opens its pane; a pane without the focus takes it on the digit.
	r.keys("2 4 1")
	for _, want := range []string{"1 Task", "2 Queue", "4 History"} {
		if !strings.Contains(r.screen(), want) {
			t.Errorf("%q is not open:\n%s", want, r.screen())
		}
	}
	r.keys("2")
	if focused() != "queue" {
		t.Errorf("2 did not take the focus: %q", r.status())
	}
	// tab cycles the open panes in the order they opened.
	r.keys("tab")
	if focused() != "history" {
		t.Errorf("tab went to %q", r.status())
	}
	// The two other panes are untouched.
	if strings.Contains(r.screen(), "3 Blockage") {
		t.Error("the blockage pane opened")
	}
}

func TestZoom(t *testing.T) {
	r := newRig(t, "testdata", 100, 30).keys("3")
	r.keys("z")
	if got := r.lastNotice(); got.Text != "zoom on: the focused pane takes the body" || got.Err {
		t.Errorf("notice = %+v", got)
	}
	scr := r.screen()
	if strings.Contains(scr, "Id   Task") || !strings.HasPrefix(strings.Split(scr, "\n")[1], "┌─ 3 Blockage") {
		t.Errorf("zoom did not give the pane the body:\n%s", scr)
	}
	if got := len(strings.Split(scr, "\n")); got != 30 {
		t.Errorf("%d lines", got)
	}
	// With the focus on the grid, the grid takes the body.
	r.keys("esc")
	if scr := r.screen(); !strings.Contains(scr, "Id   Task") || strings.Contains(scr, "┌─") {
		t.Errorf("zoom with the grid focused:\n%s", scr)
	}
	r.keys("z")
	if got := r.lastNotice(); got.Text != "zoom off" {
		t.Errorf("notice = %+v", got)
	}
	if scr := r.screen(); !strings.Contains(scr, "Id   Task") || !strings.Contains(scr, "┌─ 3 Blockage") {
		t.Errorf("zoom off did not return the split:\n%s", scr)
	}
	// Two frames share nothing.
	other := newRig(t, "testdata", 100, 30).keys("3")
	if !strings.Contains(other.screen(), "Id   Task") {
		t.Error("the zoom of one set reached another")
	}
}

func TestCommandsAndKeysDoNotClash(t *testing.T) {
	set := New(Options{})
	g := grid.New(grid.Options{Source: source.File{Dir: "../grid/testdata"}, Start: grid.GlanceStart()})
	cmds := set.Commands
	for _, k := range []string{"R", "r", "v", "a", "u"} {
		cmds = append(cmds, ui.Command{Binding: key.NewBinding(key.WithKeys(k), key.WithHelp(k, "stub"))})
	}
	o := ui.Options{Panes: append([]ui.Pane{g}, set.Panes...), Commands: cmds, Layout: set.Layout}
	if err := ui.CheckKeys(o); err != nil {
		t.Errorf("CheckKeys: %v", err)
	}
	if len(set.Panes) != 4 || len(set.Commands) != 5 || set.Layout == nil {
		t.Errorf("New returns %d panes and %d commands", len(set.Panes), len(set.Commands))
	}
	var ids, keys []string
	for _, p := range set.Panes {
		ids = append(ids, p.ID())
	}
	for _, c := range set.Commands {
		keys = append(keys, strings.Join(c.Binding.Keys(), "")+" "+c.Binding.Help().Desc)
	}
	if got := strings.Join(ids, " "); got != "task queue blockage history" {
		t.Errorf("panes = %s", got)
	}
	if got := strings.Join(keys, "; "); got != "1 task pane; 2 queue pane; 3 blockage pane; 4 history pane; z zoom" {
		t.Errorf("commands = %s", got)
	}
	// The panes are closed: they ask nothing when the frame starts.
	r := newRig(t, "testdata", 100, 30)
	if r.src.count() != 0 {
		t.Errorf("the panes asked %d times on arrival", r.src.count())
	}
	// The commands take no key of f394 or 518e, and the help mode lists them.
	r.keys("?")
	for _, want := range []string{"task pane", "queue pane", "blockage pane", "history pane", "zoom"} {
		if !strings.Contains(r.screen(), want) {
			t.Errorf("the help mode lacks %q:\n%s", want, r.screen())
		}
	}
}

// T14: enter and GotoMsg.
func TestEnterSelectsTheTaskInTheGrid(t *testing.T) {
	r := newRig(t, "testdata", 120, 24).keys("2 d down down down enter")
	if got, want := r.lastSelected(), (ui.SelectionMsg{Task: "e9c6", Gate: "implementation"}); got != want {
		t.Fatalf("selection = %+v, want %+v", got, want)
	}
	scr := r.screen()
	for _, want := range []string{"▾ Method", "▾ Views", "e9c6         Abstract views"} {
		if !strings.Contains(scr, want) {
			t.Errorf("the grid did not unfold to e9c6; %q is missing:\n%s", want, scr)
		}
	}
	if !strings.HasPrefix(r.status(), "queue ") {
		t.Errorf("the focus left the pane: %q", r.status())
	}
	// The other panes follow the selection, and the queue marks it.
	r.keys("3")
	if !strings.Contains(r.screen(), "3 Blockage e9c6") {
		t.Errorf("a pane opened afterwards does not follow:\n%s", r.screen())
	}
	// Walking the queue with down and enter moves the grid.
	r.keys("2 down down down enter")
	if got := r.lastSelected().Task; got != "99f0" {
		t.Errorf("the walk selects %q", got)
	}
}

func TestEnterOnAGateInAFoldedColumnLeavesTheColumnCursor(t *testing.T) {
	r := newRig(t, "testdata", 120, 24).keys("1")
	// Find the row of the junction of the undefined gate in the task pane.
	var d list.Task
	decode(t, "task-e9c6.json", &d)
	_, _, lines := taskLines(d, list.Detail, false)
	rows := wrap(lines, 38, ui.Measure{})
	at := -1
	for i, row := range rows {
		if l := lines[row.line]; l.Gate == "undefined" && row.sub == 0 {
			at = i
		}
	}
	if at < 0 {
		t.Fatal("no junction for the undefined gate")
	}
	// The grid's selection stands on 437e; the pane follows it, so select e9c6 first.
	r.send(ui.GotoMsg{Task: "e9c6", Gate: "implementation"})
	r.keys("1")
	r.keys("1") // the focus back to the pane
	r.keys(strings.Repeat("down ", at) + "enter")
	want := ui.SelectionMsg{Task: "e9c6", Gate: "implementation"}
	if got := r.lastSelected(); got != want {
		t.Errorf("selection = %+v, want %+v", got, want)
	}
	if got := r.status(); !strings.Contains(got, "enter selects e9c6 × undefined") {
		t.Errorf("status = %q", got)
	}
}

func TestEnterOnALineWithNoTask(t *testing.T) {
	r := newRig(t, "testdata", 120, 24).keys("2 enter")
	if got := r.lastNotice(); got != (ui.NoticeMsg{Text: "this line names no task"}) {
		t.Errorf("notice = %+v", got)
	}
	if got := r.lastSelected().Task; got != "437e" {
		t.Errorf("the selection moved to %q", got)
	}
}

func TestEnterOnATaskTheTableauLacks(t *testing.T) {
	r := newRig(t, "testdata", 120, 24)
	r.send(ui.GotoMsg{Task: "zzzz", Gate: "design"})
	if got := r.lastNotice(); got != (ui.NoticeMsg{Text: "zzzz is not in the tableau in view"}) {
		t.Errorf("notice = %+v", got)
	}
	if got := r.lastSelected().Task; got != "437e" {
		t.Errorf("the selection moved to %q", got)
	}
}

// T20: determinism.
func TestTheSameKeysGiveTheSameBytes(t *testing.T) {
	for _, script := range []string{"E down down down down 1 2 3 4", "2 d down down enter", "3 z p"} {
		a := newRig(t, "testdata", 120, 40).keys(script).m.(ui.Model).View().Content
		b := newRig(t, "testdata", 120, 40).keys(script).m.(ui.Model).View().Content
		if a != b {
			t.Errorf("keys %q: two fresh sets draw different bytes", script)
		}
		if !strings.Contains(a, "\x1b[") {
			t.Errorf("keys %q: the view carries no style", script)
		}
	}
}
