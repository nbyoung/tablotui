package detail

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view/list"
)

var allPanes = []string{TaskID, QueueID, BlockageID, HistoryID}

// T8: following.
func TestClosedPanesAskNothing(t *testing.T) {
	for _, id := range allPanes {
		x := pane(t, id, "testdata", 80, 20)
		x.sel("e9c6", "design").sel("5fe3", "").send(ui.ReloadMsg{}).sel("595e", "implementation")
		if n := x.src.count(); n != 0 {
			t.Errorf("a closed %s pane asked %d times", id, n)
		}
	}
}

func TestEachPaneAsksItsRequest(t *testing.T) {
	want := map[string]source.ListRequest{
		TaskID:     {View: "task", Task: "e9c6"},
		QueueID:    {View: "queue"},
		BlockageID: {View: "blockage", Task: "e9c6"},
		HistoryID:  {View: "history", Task: "e9c6"},
	}
	for _, id := range allPanes {
		x := pane(t, id, "testdata", 80, 20)
		x.sel("e9c6", "design").open()
		reqs := x.requests()
		if len(reqs) != 1 || reqs[0] != want[id] {
			t.Errorf("%s asked %+v, want %+v", id, reqs, want[id])
		}
	}
}

func TestAMoveOfTheRowCursorCostsOneLoadPerFollowingPane(t *testing.T) {
	x := pane(t, TaskID, "testdata", 80, 20).open().sel("e9c6", "design")
	n := x.src.count()
	x.sel("e9c6", "implementation").sel("e9c6", "")
	if got := x.src.count(); got != n {
		t.Errorf("a move of the column cursor asked %d times", got-n)
	}
	x.sel("5fe3", "")
	if got := x.src.since(n); len(got) != 1 || got[0].Task != "5fe3" {
		t.Errorf("a move of the row cursor asked %+v", got)
	}
}

func TestAReloadAsksOncePerOpenPaneEvenWithALoadInFlight(t *testing.T) {
	for _, id := range allPanes {
		x := pane(t, id, "testdata", 80, 20)
		x.sel("e9c6", "").open()
		n := x.src.count()
		x.send(ui.ReloadMsg{})
		if got := x.src.count() - n; got != 1 {
			t.Errorf("%s: a reload asked %d times", id, got)
		}
	}
	// With a load in flight, a reload asks again and the older answer drops.
	x := pane(t, TaskID, "testdata", 80, 20)
	x.hold = true
	x.sel("e9c6", "").open()
	x.send(ui.ReloadMsg{})
	if len(x.held) < 2 {
		t.Fatalf("held %d commands", len(x.held))
	}
	if got := x.src.count(); got != 0 {
		t.Fatalf("the stub saw %d requests before a command ran", got)
	}
	x.hold = false
	x.release()
	if got := x.src.count(); got != 2 {
		t.Errorf("a reload with a load in flight asked %d times in all", got)
	}
	if x.pane().pending || x.pane().data == nil {
		t.Errorf("the newest answer did not land: %+v", x.pane().pending)
	}
}

func TestAPaneClosedThroughAReloadAsksWhenItOpens(t *testing.T) {
	x := pane(t, TaskID, "testdata", 80, 20).sel("e9c6", "").open()
	n := x.src.count()
	x.open() // the pane has the focus: it closes
	if x.pane().open {
		t.Fatal("the pane stays open")
	}
	x.send(ui.ReloadMsg{})
	if x.src.count() != n {
		t.Fatal("a closed pane asked after a reload")
	}
	x.open()
	if got := x.src.count() - n; got != 1 {
		t.Errorf("the pane asked %d times when it opened", got)
	}
	// With nothing changed, opening again asks nothing.
	x.open()
	n = x.src.count()
	x.open()
	if x.src.count() != n {
		t.Error("a reopened pane with fresh data asked again")
	}
	// A selection that moved while the pane was closed asks when it opens.
	x.open()
	x.sel("5fe3", "").open()
	if got := x.src.since(n); len(got) != 1 || got[0].Task != "5fe3" {
		t.Errorf("the pane asked %+v after a selection that moved while it was closed", got)
	}
}

