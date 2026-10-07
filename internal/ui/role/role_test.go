package role

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/ui/grid"
	"github.com/nbyoung/tablotui/internal/view"
)

// T1: the screens.
func TestScreens(t *testing.T) {
	cases := []struct {
		golden, viewer, keys string
		w, h                 int
	}{
		{"owner-100x14.txt", owner, "", 100, 14},
		{"corner-100x24.txt", owner, "R", 100, 24},
		{"narrow-60x16.txt", owner, "R", 60, 16},
		{"reviewer-100x24.txt", owner, "R R", 100, 24},
		{"observer-100x14.txt", stranger, "R", 100, 14},
		{"nobody-100x14.txt", "", "", 100, 14},
	}
	for _, c := range cases {
		t.Run(c.golden, func(t *testing.T) {
			r := newRig(t, c.viewer, "", c.w, c.h).keys(c.keys)
			if got, want := r.screen(), golden(t, c.golden); got != want {
				t.Errorf("screen:\n%s\nwant:\n%s", got, want)
			}
			for _, method := range []ansi.Method{ansi.WcWidth, ansi.GraphemeWidth} {
				if method == ansi.GraphemeWidth {
					r.grapheme()
				}
				lines := r.content()
				if len(lines) != c.h {
					t.Errorf("%d lines, want %d", len(lines), c.h)
				}
				for i, l := range lines {
					if got := method.StringWidth(ansi.Strip(l)); got != c.w {
						t.Errorf("line %d is %d cells under %v, want %d: %q", i, got, method, c.w, ansi.Strip(l))
					}
				}
			}
		})
	}
}

// T2: Stops.
func TestStops(t *testing.T) {
	cases := []struct {
		roles []string
		want  []string
	}{
		{[]string{"owner", "authority", "assignee", "reviewer"}, []string{"owner", "assignee", "reviewer"}},
		{[]string{"authority", "assignee", "contributor", "agent"}, []string{"assignee", "observer"}},
		{[]string{"assignee", "contributor", "reviewer"}, []string{"assignee", "reviewer", "observer"}},
		{[]string{"reviewer"}, []string{"reviewer", "observer"}},
		{[]string{"owner"}, []string{"owner"}},
		{[]string{"observer"}, []string{"observer"}},
		{[]string{"contributor", "agent"}, []string{"contributor", "observer"}},
		{[]string{"agent"}, []string{"agent", "observer"}},
		{[]string{"authority"}, []string{"authority", "observer"}},
		{nil, []string{"observer"}},
		{[]string{"janitor"}, []string{"observer"}},
		{[]string{"janitor", "owner"}, []string{"owner"}},
		{[]string{"reviewer", "owner", "reviewer"}, []string{"owner", "reviewer"}},
	}
	for _, c := range cases {
		if got := Stops(c.roles); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Stops(%v) = %v, want %v", c.roles, got, c.want)
		}
	}
}

// T3: Arrival.
func TestArrival(t *testing.T) {
	const ref = "main"
	req := func(view, person, role string) source.Request {
		return source.Request{View: view, Ref: ref, Person: person, Window: 1, Level: "detail", Viewer: owner, Role: role}
	}
	cases := []struct {
		role string
		want grid.Start
	}{
		{Owner, grid.Start{Request: req("tableau", "", Owner), Level: grid.Glance}},
		{Observer, grid.Start{Request: req("tableau", "", Observer), Level: grid.Glance}},
		{"", grid.Start{Request: req("tableau", "", ""), Level: grid.Glance}},
		{"janitor", grid.Start{Request: req("tableau", "", "janitor"), Level: grid.Glance}},
		{Assignee, grid.Start{Request: req("context", owner, Assignee), Level: grid.Detail, Mine: true}},
		{Contributor, grid.Start{Request: req("context", owner, Contributor), Level: grid.Detail, Mine: true}},
		{Agent, grid.Start{Request: req("context", owner, Agent), Level: grid.Detail, Mine: true}},
		{Authority, grid.Start{Request: req("context", owner, Authority), Level: grid.Detail, Mine: true}},
		{Reviewer, grid.Start{Request: req("tableau", owner, Reviewer), Level: grid.Glance, Mine: true}},
	}
	for _, c := range cases {
		if got := Arrival(c.role, owner, ref); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Arrival(%q) = %+v, want %+v", c.role, got, c.want)
		}
	}
	// The observer's arrival is the grid's glance start but for viewer, role and ref.
	g := grid.GlanceStart()
	g.Request.Viewer, g.Request.Role, g.Request.Ref = owner, Owner, ref
	if got := Arrival(Owner, owner, ref); !reflect.DeepEqual(got, g) {
		t.Errorf("Arrival(owner) = %+v, want the glance start %+v", got, g)
	}
}

