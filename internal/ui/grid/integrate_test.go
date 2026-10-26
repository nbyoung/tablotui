package grid

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/refresh"
	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/source/corpustest"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view"
	"github.com/nbyoung/tablotui/internal/watch"
)

// The integrate tests of 679b ("Later gates"): the grid through the tablo
// adapter on the corpus entry weather-station and on the umbrella's plan at
// 3cdae52, the commit docs/mockups/tableau.md draws, the time of a w round
// trip, and decision 16. They skip without TABLEAUX_CORPUS or TABLEAUX_DIR.

// mockupCommit is the commit of the umbrella that tableau.md draws.
const mockupCommit = "3cdae527d47d37fdff55bc302c76c017cf91c606"

// adapter opens the tablo adapter on dir at ref, loads the state st and
// stores the snapshot, as the live refresh does. It logs the load's time,
// which 171b A8 bounds at 100 ms on the umbrella's plan.
func adapter(t *testing.T, dir, ref string, st watch.State) *source.Tablo {
	t.Helper()
	src, err := source.NewTablo(t.Context(), dir, ref, nil)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	snap, err := src.Load(t.Context(), st)
	t.Logf("the load of %s took %v", filepath.Base(dir), time.Since(start))
	var diag *source.DiagnosticError
	if err != nil && !errors.As(err, &diag) { // a diagnostic beside the snapshot leaves it good
		t.Fatal(err)
	}
	src.Store(snap)
	return src
}

// umbrella is the adapter on a clone of the umbrella at the mockup's commit,
// on main, as the mockups read it.
func umbrella(t *testing.T) *source.Tablo {
	t.Helper()
	dir := corpustest.Umbrella(t, mockupCommit)
	return adapter(t, dir, "main", watch.State{Commit: mockupCommit})
}

// data is what a tableau holds beside its head.
func data(t view.Tableau) view.Tableau {
	return view.Tableau{Legend: t.Legend, Subject: t.Subject, Columns: t.Columns, Rows: t.Rows}
}

// (a) Each of tablo's golden files of weather-station that testdata/tablo
// holds equals the adapter's tableau for the same request on a copy of the
// entry, and the grid draws every row of it through the adapter.
func TestIntegrateWeatherStation(t *testing.T) {
	src := adapter(t, corpustest.Entry(t, "weather-station"), "", watch.State{})
	for _, g := range tabloGoldens {
		t.Run(g.file, func(t *testing.T) {
			r := source.Request{View: g.command, Window: 1, Level: g.level()}
			if p, ok := g.params["person"].(string); ok {
				r.Person = p
			}
			if task, ok := g.params["task"].(string); ok {
				r.Task = task
			}
			got, err := src.Tableau(t.Context(), r)
			if err != nil {
				t.Fatal(err)
			}
			if want := g.decode(t); !reflect.DeepEqual(data(got), data(want)) {
				t.Errorf("the adapter's %s differs from testdata/tablo/%s", g.command, g.file)
			}
		})
	}
	r := newRig(t, src, "", GlanceStart(), 120, 20).keys("E d")
	if got := r.line(0); got != "Weather station · main at edb30d2, 2026-09-28" {
		t.Errorf("context line = %q", got)
	}
	for _, id := range []string{"a1c0", "4e2b", "9f31", "c07d", "7b2e", "3c5d"} {
		if !strings.Contains(r.screen(), "\n"+id+" ") {
			t.Errorf("row %s does not draw:\n%s", id, r.screen())
		}
	}
}