func TestNoRowDrawsNoTaskSelected(t *testing.T) {
	for _, id := range []string{TaskID, BlockageID, HistoryID} {
		x := pane(t, id, "testdata", 60, 10).open().sel("", "")
		if n := x.src.count(); n != 0 {
			t.Errorf("%s asked with no row", id)
		}
		if got := x.view()[1]; got != "│ no task selected" {
			t.Errorf("%s draws %q", id, got)
		}
		if got := x.status(); got != id+" · lines 1–1 of 1" {
			t.Errorf("%s status = %q", id, got)
		}
	}
	// The queue follows the viewer, not the row.
	x := pane(t, QueueID, "testdata", 60, 10).sel("", "").open()
	if n := x.src.count(); n != 1 {
		t.Errorf("the queue asked %d times with no row", n)
	}
	// A selection that leaves while a load is in flight drops the answer.
	y := pane(t, TaskID, "testdata", 60, 10)
	y.hold = true
	y.open().sel("e9c6", "").sel("", "")
	y.hold = false
	y.release()
	if y.pane().data != nil {
		t.Error("an answer for a row the grid left landed")
	}
}

// T9: the queue.
func TestQueueCostsOneLoad(t *testing.T) {
	x := pane(t, QueueID, "testdata", 80, 20).open()
	for _, id := range []string{"e9c6", "99f0", "5fe3", "", "e9c6"} {
		x.sel(id, "")
	}
	if n := x.src.count(); n != 1 {
		t.Errorf("the queue asked %d times as the selection moved", n)
	}
}

func TestQueueMarkFollowsTheSelection(t *testing.T) {
	x := pane(t, QueueID, "testdata", 80, 20).open().keys("d")
	marked := func() []string {
		var out []string
		for _, l := range x.view() {
			if strings.HasPrefix(l, "│ >") {
				out = append(out, strings.TrimSpace(strings.TrimPrefix(l, "│ >")))
			}
		}
		return out
	}
	if got := marked(); len(got) != 0 {
		t.Errorf("no selection marks %v", got)
	}
	x.sel("e9c6", "")
	if got := marked(); len(got) != 1 || !strings.HasPrefix(got[0], "e9c6 Abstract views") {
		t.Errorf("e9c6 marks %v", got)
	}
	x.sel("c2ad", "")
	if got := marked(); len(got) != 1 || !strings.HasPrefix(got[0], "c2ad Roles") {
		t.Errorf("c2ad marks %v", got)
	}
}

func TestQueueNeedsAPerson(t *testing.T) {
	x := pane(t, QueueID, "testdata", 80, 10)
	x.src.fail = func(source.ListRequest) error { return source.ErrNoPerson }
	x.open().sel("e9c6", "").sel("99f0", "")
	if n := x.src.count(); n != 1 {
		t.Errorf("the queue asked %d times for a viewer with no email", n)
	}
	if got := x.view()[1]; got != "│ The work queue needs a person, and the viewer states none." {
		t.Errorf("view = %q", got)
	}
	// A reload asks again.
	x.send(ui.ReloadMsg{})
	if n := x.src.count(); n != 2 {
		t.Errorf("a reload asked %d times in all", n)
	}
	// The sentence draws in plain text.
	if strings.Contains(x.p.View(80, 10, x.c), "\x1b[1;31m") {
		t.Error("the sentence draws as an error")
	}
}

func TestEmptyQueue(t *testing.T) {
	x := pane(t, QueueID, "testdata/empty", 80, 10).open()
	v := x.view()
	if v[0] != "┌─ 2 Queue nbyoung@nbyoung.com · 0 items · glance ──────────────────────────────" || v[1] != "│ The queue is empty." {
		t.Errorf("view = %q", v)
	}
}