// T4: one load at the start, and the cycle.
func TestTheCycle(t *testing.T) {
	r := newRig(t, owner, "", 100, 24)
	if n := len(r.spy.reqs); n != 1 || r.spy.asked != 1 {
		t.Fatalf("start: %d tableau requests, %d role requests; want one each", n, r.spy.asked)
	}
	if want := (source.Request{View: "tableau", Window: 1, Level: "detail", Viewer: owner, Role: Owner}); r.spy.last() != want {
		t.Errorf("first request = %+v, want %+v", r.spy.last(), want)
	}
	if got := r.context(); !strings.HasSuffix(got, "viewer "+owner+", owner") {
		t.Errorf("context = %q", got)
	}
	steps := []struct {
		req    source.Request
		notice string
		role   string
	}{
		{source.Request{View: "context", Person: owner, Window: 1, Level: "detail", Viewer: owner, Role: Assignee}, "reading as assignee, 2 of 3", "assignee"},
		{source.Request{View: "tableau", Person: owner, Window: 1, Level: "detail", Viewer: owner, Role: Reviewer}, "reading as reviewer, 3 of 3", "reviewer"},
		{source.Request{View: "tableau", Window: 1, Level: "detail", Viewer: owner, Role: Owner}, "reading as owner, 1 of 3", "owner"},
	}
	for i, s := range steps {
		r.keys("R")
		if got := r.spy.last(); got != s.req || len(r.spy.reqs) != i+2 {
			t.Errorf("R %d asked %+v (%d in all), want %+v", i+1, got, len(r.spy.reqs), s.req)
		}
		if got := r.status(); got != s.notice {
			t.Errorf("R %d: status = %q, want %q", i+1, got, s.notice)
		}
		if got := r.context(); !strings.HasSuffix(got, "viewer "+owner+", "+s.role) {
			t.Errorf("R %d: context = %q", i+1, got)
		}
	}
	if r.spy.asked != 1 {
		t.Errorf("the roles loaded %d times, want once", r.spy.asked)
	}
}

// The ref of the options reaches every arrival and the roles request.
func TestTheRefReachesEveryArrival(t *testing.T) {
	src := &spy{File: source.File{Dir: fixtures}}
	g := grid.New(grid.Options{Source: src})
	who := New(Options{Source: src, Viewer: owner, Ref: "v1"})
	m := ui.New(ui.Options{Panes: []ui.Pane{g, who}, Commands: []ui.Command{Command()}, Styles: ui.DefaultStyles()})
	launch(t, m, src, 100, 24).keys("R R R")
	if len(src.reqs) != 4 {
		t.Fatalf("%d requests", len(src.reqs))
	}
	for _, q := range src.reqs {
		if q.Ref != "v1" {
			t.Errorf("request %+v states no ref", q)
		}
	}
}

// T5: what a switch keeps and resets.
func TestASwitchKeepsTheColumnsAndResetsTheRest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	r := newRig(t, owner, path, 100, 24)
	r.spy.lenient = true
	r.keys("right right right x") // hide 📌
	r.keys("w H p")
	if last := r.spy.last(); last.Window != 2 || !last.Historical || last.Level != "provenance" {
		t.Fatalf("setup asked %+v", last)
	}
	before, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(before), `"mockup": "hide"`) {
		t.Fatalf("settings = %q, %v", before, err)
	}
	r.keys("R")
	want := source.Request{View: "context", Person: owner, Window: 1, Level: "provenance", Viewer: owner, Role: Assignee}
	if got := r.spy.last(); got != want {
		t.Errorf("R asked %+v, want %+v", got, want)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Errorf("settings changed: %q, then %q, %v", before, after, err)
	}
	if head := r.line(1); !strings.Contains(head, "📌 ×0") {
		t.Errorf("the column hidden before the switch is back: %q", head)
	}
	if got := r.status(); got != "reading as assignee, 2 of 3" {
		t.Errorf("status = %q", got)
	}
}

