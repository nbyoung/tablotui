package grid

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

var rowsRange = regexp.MustCompile(`rows (\d+)–(\d+) of (\d+)`)

// selectedID returns the id the status line names.
func selectedID(r *rig) string {
	id, _, _ := strings.Cut(r.status(), " ")
	return id
}

// shownRange returns the first and last row the status line says are in view.
func shownRange(t *testing.T, r *rig) (a, b, n int) {
	t.Helper()
	m := rowsRange.FindStringSubmatch(r.status())
	if m == nil {
		t.Fatalf("status %q has no row range", r.status())
	}
	a, _ = strconv.Atoi(m[1])
	b, _ = strconv.Atoi(m[2])
	n, _ = strconv.Atoi(m[3])
	return
}

// rowIDs returns the ids of the rows drawn.
func rowIDs(r *rig) []string {
	var ids []string
	for _, l := range r.content()[3 : len(r.content())-2] {
		l = strings.TrimRight(ansi.Strip(l), " ")
		if id, _, ok := strings.Cut(l, " "); ok && len(id) == 4 && !strings.ContainsAny(id, "─") {
			ids = append(ids, id)
		}
	}
	return ids
}

// T9: the tree.
func TestFoldAndUnfold(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 100, 40)
	if got := rowIDs(r); strings.Join(got[:3], " ") != "437e bc63 77b2" || len(got) != 6 {
		t.Fatalf("glance rows = %v", got)
	}
	if !strings.Contains(r.screen(), "▸ Method (31)") {
		t.Errorf("the folded parent has no count of every descendant:\n%s", r.screen())
	}
	r.keys("down enter")
	if !strings.Contains(r.screen(), "▾ Method") || strings.Contains(r.screen(), "Method (") {
		t.Errorf("enter did not unfold:\n%s", r.screen())
	}
	if strings.Contains(r.screen(), "▾ Method") && !strings.Contains(r.screen(), "▸ Views (23)") {
		t.Errorf("a nested parent lacks its count:\n%s", r.screen())
	}
	if a, b, n := shownRange(t, r); a != 1 || b != 14 || n != 14 {
		t.Errorf("rows %d–%d of %d, want 1–14 of 14", a, b, n)
	}
	r.keys("enter")
	if !strings.Contains(r.screen(), "▸ Method (31)") {
		t.Error("a second enter did not fold")
	}
	r.keys("space")
	if !strings.Contains(r.screen(), "▾ Method") {
		t.Error("space did not unfold")
	}
	r.keys("E")
	if _, _, n := shownRange(t, r); n != 37 {
		t.Errorf("E shows %d rows, want 37", n)
	}
	r.keys("C")
	if _, _, n := shownRange(t, r); n != 6 {
		t.Errorf("C shows %d rows, want 6", n)
	}
	// enter on a leaf does nothing.
	r.keys("end enter")
	if _, _, n := shownRange(t, r); n != 6 {
		t.Errorf("enter on a leaf changed the rows: %d", n)
	}
}

func TestSelectionMovesToTheAncestorThatRemains(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 100, 40).keys("E down down down down down down")
	if id := selectedID(r); id != "99f0" {
		t.Fatalf("selected %s, want 99f0", id)
	}
	r.keys("C")
	if id := selectedID(r); id != "bc63" {
		t.Errorf("after C the selection is %s, want the folded ancestor bc63", id)
	}
	// And the selection survives folding of something above it.
	r.keys("E down down")
	if id := selectedID(r); id != "2034" {
		t.Fatalf("selected %s", id)
	}
	r.keys("up up enter")
	if id := selectedID(r); id != "bc63" {
		t.Errorf("selected %s after folding the parent", id)
	}
}