// T10: sequence and failure.
func TestOlderAnswersChangeNothing(t *testing.T) {
	x := pane(t, TaskID, "testdata", 80, 12)
	x.hold = true
	x.open()
	tasks := []string{"e9c6", "5fe3", "595e"}
	for i := range 12 {
		x.sel(tasks[i%3], "")
	}
	// The open asked nothing: no row stood. Twelve selections ask twelve times.
	if len(x.held) != 12+1 {
		t.Fatalf("held %d commands", len(x.held))
	}
	if got := x.view()[0]; !strings.HasSuffix(got, "1 Task · loading… ─") && !strings.Contains(got, "· loading…") {
		t.Errorf("title = %q", got)
	}
	// The focus command and the twelve loads: the first held command is the
	// open's, which carries a focus message and no load.
	for i := 1; i < 12; i++ {
		x.runHeld(1)
		if x.pane().data != nil {
			t.Fatalf("answer %d changed the pane", i)
		}
		if !strings.Contains(x.view()[0], "· loading…") {
			t.Fatalf("after answer %d the title = %q", i, x.view()[0])
		}
	}
	x.runHeld(1) // the newest
	x.runHeld(0)
	if x.pane().data == nil || x.pane().pending {
		t.Fatalf("the newest answer did not land")
	}
	if got := x.view()[0]; strings.Contains(got, "loading") || !strings.Contains(got, "1 Task 595e") {
		t.Errorf("title = %q", got)
	}
}

func TestAFailedReloadKeepsTheView(t *testing.T) {
	x := pane(t, TaskID, "testdata", 80, 12).open().sel("e9c6", "")
	before := strings.Join(x.view(), "\n")
	x.src.fail = func(source.ListRequest) error { return errBoom }
	x.notices = nil
	x.send(ui.ReloadMsg{})
	if len(x.notices) != 1 || x.notices[0] != (ui.NoticeMsg{Text: "task: boom (showing the last good view)", Err: true}) {
		t.Errorf("notices = %+v", x.notices)
	}
	if got := strings.Join(x.view(), "\n"); got != before {
		t.Errorf("the view changed:\n%s\n%s", got, before)
	}
	// The pane does not ask again by itself; the next selection does.
	n := x.src.count()
	x.sel("e9c6", "implementation")
	if x.src.count() != n {
		t.Error("the pane asked again for the same request")
	}
}

func TestAFailedLoadForAnotherTaskDropsTheView(t *testing.T) {
	x := pane(t, TaskID, "testdata", 80, 12).open().sel("e9c6", "")
	x.src.fail = func(source.ListRequest) error { return errBoom }
	x.notices = nil
	x.sel("5fe3", "")
	if len(x.notices) != 0 {
		t.Errorf("notices = %+v", x.notices)
	}
	v := x.view()
	if v[1] != "│ error: boom" || strings.Contains(v[0], "e9c6") {
		t.Errorf("view = %q", v[:2])
	}
	if st := x.status(); st != "task · lines 1–1 of 1" {
		t.Errorf("status = %q", st)
	}
	// The next selection asks again, and the view returns.
	x.src.fail = nil
	x.sel("595e", "")
	if got := x.view()[0]; !strings.Contains(got, "1 Task 595e") {
		t.Errorf("title = %q", got)
	}
}