// T6: the selection.
func TestTheSelectionSurvivesASwitch(t *testing.T) {
	r := newRig(t, owner, "", 100, 24).keys("down down")
	r.keys("R")
	starts := r.starts()
	if got := starts[len(starts)-1]; got.Select != "77b2" {
		t.Fatalf("the start selects %q, want 77b2", got.Select)
	}
	r.keys("down")
	if got := r.lastSel(); got != "6103" {
		t.Errorf("one more down selects %q, want 6103", got)
	}
}

// T7: one stop, nobody, a stranger.
func TestOneStopNobodyAStranger(t *testing.T) {
	t.Run("one stop", func(t *testing.T) {
		r := newRig(t, stranger, "", 100, 14)
		n := len(r.spy.reqs)
		r.keys("R")
		if len(r.spy.reqs) != n || r.status() != "one role only: observer" {
			t.Errorf("R asked %d more; status %q", len(r.spy.reqs)-n, r.status())
		}
		if got := r.context(); !strings.HasSuffix(got, "viewer "+stranger+", observer") {
			t.Errorf("context = %q", got)
		}
	})
	t.Run("nobody", func(t *testing.T) {
		r := newRig(t, "", "", 100, 14)
		if r.spy.asked != 0 || len(r.spy.reqs) != 1 {
			t.Fatalf("nobody asked %d times for roles and %d for tableaux", r.spy.asked, len(r.spy.reqs))
		}
		if want := (source.Request{View: "tableau", Window: 1, Level: "detail", Role: Observer}); r.spy.last() != want {
			t.Errorf("request = %+v, want %+v", r.spy.last(), want)
		}
		if got := r.status(); got != "no Git identity: reading as an observer; pass --as <email>" {
			t.Errorf("status = %q", got)
		}
		if strings.Contains(r.context(), "viewer") {
			t.Errorf("context = %q", r.context())
		}
		r.keys("R")
		if r.status() != "one role only: observer" || r.spy.asked != 0 {
			t.Errorf("status = %q after R; %d role requests", r.status(), r.spy.asked)
		}
		r.send(ui.ReloadMsg{})
		if r.spy.asked != 0 {
			t.Errorf("a reload asked nobody's roles")
		}
	})
}

// T8: a roles load that fails.
func TestARolesLoadThatFails(t *testing.T) {
	src := &spy{File: source.File{Dir: fixtures}, failing: true}
	g := grid.New(grid.Options{Source: src})
	who := New(Options{Source: src, Viewer: owner})
	m := ui.New(ui.Options{Panes: []ui.Pane{g, who}, Commands: []ui.Command{Command()}, Styles: ui.DefaultStyles()})
	r := launch(t, m, src, 100, 24)
	if len(src.reqs) != 1 || src.last().Role != "" || src.last().View != "tableau" || src.last().Viewer != owner {
		t.Fatalf("the grid asked %+v", src.reqs)
	}
	if got := r.status(); got != "error: roles: the project does not load" {
		t.Errorf("status = %q", got)
	}
	if !strings.Contains(r.screen(), "▾ Tableaux tooling") {
		t.Errorf("the grid shows no tableau:\n%s", r.screen())
	}
	if got := r.context(); !strings.HasSuffix(got, "viewer "+owner) {
		t.Errorf("context = %q", got)
	}
	// R asks again; the roles still fail, and the notice says so.
	r.keys("R")
	if src.asked != 2 || r.status() != "the roles have not loaded; asking again" {
		t.Errorf("asked %d times, status %q", src.asked, r.status())
	}
	// The roles load; a reload then arrives at the first stop.
	src.failing = false
	n := len(src.reqs)
	r.send(ui.ReloadMsg{})
	if src.asked != 3 {
		t.Fatalf("the reload asked %d times in all", src.asked)
	}
	if got := src.last(); len(src.reqs) <= n || got.Role != Owner || got.View != "tableau" {
		t.Errorf("after the reload: %d requests, last %+v", len(src.reqs), got)
	}
	if got := r.context(); !strings.HasSuffix(got, "viewer "+owner+", owner") {
		t.Errorf("context = %q", got)
	}
	// An error after an arrival keeps the stops and sends nothing.
	src.failing = true
	starts := len(r.starts())
	r.send(ui.ReloadMsg{})
	if len(r.starts()) != starts {
		t.Errorf("a failed reload started the grid again")
	}
	src.failing = false
	r.keys("R")
	if got := src.last().Role; got != Assignee {
		t.Errorf("R after a failed reload reads as %q", got)
	}
}

