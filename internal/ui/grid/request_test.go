package grid

import (
	"os"
	"strings"
	"testing"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view"
)

// T11: requests and loads.
func TestWindowAndHistoricalRequests(t *testing.T) {
	src := asFixture(t)
	r := newRig(t, src, "", GlanceStart(), 100, 20)
	if len(src.reqs) != 1 || src.reqs[0] != (source.Request{View: "tableau", Window: 1, Level: "detail"}) {
		t.Fatalf("first request = %+v", src.reqs)
	}
	r.keys("w")
	if want := (source.Request{View: "tableau", Window: 2, Level: "detail"}); src.last() != want || len(src.reqs) != 2 {
		t.Errorf("w asked %+v", src.reqs)
	}
	if !strings.Contains(r.status(), "window 2") {
		t.Errorf("status = %q", r.status())
	}
	r.keys("W W")
	if want := (source.Request{View: "tableau", Window: 0, Level: "detail"}); src.last() != want || len(src.reqs) != 4 {
		t.Errorf("W W asked %+v", src.reqs)
	}
	r.keys("H")
	if want := (source.Request{View: "tableau", Window: 0, Historical: true, Level: "detail"}); src.last() != want {
		t.Errorf("H asked %+v", src.last())
	}
	if !strings.Contains(r.status(), "historical on") {
		t.Errorf("status = %q", r.status())
	}
	r.keys("H")
	if src.last().Historical || !strings.Contains(r.status(), "historical off") {
		t.Errorf("second H asked %+v, status %q", src.last(), r.status())
	}
}

func TestWindowLimits(t *testing.T) {
	src := asFixture(t)
	r := newRig(t, src, "", GlanceStart(), 100, 20).keys("W")
	n := len(src.reqs)
	r.keys("W")
	if len(src.reqs) != n || !strings.Contains(r.status(), "window 0") {
		t.Errorf("W at window 0 asked again: %d requests; status %q", len(src.reqs), r.status())
	}
	// A window that shows every gate cannot widen.
	all := asFixture(t)
	base := all.fn
	all.fn = func(q source.Request) (view.Tableau, error) {
		tb, _ := base(q)
		tb.Gates = append([]view.Gate(nil), tb.Gates...)
		for i := range tb.Gates {
			tb.Gates[i].Window = true
		}
		return tb, nil
	}
	r = newRig(t, all, "", GlanceStart(), 100, 20).keys("w")
	if len(all.reqs) != 1 || !strings.Contains(r.status(), "every gate") {
		t.Errorf("w with every gate shown asked again: %d requests; status %q", len(all.reqs), r.status())
	}
}

func TestAnOlderSequenceChangesNothing(t *testing.T) {
	src := asFixture(t)
	r := newRig(t, src, "", GlanceStart(), 100, 20).keys("w")
	before := r.screen()
	other := load(t, "tableau-historical.json")
	other.Project = "Other"
	r.send(ui.LoadedMsg{Pane: ID, Seq: 1, Value: other})
	if r.screen() != before {
		t.Error("a LoadedMsg of an older Seq changed the screen")
	}
	r.send(ui.LoadedMsg{Pane: ID, Seq: 99, Value: other})
	if r.screen() != before {
		t.Error("a LoadedMsg of a newer Seq changed the screen")
	}
	r.send(ui.LoadedMsg{Pane: ID, Seq: 2, Value: other})
	if !strings.HasPrefix(r.line(0), "Other") {
		t.Errorf("the newest Seq did not apply: %q", r.line(0))
	}
}

func TestAFailedLoadKeepsTheView(t *testing.T) {
	src := asFixture(t)
	good := src.fn
	src.fn = func(q source.Request) (view.Tableau, error) {
		if q.Window == 3 {
			return view.Tableau{}, errBoom
		}
		return good(q)
	}
	r := newRig(t, src, "", GlanceStart(), 100, 20).keys("right right down")
	before := strings.Split(r.screen(), "\n")
	r.keys("w w")
	if n := len(r.notices); n != 1 || r.notices[0].Text != "the source failed (showing the last good view)" || !r.notices[0].Err {
		t.Fatalf("notices = %+v", r.notices)
	}
	if got := r.status(); got != "error: the source failed (showing the last good view)" {
		t.Errorf("status = %q", got)
	}
	after := strings.Split(r.screen(), "\n")
	for i := 1; i < len(before)-2; i++ {
		if before[i] != after[i] && !strings.Contains(before[i], "window") {
			t.Errorf("line %d changed:\n%s\n%s", i, before[i], after[i])
		}
	}
	// The next key shows the view's own status, and w asks for window 3 again.
	r.keys("w")
	if src.last().Window != 3 {
		t.Errorf("the failed window is not asked again: %+v", src.last())
	}
}

func TestAFailedFirstLoad(t *testing.T) {
	src := &recorder{fn: func(source.Request) (view.Tableau, error) { return view.Tableau{}, errBoom }}
	r := newRig(t, src, "", GlanceStart(), 100, 12)
	if got := r.line(1); got != "error: the source failed" {
		t.Errorf("body = %q", got)
	}
	r.keys("down x w H p E")
	if got := r.line(1); got == "" {
		t.Error("keys cleared the error")
	}
	// A later load recovers.
	src.fn = asFixture(t).fn
	r.send(ui.ReloadMsg{})
	if got := r.line(1); !strings.HasPrefix(got, "Id") {
		t.Errorf("after a good load, body = %q", got)
	}
}