// mockupTable reads the first table under the heading of tableau.md: its
// header's gate cells and, by id, each row's gate cells, in order.
func mockupTable(t *testing.T, md, heading string) (header []string, ids []string, cells map[string][]string) {
	t.Helper()
	_, rest, ok := strings.Cut(md, "\n## "+heading+"\n")
	if !ok {
		t.Fatalf("tableau.md has no section %s", heading)
	}
	split := func(line string) []string {
		parts := strings.Split(strings.Trim(line, "|"), "|")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	cells = map[string][]string{}
	id := regexp.MustCompile("^`([0-9a-f]{4})`$")
	for _, line := range strings.Split(rest, "\n") {
		switch {
		case strings.HasPrefix(line, "| Id |"):
			header = split(line)[2:]
		case header != nil && strings.HasPrefix(line, "|"):
			parts := split(line)
			if m := id.FindStringSubmatch(parts[0]); m != nil {
				ids = append(ids, m[1])
				cells[m[1]] = parts[2:]
			}
		case header != nil && len(ids) > 0:
			return header, ids, cells
		}
	}
	return header, ids, cells
}

// (b) The grid's cells on the umbrella's plan at 3cdae52 equal the cells of
// docs/mockups/tableau.md, which draws that commit, at the detail level and
// at a glance: the headers with the fold count, the ids in order, and each
// gate cell, a folded one empty.
func TestIntegrateUmbrellaMatchesTheMockup(t *testing.T) {
	md, err := os.ReadFile(filepath.Join(os.Getenv("TABLEAUX_DIR"), "docs", "mockups", "tableau.md"))
	if err != nil {
		t.Skipf("the umbrella's docs/mockups/tableau.md: %v", err)
	}
	src := umbrella(t)
	tb, err := src.Tableau(t.Context(), source.Request{View: "tableau", Window: 1, Level: "detail"})
	var diag *source.DiagnosticError
	if err != nil && !errors.As(err, &diag) {
		t.Fatal(err)
	}
	if diag != nil {
		t.Logf("beside the view: %v", diag)
	}
	m := ui.Measure{Method: ansi.GraphemeWidth}
	mt := buildMetrics(&tb, nil, m)
	var header []string
	for _, c := range mt.cols {
		header = append(header, c.header)
	}
	byID := map[string]view.Row{}
	var order []string
	for _, row := range tb.Rows {
		byID[row.ID] = row
		order = append(order, row.ID)
	}
	wantHeader, ids, cells := mockupTable(t, string(md), "Detail")
	if !reflect.DeepEqual(header, wantHeader) {
		t.Errorf("header %q, the mockup's %q", header, wantHeader)
	}
	if !reflect.DeepEqual(order, ids) {
		t.Errorf("rows %v, the mockup's %v", order, ids)
	}
	matched := 0
	for _, id := range ids {
		row, ok := byID[id]
		if !ok {
			continue
		}
		var got []string
		for _, c := range mt.cols {
			s := ""
			if !c.folded {
				s = mt.cellText(row.Cells[c.gates[0]], m)
			}
			got = append(got, s)
		}
		if !reflect.DeepEqual(got, cells[id]) {
			t.Errorf("row %s: cells %q, the mockup's %q", id, got, cells[id])
			continue
		}
		matched++
	}
	if matched != 37 {
		t.Errorf("%d of the mockup's %d rows match, want 37", matched, len(ids))
	}
	// At a glance the grid draws the root and its children, and the count
	// after a folded parent, as the mockup's Glance does.
	r := newRig(t, src, "", GlanceStart(), 140, 14)
	if got := r.line(0); got != "Tableaux tooling · main at 3cdae52, 2026-10-05" {
		t.Errorf("context line = %q", got)
	}
	_, glance, _ := mockupTable(t, string(md), "Glance")
	for i, id := range glance {
		if line := r.line(3 + i); !strings.HasPrefix(line, id+" ") {
			t.Errorf("glance row %d = %q, the mockup's %s", i, line, id)
		}
	}
	if line := r.line(4); !strings.Contains(line, "▸ Method (31)") {
		t.Errorf("glance row of bc63 = %q", line)
	}
}

// wBound bounds the w round trip: from the key to the view that answers it,
// one derivation of the stored snapshot in process and one draw. A load of
// the repository is no part of it (A9). On the umbrella's plan it took under
// 5 ms, and under 45 ms with the race detector, on the host of the integrate
// gate; the bound leaves room for a loaded host.
const wBound = 500 * time.Millisecond

// (c) The time of a w round trip, logged, under wBound: on weather-station,
// and on the umbrella's plan of 37 rows where TABLEAUX_DIR names it.
func TestIntegrateWRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		src  func(t *testing.T) *source.Tablo
	}{
		{"weather-station", func(t *testing.T) *source.Tablo {
			return adapter(t, corpustest.Entry(t, "weather-station"), "", watch.State{})
		}},
		{"umbrella", umbrella},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := c.src(t)
			r := newRig(t, src, "", GlanceStart(), 120, 30).keys("E left")
			before := r.status()
			var worst time.Duration
			for i := range 5 {
				start := time.Now()
				r.keys("w")
				took := time.Since(start)
				worst = max(worst, took)
				t.Logf("w round trip %d: %v", i+1, took)
				r.keys("left") // a key clears a diagnostic's notice, and the column cursor stays
				if !strings.Contains(r.status(), "window 2") {
					t.Fatalf("after w the status reads %q", r.status())
				}
				r.keys("W left")
			}
			if r.status() != before {
				t.Errorf("after w W the status reads %q, before %q", r.status(), before)
			}
			if worst > wBound {
				t.Errorf("the slowest w round trip took %v, over %v", worst, wBound)
			}
		})
	}
}

// stillWatcher is a watcher that reports one state and then no change: its
// wait ends at once, so that the test's commands run to their end.
type stillWatcher struct{}

func (stillWatcher) Snapshot(context.Context) watch.State { return watch.State{} }
func (stillWatcher) Next(_ context.Context, prev watch.State) (watch.State, error) {
	return prev, context.Canceled
}
func (stillWatcher) Poke() {}

// forceWatcher is a stillWatcher that counts the pokes it receives.
type forceWatcher struct {
	stillWatcher
	pokes int
}