func TestAFirstLoadThatFails(t *testing.T) {
	x := pane(t, HistoryID, "testdata", 80, 12)
	x.src.fail = func(source.ListRequest) error { return errors.New("no such view") }
	x.sel("e9c6", "").open()
	if got := x.view()[1]; got != "│ error: no such view" {
		t.Errorf("view = %q", got)
	}
	x.src.fail = nil
	x.send(ui.ReloadMsg{})
	if got := x.view()[0]; !strings.Contains(got, "4 History e9c6") {
		t.Errorf("title = %q", got)
	}
	// An answer of another type is an error, not a panic.
	p, _ := x.p.Update(ui.LoadedMsg{Pane: HistoryID, Seq: x.pane().seq, Value: 42}, x.c)
	if p.(Pane).data != nil && p.(Pane).failed {
		t.Error("a pane with data and a bad answer drops the data")
	}
	y := pane(t, HistoryID, "testdata", 80, 12).sel("e9c6", "")
	y.hold = true
	y.open()
	y.hold = false
	z, _ := y.p.Update(ui.LoadedMsg{Pane: HistoryID, Seq: y.pane().seq, Value: list.Task{}}, y.c)
	if got := z.(Pane).errText; !strings.Contains(got, "unexpected data of type list.Task") {
		t.Errorf("error = %q", got)
	}
	// A message for another pane changes nothing.
	z2, cmd := y.p.Update(ui.LoadedMsg{Pane: TaskID, Seq: 1}, y.c)
	if cmd != nil || fmt.Sprint(z2.(Pane).seq) != fmt.Sprint(y.pane().seq) {
		t.Error("a message for another pane changed the pane")
	}
}

// T11: levels.
func TestDataThatStatesGlanceDrawsGlance(t *testing.T) {
	x := pane(t, HistoryID, "testdata", 80, 12).open().sel("5fe3", "")
	if got := x.view()[0]; !strings.Contains(got, "4 History 5fe3 · 0 events · glance") {
		t.Errorf("title = %q", got)
	}
	if got := x.status(); got != "history 5fe3 · glance · lines 1–1 of 1" {
		t.Errorf("status = %q", got)
	}
}

func TestDetailIsAskedOnceAndThenCostsNothing(t *testing.T) {
	x := pane(t, TaskID, "testdata", 80, 12).withSource(roleSource{source.File{Dir: "testdata"}})
	x.open().sel("e9c6", "")
	if got := x.pane().shown(); got != list.Glance {
		t.Fatalf("the role opens the pane at %v", got)
	}
	x.keys("d")
	if got := x.pane().shown(); got != list.Detail {
		t.Errorf("d shows %v", got)
	}
	reqs := x.requests()
	if len(reqs) != 2 || reqs[0].Level != "" || reqs[1] != (source.ListRequest{View: "task", Task: "e9c6", Level: "detail"}) {
		t.Errorf("requests = %+v", reqs)
	}
	x.keys("d")
	if got := x.pane().shown(); got != list.Glance {
		t.Errorf("d again shows %v", got)
	}
	x.keys("d")
	if got := x.pane().shown(); got != list.Detail || x.src.count() != 2 {
		t.Errorf("d a third time shows %v after %d requests", got, x.src.count())
	}
}

func TestProvenanceIsAskedOnceAndPReturnsToDetail(t *testing.T) {
	x := pane(t, TaskID, "testdata", 80, 12).open().sel("e9c6", "")
	x.keys("p")
	if got := x.pane().shown(); got != list.Provenance {
		t.Fatalf("p shows %v", got)
	}
	reqs := x.requests()
	if len(reqs) != 2 || reqs[1] != (source.ListRequest{View: "task", Task: "e9c6", Level: "provenance"}) {
		t.Errorf("requests = %+v", reqs)
	}
	x.keys("p")
	if got := x.pane().shown(); got != list.Detail || x.src.count() != 2 {
		t.Errorf("p again shows %v after %d requests", got, x.src.count())
	}
	x.keys("p")
	if got := x.pane().shown(); got != list.Provenance || x.src.count() != 2 {
		t.Errorf("p a third time shows %v after %d requests", got, x.src.count())
	}
}