func TestLoadingBeforeTheFirstAnswer(t *testing.T) {
	g := New(Options{Source: asFixture(t), Start: GlanceStart()})
	m := ui.New(ui.Options{Panes: []ui.Pane{g}, Styles: ui.DefaultStyles()})
	r := &rig{t: t, m: m}
	r.send(sizeMsg(60, 10))
	if got := r.line(1); got != "loading…" {
		t.Errorf("body = %q", got)
	}
	if got := r.status(); got != "" {
		t.Errorf("status = %q", got)
	}
	r.keys("down x s")
	if got := r.line(1); got != "loading…" {
		t.Errorf("body after keys = %q", got)
	}
}

func TestReloadRepeatsTheRequest(t *testing.T) {
	src := asFixture(t)
	r := newRig(t, src, "", GlanceStart(), 100, 20).keys("w H")
	n := len(src.reqs)
	want := src.last()
	r.send(ui.ReloadMsg{})
	if len(src.reqs) != n+1 || src.last() != want {
		t.Errorf("requests = %+v, want %+v again", src.reqs[n-1:], want)
	}
}

func TestStateSurvivesAReload(t *testing.T) {
	path := settingsFile(t, "")
	src := asFixture(t)
	r := newRig(t, src, path, GlanceStart(), 100, 20)
	r.keys("E down down down right right right right x down down")
	before := r.screen()
	r.send(ui.ReloadMsg{})
	if after := r.screen(); after != before {
		t.Errorf("a reload changed the screen:\n%s\n%s", before, after)
	}
	if got := loadSettings(t, path); len(got) != 1 {
		t.Errorf("settings = %v", got)
	}
}

func TestASelectedIdThatLeavesTheData(t *testing.T) {
	src := asFixture(t)
	r := newRig(t, src, "", GlanceStart(), 100, 20).keys("E down down down down down down")
	if id := selectedID(r); id != "99f0" {
		t.Fatalf("selected %s", id)
	}
	base := src.fn
	src.fn = func(q source.Request) (view.Tableau, error) {
		tb, _ := base(q)
		var rows []view.Row
		for _, row := range tb.Rows {
			if row.ID != "99f0" {
				rows = append(rows, row)
			}
		}
		tb.Rows = rows
		return tb, nil
	}
	r.send(ui.ReloadMsg{})
	if id := selectedID(r); id != "bc86" {
		t.Errorf("after the row left the data the selection is %s, want the row above, bc86", id)
	}
	if last := r.selected[len(r.selected)-1]; last.Task != "bc86" {
		t.Errorf("the frame was told %+v", last)
	}
	// Everything from the selection on leaves: the nearest row above that stays.
	src.fn = func(q source.Request) (view.Tableau, error) {
		tb, _ := base(q)
		tb.Rows = tb.Rows[:3]
		return tb, nil
	}
	r.send(ui.ReloadMsg{})
	if id := selectedID(r); id != "c2ad" {
		t.Errorf("selected %s, want c2ad", id)
	}
	// No row at all.
	src.fn = func(q source.Request) (view.Tableau, error) {
		tb, _ := base(q)
		tb.Rows = nil
		return tb, nil
	}
	r.send(ui.ReloadMsg{})
	if got := r.line(3); got != "no task in view" {
		t.Errorf("empty body = %q", got)
	}
	if !strings.HasPrefix(r.status(), "rows 0 of 0 · window 1") {
		t.Errorf("status = %q", r.status())
	}
	r.keys("down up enter E C d p right left x s")
}

// T15: StartMsg sets request, folds, selection and level, and leaves the choices alone.
func TestStartMsg(t *testing.T) {
	path := settingsFile(t, "")
	src := asFixture(t)
	r := newRig(t, src, path, GlanceStart(), 100, 30).keys("right right right right x")
	file := readFile(t, path)
	r.send(StartMsg{Start: Start{
		Request: source.Request{View: "tableau", Window: 0, Historical: true, Level: "detail"},
		Open:    []string{"437e", "bc63", "2034"},
		Select:  "e9c6",
		Level:   Detail,
	}})
	if want := (source.Request{View: "tableau", Window: 0, Historical: true, Level: "detail"}); src.last() != want {
		t.Errorf("request = %+v, want %+v", src.last(), want)
	}
	if id := selectedID(r); id != "e9c6" {
		t.Errorf("selected %s, want e9c6", id)
	}
	ids := strings.Join(rowIDs(r), " ")
	if !strings.Contains(ids, "2034 e9c6 bc86 5fe3 e3cb") || strings.Contains(ids, "99f0") {
		t.Errorf("the folds are %s", ids)
	}
	if ruleAt(r) == -1 {
		t.Error("the strip is closed, want detail")
	}
	if !strings.Contains(r.status(), "window 0") || !strings.Contains(r.status(), "historical on") || !strings.Contains(r.status(), "1 changed") {
		t.Errorf("status = %q", r.status())
	}
	if !strings.Contains(r.line(1), "🔧 ×0") {
		t.Errorf("the choice was lost: %q", r.line(1))
	}
	if got := readFile(t, path); got != file {
		t.Errorf("the settings file changed:\n%s\n%s", file, got)
	}
	// A glance start folds back to the root and its children, and drops the strip.
	r.send(StartMsg{Start: GlanceStart()})
	if _, _, n := shownRange(t, r); n != 6 || ruleAt(r) != -1 || selectedID(r) != "437e" {
		t.Errorf("glance start: %d rows, rule at %d, selected %s", n, ruleAt(r), selectedID(r))
	}
	if got := readFile(t, path); got != file {
		t.Error("the settings file changed")
	}
	if _, err := os.Stat(path); err != nil {
		t.Error(err)
	}
	// A provenance start asks for the facts at once.
	st := GlanceStart()
	st.Level = Provenance
	r.send(StartMsg{Start: st})
	if src.last().Level != "provenance" {
		t.Errorf("request = %+v", src.last())
	}
}