func (w *forceWatcher) Poke() { w.pokes++ }

// liveRig builds the frame with the grid, whose start waits, and the live
// refresh over the adapter on dir, and runs the first load.
func liveRig(t *testing.T, dir string) *rig {
	t.Helper()
	r, _, _ := liveRigWith(t, dir)
	return r
}

// liveRigWith is liveRig that also returns the refresh pane and the watcher
// that counts the pokes.
func liveRigWith(t *testing.T, dir string) (*rig, ui.Pane, *forceWatcher) {
	t.Helper()
	src, err := source.NewTablo(t.Context(), dir, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := &forceWatcher{}
	m := refresh.New(t.Context(), watch.Source{Dir: dir}, w, time.Now, src.Load)
	p := refresh.NewPane(m, src.Store)
	g := New(Options{Source: src})
	return newRigWith(t, ui.New(ui.Options{Panes: []ui.Pane{g, p}, Styles: ui.DefaultStyles()}), 120, 16), p, w
}

// notice returns the frame's notice line, the third from the bottom.
func (r *rig) notice() string {
	lines := strings.Split(r.screen(), "\n")
	return lines[len(lines)-3]
}

// (d) Decision 16, through the live refresh, the adapter and the grid: an
// error diagnostic beside view data stores the snapshot, draws the tableau
// and reports the diagnostic on the notice line (171b decision 15); a
// missing version, an unaccepted version, an unusable gates.yaml and a broken
// tree fail the load, and the screen keeps what it shows and says why, on the
// live refresh's notice line (171b A5).
func TestIntegrateDiagnosticBesideTheView(t *testing.T) {
	r := liveRig(t, corpustest.Entry(t, "status-unknown-state"))
	r.send(StartMsg{Start: GlanceStart()})
	if !strings.HasPrefix(r.line(1), "Id") || r.line(3) == "" || strings.Contains(r.screen(), "no task in view") {
		t.Errorf("the tableau does not draw:\n%s", r.screen())
	}
	if got := r.notice(); !strings.HasPrefix(got, "The project has errors: .tableaux/status/") || !strings.Contains(got, ": S6 ") {
		t.Errorf("notice line = %q", got)
	}
	if got := r.status(); strings.HasPrefix(got, "error:") || !strings.HasSuffix(got, "working tree at , detached") {
		t.Errorf("status = %q, want the source", got)
	}
	for entry, code := range map[string]string{
		"version-missing":        "P2",
		"version-major-mismatch": "P4",
		"gates-missing":          "G1",
		"tree-parent-cycle":      "T10",
	} {
		t.Run(entry, func(t *testing.T) {
			r := liveRig(t, corpustest.Entry(t, entry))
			if got := r.line(1); got != "loading…" {
				t.Errorf("body = %q", got)
			}
			lines := strings.Split(r.screen(), "\n")
			if got := lines[len(lines)-3]; !strings.HasPrefix(got, "No view yet: ") || !strings.Contains(got, code+" ") {
				t.Errorf("notice line = %q, want the first diagnostic, %s", got, code)
			}
			if got := r.status(); got != "working tree" {
				t.Errorf("status = %q, want the source", got)
			}
		})
	}
}

// 171b decision 15: the notice line holds an error beside the view for as long
// as the error holds. A key leaves it; the mend of the file and a forced
// reload clear it. The watcher never reports a change, so ctrl+r's ForceMsg
// pokes it, and the pane's Init, which reads the watcher's state again,
// stands in for the poll that the poke wakes: no test sleeps on the clock.
func TestIntegrateDiagnosticClearsOnTheMend(t *testing.T) {
	dir := corpustest.Entry(t, "status-unknown-state")
	r, p, w := liveRigWith(t, dir)
	r.send(StartMsg{Start: GlanceStart()})
	if got := r.notice(); !strings.HasPrefix(got, "The project has errors: .tableaux/status/") || !strings.Contains(got, ": S6 ") {
		t.Fatalf("notice line = %q", got)
	}
	r.keys("j j")
	if got := r.notice(); !strings.HasPrefix(got, "The project has errors: ") {
		t.Errorf("after keys the notice line = %q", got)
	}
	file := filepath.Join(dir, ".tableaux", "status", "b2c9.yaml")
	if err := os.WriteFile(file, []byte("gate: defined\nstate: nominal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.send(refresh.ForceMsg{})
	if w.pokes != 1 {
		t.Fatalf("%d pokes after the force", w.pokes)
	}
	r.run(p.Init(ui.Context{}))
	if got := r.notice(); got != "" {
		t.Errorf("after the mend the notice line = %q", got)
	}
	if strings.Contains(r.screen(), "The project has errors") || !strings.HasPrefix(r.line(1), "Id") {
		t.Errorf("after the mend:\n%s", r.screen())
	}
}