func TestTheChoiceSurvivesANewSelection(t *testing.T) {
	x := pane(t, TaskID, "testdata", 80, 12).open().sel("e9c6", "").keys("p")
	n := x.src.count()
	x.sel("5fe3", "")
	got := x.src.since(n)
	if len(got) != 1 || got[0] != (source.ListRequest{View: "task", Task: "5fe3", Level: "provenance"}) {
		t.Errorf("the next request = %+v", got)
	}
	if lvl := x.pane().shown(); lvl != list.Provenance {
		t.Errorf("the new task draws %v", lvl)
	}
	// After detail was chosen from provenance, requests still ask for the
	// facts: the pane drew them once.
	x.keys("p")
	n = x.src.count()
	x.sel("595e", "")
	if got := x.src.since(n); len(got) != 1 || got[0].Level != "provenance" || x.pane().shown() != list.Detail {
		t.Errorf("after p, the next request = %+v, shown %v", got, x.pane().shown())
	}
	// A glance choice asks for detail, never glance, and costs no load.
	y := pane(t, TaskID, "testdata", 80, 12).open().sel("e9c6", "").keys("d")
	if y.src.count() != 1 || y.pane().shown() != list.Glance {
		t.Errorf("d on detail data: %d requests, shown %v", y.src.count(), y.pane().shown())
	}
	y.sel("5fe3", "")
	if got := y.src.since(1); len(got) != 1 || got[0].Level != "detail" {
		t.Errorf("a glance choice asked %+v", got)
	}
}

func TestProvenanceOnTheQueueSaysWhyNot(t *testing.T) {
	x := pane(t, QueueID, "testdata", 80, 12).open()
	n := x.src.count()
	x.keys("p")
	want := ui.NoticeMsg{Text: "the work queue's provenance is the brief of one item, which this pane does not draw"}
	if len(x.notices) != 1 || x.notices[0] != want {
		t.Errorf("notices = %+v", x.notices)
	}
	if x.src.count() != n || x.pane().shown() != list.Detail {
		t.Errorf("p changed the queue: %d requests, shown %v", x.src.count()-n, x.pane().shown())
	}
}

// T13: the cursor.
func TestCursorKeys(t *testing.T) {
	x := pane(t, TaskID, "testdata", 80, 10).open().sel("e9c6", "")
	at := func() int { return x.pane().cur }
	if at() != 0 {
		t.Fatalf("the cursor arrives on line %d", at())
	}
	x.keys("down")
	if at() != 1 {
		t.Errorf("down: line %d", at())
	}
	x.keys("j j")
	if at() != 3 {
		t.Errorf("j j: line %d", at())
	}
	x.keys("up k")
	if at() != 1 {
		t.Errorf("up k: line %d", at())
	}
	x.keys("up up")
	if at() != 0 {
		t.Errorf("up past the top: line %d", at())
	}
	// A page is the pane's rows: nine.
	x.keys("pgdown")
	win := x.pane().settle(80, 10, ui.Measure{})
	if win.cur != 9 || win.top != 1 {
		t.Errorf("pgdown: row %d, top %d", win.cur, win.top)
	}
	x.keys("pgup")
	if win := x.pane().settle(80, 10, ui.Measure{}); win.cur != 0 || win.top != 0 {
		t.Errorf("pgup: row %d, top %d", win.cur, win.top)
	}
	x.keys("end")
	n := len(x.pane().settle(80, 10, ui.Measure{}).rows)
	if win := x.pane().settle(80, 10, ui.Measure{}); win.cur != n-1 || win.top != n-9 {
		t.Errorf("end: row %d of %d, top %d", win.cur, n, win.top)
	}
	x.keys("down")
	if win := x.pane().settle(80, 10, ui.Measure{}); win.cur != n-1 {
		t.Errorf("down past the end: row %d", win.cur)
	}
	x.keys("home")
	if win := x.pane().settle(80, 10, ui.Measure{}); win.cur != 0 || win.top != 0 {
		t.Errorf("home: row %d, top %d", win.cur, win.top)
	}
	x.keys("G")
	if win := x.pane().settle(80, 10, ui.Measure{}); win.cur != n-1 {
		t.Errorf("G: row %d", win.cur)
	}
	x.keys("g")
	if win := x.pane().settle(80, 10, ui.Measure{}); win.cur != 0 {
		t.Errorf("g: row %d", win.cur)
	}
}

func TestCursorMovesByRows(t *testing.T) {
	// The queue's first entry wraps at 40 cells, so down visits its rows.
	x := pane(t, QueueID, "testdata", 40, 10).open().keys("d down down")
	p := x.pane()
	if p.cur != 1 || p.sub != 1 {
		t.Errorf("the cursor stands on line %d row %d", p.cur, p.sub)
	}
}