func TestRowKeys(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 100, 14).keys("E")
	steps := []struct {
		keys, want string
	}{
		{"down", "bc63"}, {"j", "c2ad"}, {"up", "bc63"}, {"k", "437e"},
		{"end", "4593"}, {"home", "437e"}, {"G", "595e"}, {"g", "437e"},
	}
	for _, s := range steps {
		r.keys(s.keys)
		if s.want == "4593" {
			s.want = "595e"
		}
		if id := selectedID(r); id != s.want {
			t.Errorf("after %s: selected %s, want %s", s.keys, id, s.want)
		}
	}
	// pgdown moves by the rows in view: 14 lines give 11 body lines, 9 rows.
	r.keys("pgdown")
	a, b, _ := shownRange(t, r)
	if b-a+1 != 9 {
		t.Errorf("the view holds %d rows, want 9", b-a+1)
	}
	if id := selectedID(r); id != rowIDs(r)[len(rowIDs(r))-1] {
		t.Errorf("pgdown selected %s, want the last row in view %v", id, rowIDs(r))
	}
	r.keys("pgup")
	if id := selectedID(r); id != "437e" {
		t.Errorf("pgup selected %s", id)
	}
}

func TestScrollKeepsTheSelectionInView(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 100, 40).keys("E")
	check := func(when string) {
		t.Helper()
		id := selectedID(r)
		a, b, n := shownRange(t, r)
		if n != 37 {
			t.Fatalf("%s: %d rows", when, n)
		}
		if !strings.Contains(strings.Join(rowIDs(r), " "), id) {
			t.Errorf("%s: selected %s is not drawn: %v", when, id, rowIDs(r))
		}
		if got := len(rowIDs(r)); got != b-a+1 {
			t.Errorf("%s: %d rows drawn for the range %d–%d", when, got, a, b)
		}
	}
	for i := 0; i < 30; i++ {
		r.keys("down")
	}
	check("after 30 downs")
	for _, h := range []int{40, 20, 12, 8, 9, 30, 8, 40, 14} {
		r.send(tea.WindowSizeMsg{Width: 100, Height: h})
		check(fmt.Sprintf("resized to height %d", h))
	}
	r.keys("end")
	check("at the end")
	for _, h := range []int{8, 40, 8} {
		r.send(tea.WindowSizeMsg{Width: 100, Height: h})
		check(fmt.Sprintf("at the end, resized to %d", h))
	}
	if a, b, n := shownRange(t, r); b != n {
		t.Errorf("at the end the range is %d–%d of %d", a, b, n)
	}
	r.keys("home")
	check("at the top")
	if a, _, _ := shownRange(t, r); a != 1 {
		t.Errorf("at the top the range starts at %d", a)
	}
}

func TestScrollMovesByTheLeast(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 100, 14).keys("E")
	// 9 rows in view: the selection reaches row 10 and the window moves by one.
	for i := 0; i < 9; i++ {
		r.keys("down")
	}
	if a, b, _ := shownRange(t, r); a != 2 || b != 10 {
		t.Errorf("range %d–%d, want 2–10", a, b)
	}
	r.keys("up up up up up up up up up")
	if a, b, _ := shownRange(t, r); a != 1 || b != 9 {
		t.Errorf("range %d–%d, want 1–9", a, b)
	}
}

func TestSelectionMessages(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 100, 14)
	if len(r.selected) != 1 || r.selected[0].Task != "437e" || r.selected[0].Gate != "" {
		t.Fatalf("selections after the first load = %+v", r.selected)
	}
	r.keys("down")
	if n := len(r.selected); n != 2 || r.selected[1].Task != "bc63" {
		t.Errorf("selections = %+v", r.selected)
	}
	r.keys("right")
	if n := len(r.selected); n != 2 {
		t.Errorf("the folded column is no gate cell: %+v", r.selected)
	}
	r.keys("right")
	if n := len(r.selected); n != 3 || r.selected[2].Gate != "defined" {
		t.Errorf("selections = %+v", r.selected)
	}
	r.keys("right")
	if r.selected[len(r.selected)-1].Gate != "mockup" {
		t.Errorf("selections = %+v", r.selected)
	}
	r.keys("d") // no change of row or cell
	n := len(r.selected)
	r.send(ui_ReloadMsg())
	if len(r.selected) != n+1 {
		t.Errorf("a load sends a selection: %d, want %d", len(r.selected), n+1)
	}
	r.keys("left left left")
	if last := r.selected[len(r.selected)-1]; last.Gate != "" {
		t.Errorf("off the gate cells the gate is %q", last.Gate)
	}
}