// T9: a reload.
func TestAReload(t *testing.T) {
	r := newRig(t, owner, "", 100, 24)
	starts := len(r.starts())
	r.send(ui.ReloadMsg{})
	if r.spy.asked != 2 {
		t.Fatalf("the roles loaded %d times, want 2", r.spy.asked)
	}
	if len(r.starts()) != starts {
		t.Errorf("a reload with the role holding started the grid")
	}
	// The role in force is lost: the owner no longer holds owner.
	r.spy.roles = []string{"assignee"}
	r.send(ui.ReloadMsg{})
	if got := r.spy.last(); got.Role != Assignee || got.View != "context" {
		t.Errorf("the grid asked %+v", got)
	}
	if got := r.status(); got != "the role owner no longer holds: reading as assignee" {
		t.Errorf("status = %q", got)
	}
	// An answer for an old sequence number changes nothing.
	n, asked := len(r.starts()), r.spy.asked
	r.send(ui.LoadedMsg{Pane: ID, Seq: 1, Value: view.Viewer{Email: owner, Roles: []string{"reviewer"}}})
	if len(r.starts()) != n || r.spy.asked != asked {
		t.Errorf("an old answer started the grid")
	}
	r.keys("R")
	if got := r.spy.last().Role; got != Observer {
		t.Errorf("R reads as %q, want observer: the old answer changed the stops", got)
	}
	// An answer of the wrong type after an arrival changes nothing.
	r.send(ui.LoadedMsg{Pane: ID, Seq: 3, Value: 42})
}

// T10: the pane stays out of sight.
func TestThePaneStaysOutOfSight(t *testing.T) {
	r := newRig(t, owner, "", 100, 24)
	before := r.screen()
	r.keys("tab")
	if r.screen() != before {
		t.Errorf("tab changed the screen")
	}
	p := New(Options{})
	if p.Keys() != nil || p.Status(ui.Context{}) != "" || p.ID() != ID {
		t.Errorf("keys %v, status %q, id %q", p.Keys(), p.Status(ui.Context{}), p.ID())
	}
	for _, l := range strings.Split(p.View(7, 3, ui.Context{}), "\n") {
		if l != strings.Repeat(" ", 7) {
			t.Errorf("view line %q", l)
		}
	}
	g := grid.New(grid.Options{})
	o := ui.Options{Panes: []ui.Pane{g, p}, Commands: []ui.Command{Command()}}
	if err := ui.CheckKeys(o); err != nil {
		t.Errorf("CheckKeys: %v", err)
	}
	r.keys("?")
	help := r.screen()
	if !strings.Contains(help, "Commands") || !regexp.MustCompile(`(?m)^  R +role$`).MatchString(help) {
		t.Errorf("help:\n%s", help)
	}
}

// T11: every line fits.
func TestEveryLineFits(t *testing.T) {
	scripts := []string{"", "R", "R R", "R down down p right right right"}
	for _, script := range scripts {
		for w := 40; w <= 138; w += 7 {
			for h := 8; h <= 38; h += 5 {
				r := newRig(t, owner, "", w, h).keys(script)
				lines := r.content()
				if len(lines) != h {
					t.Fatalf("%q at %dx%d: %d lines", script, w, h, len(lines))
				}
				for i, l := range lines {
					if got := ansi.WcWidth.StringWidth(ansi.Strip(l)); got != w {
						t.Fatalf("%q at %dx%d: line %d is %d cells: %q", script, w, h, i, got, ansi.Strip(l))
					}
				}
			}
		}
	}
}