func TestHistoryArrivesOnItsLastRow(t *testing.T) {
	x := pane(t, HistoryID, "testdata", 80, 12).open().sel("e9c6", "")
	win := x.pane().settle(80, 12, ui.Measure{})
	if win.cur != len(win.rows)-1 || win.top != len(win.rows)-11 {
		t.Errorf("row %d of %d, top %d", win.cur, len(win.rows), win.top)
	}
	if got := x.status(); !strings.HasSuffix(got, fmt.Sprintf("lines %d–%d of %d · enter selects e9c6 × design", len(win.rows)-10, len(win.rows), len(win.rows))) {
		t.Errorf("status = %q", got)
	}
	// It arrives on the last row for another task too, and the task pane on the first.
	x.sel("595e", "")
	win = x.pane().settle(80, 12, ui.Measure{})
	if win.cur != len(win.rows)-1 {
		t.Errorf("for 595e: row %d of %d", win.cur, len(win.rows))
	}
	y := pane(t, TaskID, "testdata", 80, 12).open().sel("e9c6", "").keys("end").sel("5fe3", "")
	if y.pane().cur != 0 {
		t.Errorf("the task pane arrives on line %d", y.pane().cur)
	}
}

func TestCursorStaysOnItsTaskAcrossAnAnswerForTheSame(t *testing.T) {
	x := pane(t, TaskID, "testdata", 80, 12).open().sel("e9c6", "")
	x.keys("end")
	line := x.pane().cur
	item := x.pane().lines[line].Item
	x.send(ui.ReloadMsg{})
	if got := x.pane().lines[x.pane().cur].Item; got != item {
		t.Errorf("after a reload the cursor stands on item %d, not %d", got, item)
	}
}

func TestCursorStaysInViewThroughAResize(t *testing.T) {
	x := pane(t, HistoryID, "testdata", 100, 30).open().sel("e9c6", "")
	for _, sz := range [][2]int{{60, 12}, {100, 30}, {40, 5}, {100, 30}} {
		x.c.Width, x.c.Height = sz[0], sz[1]
		win := x.pane().settle(sz[0], sz[1], ui.Measure{})
		if win.cur < win.top || win.cur >= win.top+max(sz[1]-1, 1) {
			t.Errorf("at %dx%d the cursor row %d stands outside the rows from %d", sz[0], sz[1], win.cur, win.top)
		}
		var tail string
		for _, l := range x.view() {
			if strings.Contains(l, "claude-fable") {
				tail = l
			}
		}
		if sz[1] >= 9 && tail == "" {
			t.Errorf("at %dx%d the last row is not drawn:\n%s", sz[0], sz[1], strings.Join(x.view(), "\n"))
		}
		x.send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]}) // no pane reads it
	}
}

func TestCursorStaysOnItsItemThroughDFAndAReload(t *testing.T) {
	x := pane(t, QueueID, "testdata", 80, 20).open()
	x.keys("d down down")
	itemAt := func() (int, string) {
		p := x.pane()
		return p.lines[p.cur].Item, p.lines[p.cur].Task
	}
	item, task := itemAt()
	if task != "e9c6" {
		t.Fatalf("the cursor stands on %q", task)
	}
	for _, step := range []string{"d", "d", "f", "f"} {
		x.keys(step)
		if i, tk := itemAt(); i != item || tk != "e9c6" {
			t.Errorf("after %s the cursor stands on item %d (%s), not %d", step, i, tk, item)
		}
	}
	x.send(ui.ReloadMsg{})
	if i, _ := itemAt(); i != item {
		t.Errorf("after a reload the cursor stands on item %d, not %d", i, item)
	}
	// The cursor returns to the first line of its item.
	if l := x.pane().lines[x.pane().cur]; l.Gate == "" || x.pane().sub != 0 {
		t.Errorf("cursor line = %+v, row %d", l, x.pane().sub)
	}
}

var statusRE = regexp.MustCompile(`^queue noreply@anthropic\.com · (glance|detail) · lines (\d+)–(\d+) of (\d+)$`)

func TestStatusLine(t *testing.T) {
	x := pane(t, QueueID, "testdata", 60, 10).open()
	if got := x.status(); !statusRE.MatchString(got) || !strings.Contains(got, "detail · lines 1–9 of ") {
		// The queue's first row is a heading, which names no task.
		t.Errorf("status at the first row = %q", got)
	}
	x.keys("down")
	if got := x.status(); !strings.HasSuffix(got, " · enter selects c2ad × validate") {
		t.Errorf("status on an entry = %q", got)
	}
	y := pane(t, TaskID, "testdata", 60, 10)
	if got := y.status(); got != "task · lines 1–1 of 1" {
		t.Errorf("a closed pane with no row = %q", got)
	}
	z := pane(t, QueueID, "testdata", 60, 10).open().keys("d")
	if got := z.status(); !strings.Contains(got, "glance") {
		t.Errorf("status = %q", got)
	}
}

// T15: colour.
var sgr = regexp.MustCompile("\x1b\\[([0-9;]*)m")

func hasParam(s, param string) int {
	n := 0
	for _, m := range sgr.FindAllStringSubmatch(s, -1) {
		for _, p := range strings.Split(m[1], ";") {
			if p == param {
				n++
			}
		}
	}
	return n
}

func TestColour(t *testing.T) {
	styled := pane(t, TaskID, "testdata", 80, 12).open().sel("e9c6", "")
	plain := pane(t, TaskID, "testdata", 80, 12).open().sel("e9c6", "")
	plain.c.Styles = ui.Styles{}
	for _, keys := range []string{"", "down down", "d", "p end"} {
		styled.keys(keys)
		plain.keys(keys)
		if got, want := strings.Join(styled.view(), "\n"), strings.Join(plain.view(), "\n"); got != want {
			t.Errorf("keys %q: the stripped view differs from the plain view:\n%s\n%s", keys, got, want)
		}
	}
	if strings.Contains(plain.p.View(80, 12, plain.c), "\x1b") {
		t.Error("the zero Styles draw an escape sequence")
	}

	x := pane(t, TaskID, "testdata", 80, 12).open().sel("e9c6", "").keys("down")
	lines := strings.Split(x.p.View(80, 12, x.c), "\n")
	if hasParam(lines[0], "7") != 1 {
		t.Errorf("the focused title is not reversed: %q", lines[0])
	}
	if hasParam(lines[2], "7") != 1 || hasParam(lines[1], "7") != 0 || hasParam(lines[3], "7") != 0 {
		t.Errorf("the cursor row is not the only reversed row: %q", lines[1:4])
	}
	if hasParam(lines[1], "1") != 1 {
		t.Errorf("the heading is not bold: %q", lines[1])
	}
	if hasParam(lines[3], "1") != 0 {
		t.Errorf("a field is bold: %q", lines[3])
	}
	// Without the focus the title and the cursor row do not reverse.
	x.c.Focused = false
	lines = strings.Split(x.p.View(80, 12, x.c), "\n")
	for i, l := range lines {
		if hasParam(l, "7") != 0 {
			t.Errorf("line %d carries reverse without the focus: %q", i, l)
		}
	}
	if hasParam(lines[0], "1") != 1 {
		t.Errorf("the unfocused title is not bold: %q", lines[0])
	}
	// An error draws in the error style.
	e := pane(t, TaskID, "testdata", 80, 12)
	e.src.fail = func(source.ListRequest) error { return errBoom }
	e.open().sel("e9c6", "")
	if row := strings.Split(e.p.View(80, 12, e.c), "\n")[1]; !strings.Contains(row, "\x1b[") || !strings.Contains(ansiStrip(row), "error: boom") {
		t.Errorf("the error row = %q", row)
	}
}
