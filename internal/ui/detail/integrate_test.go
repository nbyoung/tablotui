package detail

import (
	"errors"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/nbyoung/tablo"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/source/corpustest"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/ui/grid"
	"github.com/nbyoung/tablotui/internal/view/list"
	"github.com/nbyoung/tablotui/internal/watch"
)

// The integrate tests of 0afa ("Later gates"): the tablo adapter behind
// source.Lists proves A1 to A9 on a copy of the corpus entry weather-station
// and on a clone of the umbrella's plan at 3cdae52, the commit the fixtures
// of testdata draw, which TestListFixtures in internal/source proves are the
// adapter's output there; a pane opens at the level tablo resolves for the
// viewer's role (decision 13); and a move of the row cursor with four panes
// open stays under a bound (A5). They skip without TABLEAUX_CORPUS or
// TABLEAUX_DIR.

// umbrellaCommit is the commit of the umbrella the fixtures draw.
const umbrellaCommit = "3cdae527d47d37fdff55bc302c76c017cf91c606"

// project is one repository the integrate tests read through the adapter,
// with the tasks and the viewer each test asks about.
type project struct {
	name   string
	src    *source.Tablo
	root   string // the root task
	parent string // a parent below the root
	leaf   string // a leaf
	viewer string // a person with work in the queue
	tasks  []string
}

// adapter opens the adapter on dir at ref and stores the snapshot of st.
func adapter(t *testing.T, dir, ref string, st watch.State) *source.Tablo {
	t.Helper()
	src, err := source.NewTablo(t.Context(), dir, ref, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := src.Load(t.Context(), st)
	if err != nil {
		t.Fatal(err)
	}
	src.Store(snap)
	return src
}

// withTasks fills the project's task ids from its tableau, in display order.
func withTasks(t *testing.T, p project) project {
	t.Helper()
	tb, err := p.src.Tableau(t.Context(), source.Request{View: "tableau", Window: 1, Level: "detail"})
	var diag *source.DiagnosticError
	if err != nil && !errors.As(err, &diag) {
		t.Fatal(err)
	}
	for _, r := range tb.Rows {
		p.tasks = append(p.tasks, r.ID)
	}
	return p
}

func weatherStation(t *testing.T) project {
	t.Helper()
	src := adapter(t, corpustest.Entry(t, "weather-station"), "", watch.State{})
	return withTasks(t, project{name: "weather-station", src: src, root: "a1c0", parent: "4e2b", leaf: "3c5d", viewer: "ada@example.org"})
}

func umbrellaPlan(t *testing.T) project {
	t.Helper()
	src := adapter(t, corpustest.Umbrella(t, umbrellaCommit), "main", watch.State{Commit: umbrellaCommit})
	return withTasks(t, project{name: "umbrella", src: src, root: "437e", parent: "5fe3", leaf: "e9c6", viewer: "noreply@anthropic.com"})
}

// both runs f on each project as a subtest.
func both(t *testing.T, f func(t *testing.T, p project)) {
	for _, open := range []func(*testing.T) project{weatherStation, umbrellaPlan} {
		p := open(t)
		t.Run(p.name, func(t *testing.T) { f(t, p) })
	}
}

// heads loads every view the panes draw, for the root, the parent and the
// leaf, at each level, and returns the heads by "view task level".
func heads(t *testing.T, p project) map[string]*list.Head {
	t.Helper()
	ctx := t.Context()
	out := map[string]*list.Head{}
	for _, level := range []string{"detail", "provenance"} {
		for _, task := range []string{p.root, p.parent, p.leaf} {
			r := source.ListRequest{Task: task, Level: level, Viewer: p.viewer}
			tk, err := p.src.Task(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			b, err := p.src.Blockage(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			h, err := p.src.History(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			out["task "+task+" "+level], out["blockage "+task+" "+level], out["history "+task+" "+level] = &tk.Head, &b.Head, &h.Head
		}
	}
	// The queue's deepest level is detail (decision 8).
	q, err := p.src.Queue(ctx, source.ListRequest{Level: "detail", Viewer: p.viewer})
	if err != nil {
		t.Fatal(err)
	}
	out["queue - detail"] = &q.Head
	return out
}

// A1: the adapter maps tablo's envelope into the list types, and a
// *tablo.NotFoundError reaches the pane as a sentence a person reads. A
// *tablo.ReadError arises where NewTablo cannot open the repository, before
// any pane: TestNewTabloOutsideARepository and the command's exit 3 prove it.
func TestIntegrateA1Errors(t *testing.T) {
	both(t, func(t *testing.T, p project) {
		var nf *tablo.NotFoundError
		if _, err := p.src.Task(t.Context(), source.ListRequest{Task: "ffff"}); !errors.As(err, &nf) {
			t.Fatalf("an unknown task gives %v", err)
		}
		x := pane(t, TaskID, "testdata", 80, 10).withSource(p.src).open().sel("ffff", "")
		if got := x.view()[1]; !strings.HasPrefix(got, "│ error: ") || !strings.Contains(got, "ffff") || strings.Contains(got, "*tablo") {
			t.Errorf("the pane draws %q", got)
		}
	})
}

// A2: with no viewer the queue is source.ErrNoPerson, and the pane draws its
// sentence; with one, the queue states the viewer as its person (A9).
func TestIntegrateA2NoPerson(t *testing.T) {
	both(t, func(t *testing.T, p project) {
		if _, err := p.src.Queue(t.Context(), source.ListRequest{}); !errors.Is(err, source.ErrNoPerson) {
			t.Fatalf("a queue with no viewer gives %v", err)
		}
		x := pane(t, QueueID, "testdata", 80, 10).withSource(p.src).open()
		if got := x.view()[1]; got != "│ "+noPersonText {
			t.Errorf("the pane draws %q", got)
		}
	})
}

// opensAt is the level each view opens at for a role, as tablo's 4ed9
// command table states it: the task at detail for every role; the queue at
// detail for a contributor, an agent, a reviewer and an authority; the
// blockage at detail for the owner, an authority and a contributor; the
// history at provenance for the owner and at detail for every other role.
var opensAt = map[string]map[string]string{
	TaskID:     {"owner": "detail", "assignee": "detail", "contributor": "detail", "observer": "detail"},
	QueueID:    {"owner": "glance", "assignee": "glance", "contributor": "detail", "observer": "glance"},
	BlockageID: {"owner": "detail", "assignee": "glance", "contributor": "detail", "observer": "glance"},
	HistoryID:  {"owner": "provenance", "assignee": "detail", "contributor": "detail", "observer": "detail"},
}

// Decision 13 and A2: a pane that the viewer has not set with d or p asks
// with no level, and opens at the level tablo resolves for the role the role
// pane names in ui.ReadingMsg: the title states it, and so does the data.
func TestIntegrateOpeningLevelComesFromTablo(t *testing.T) {
	both(t, func(t *testing.T, p project) {
		for _, id := range []string{TaskID, QueueID, BlockageID, HistoryID} {
			for role, want := range opensAt[id] {
				t.Run(id+" "+role, func(t *testing.T) {
					x := pane(t, id, "testdata", 100, 12).withSource(p.src)
					x.send(ui.ReadingMsg{Viewer: p.viewer, Role: role}).open().sel(p.leaf, "")
					reqs := x.requests()
					if len(reqs) == 0 || reqs[len(reqs)-1].Level != "" || reqs[len(reqs)-1].Role != role {
						t.Fatalf("the requests %+v", reqs)
					}
					if got := x.pane().shown().String(); got != want {
						t.Errorf("the pane opens at %s, want %s", got, want)
					}
					if title := x.view()[0]; !strings.Contains(title, " · "+want+" ") {
						t.Errorf("the title %q", title)
					}
				})
			}
		}
	})
}

// A3: every view's data carries its level, project, ref, params and legend,
// and the legend holds a symbol for every gate, state, reason and mark.
func TestIntegrateA3Heads(t *testing.T) {
	both(t, func(t *testing.T, p project) {
		for name, h := range heads(t, p) {
			level := strings.Fields(name)[2]
			if h.Level.String() != level || h.Project.Root != p.root || h.Project.Title == "" ||
				h.Source.Ref != "main" || len(h.Source.Commit) != 40 || h.Source.Date == "" {
				t.Errorf("%s: level %v, project %+v, source %+v", name, h.Level, h.Project, h.Source)
			}
			if task := strings.Fields(name)[1]; task != "-" && h.Params.Task != task {
				t.Errorf("%s: params %+v", name, h.Params)
			}
			if strings.HasPrefix(name, "queue") && h.Params.Person != p.viewer {
				t.Errorf("%s: params %+v", name, h.Params)
			}
			l := h.Legend
			if len(l.Gates) == 0 || len(l.States) == 0 || len(l.Reasons) == 0 || len(l.Marks) == 0 {
				t.Errorf("%s: the legend %+v", name, l)
			}
			for _, s := range append(append(append(gateSymbols(l), stateSymbols(l)...), reasonSymbols(l)...), markSymbols(l)...) {
				if s == "" {
					t.Errorf("%s: a symbol of the legend is empty: %+v", name, l)
					break
				}
			}
		}
	})
}

func gateSymbols(l list.Legend) (out []string) {
	for _, g := range l.Gates {
		out = append(out, g.Symbol)
	}
	return out
}

func stateSymbols(l list.Legend) (out []string) {
	for _, s := range l.States {
		out = append(out, s.Symbol)
	}
	return out
}

func reasonSymbols(l list.Legend) (out []string) {
	for _, r := range l.Reasons {
		out = append(out, r.Symbol)
	}
	return out
}

func markSymbols(l list.Legend) (out []string) {
	for _, m := range l.Marks {
		out = append(out, m.Symbol)
	}
	return out
}

// A4: the levels nest. A pane that holds the provenance data and steps back
// to detail with p draws, with no load, the document it draws from the
// detail data, for every view and every task of the project.
func TestIntegrateA4LevelsNest(t *testing.T) {
	both(t, func(t *testing.T, p project) {
		for _, id := range []string{TaskID, BlockageID, HistoryID} {
			for _, task := range p.tasks {
				detail := pane(t, id, "testdata", 100, 12).withSource(p.src).open().sel(task, "").keys("d")
				if detail.pane().shown() == list.Glance {
					detail.keys("d") // from glance d shows detail
				}
				deep := pane(t, id, "testdata", 100, 12).withSource(p.src).open().sel(task, "").keys("p")
				n := deep.src.count()
				deep.keys("p")
				if deep.src.count() != n || deep.pane().shown() != list.Detail {
					t.Fatalf("%s %s: p back to detail loads or draws %v", id, task, deep.pane().shown())
				}
				if a, b := detail.doc(98), deep.doc(98); a != b {
					t.Errorf("%s %s: detail drawn from provenance differs:\n%s\nfrom detail:\n%s", id, task, b, a)
				}
			}
		}
	})
}

// A6: the task data holds the fields of list.Task: at detail the task, its
// assignee, parent, path, status, description and junctions, a parent's
// children; at provenance the status commit in full, the authorisation, the
// chain and the commands, and across the project the suppliers of the
// junction fields that an entry supplies.
func TestIntegrateA6Task(t *testing.T) {
	both(t, func(t *testing.T, p project) {
		supplied := 0
		for _, task := range p.tasks {
			d, err := p.src.Task(t.Context(), source.ListRequest{Task: task, Level: "provenance"})
			if err != nil {
				t.Fatal(err)
			}
			if d.Task.ID != task || d.Task.Title == "" || d.Assignee == "" || d.Status.Gate == "" || d.Status.State == "" ||
				len(d.Path) == 0 || d.Path[len(d.Path)-1].ID != task || d.Path[0].ID != p.root || len(d.Junctions) == 0 {
				t.Errorf("%s: %+v", task, d.Task)
			}
			if (task == p.root) != (d.Parent == nil) {
				t.Errorf("%s: parent %+v", task, d.Parent)
			}
			if task == p.parent && (len(d.Children) == 0 || len(d.Considered) == 0) {
				t.Errorf("%s: children %d, considered %v", task, len(d.Children), d.Considered)
			}
			if d.StatusCommit == nil || len(d.StatusCommit.Hash) != 40 || d.StatusCommit.Author.Email == "" ||
				d.Authorisation == nil || len(d.Commands) == 0 || len(d.Chain) == 0 {
				t.Errorf("%s: the provenance facts are incomplete: commit %v, authorisation %v, %d commands, %d steps",
					task, d.StatusCommit != nil, d.Authorisation != nil, len(d.Commands), len(d.Chain))
			}
			supplied += len(d.Sources)
		}
		if supplied == 0 {
			t.Error("no task names the supplier of a junction field")
		}
	})
}

// A7: the blockage holds the fields of list.Blockage, the five kinds in
// order, and the task parameter filters it: every cause of a task is a cause
// of the root, and either stands in the task's subtree or holds the task.
func TestIntegrateA7Blockage(t *testing.T) {
	both(t, func(t *testing.T, p project) {
		ctx := t.Context()
		all, err := p.src.Blockage(ctx, source.ListRequest{Task: p.root, Level: "detail"})
		if err != nil {
			t.Fatal(err)
		}
		var kinds []string
		for _, k := range all.Kinds {
			kinds = append(kinds, k.Kind)
		}
		if strings.Join(kinds, " ") != "requirement status review authorisation snapshot" {
			t.Errorf("kinds %v", kinds)
		}
		key := func(c list.Cause) string { return c.Kind + " " + c.Task.ID + " " + c.Gate }
		root := map[string]bool{}
		for _, c := range all.Causes {
			root[key(c)] = true
			if c.Task.ID == "" || c.Gate == "" || c.Act == "" || c.Holds == 0 {
				t.Errorf("a cause of the root is incomplete: %+v", c)
			}
		}
		for _, task := range p.tasks {
			b, err := p.src.Blockage(ctx, source.ListRequest{Task: task, Level: "detail"})
			if err != nil {
				t.Fatal(err)
			}
			path := pathOf(t, p, task)
			for _, c := range b.Causes {
				if !root[key(c)] {
					t.Errorf("%s: %s is no cause of the root", task, key(c))
				}
				if !slices.Contains(pathOf(t, p, c.Task.ID), task) && !holds(c.Held, task) && !slices.Contains(path, c.Task.ID) {
					t.Errorf("%s: %s neither lies in its subtree nor holds it", task, key(c))
				}
			}
		}
	})
}

// pathOf returns the ids from the root to task.
func pathOf(t *testing.T, p project, task string) []string {
	t.Helper()
	d, err := p.src.Task(t.Context(), source.ListRequest{Task: task, Level: "detail"})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range d.Path {
		ids = append(ids, r.ID)
	}
	return ids
}

// holds reports whether task stands in the tree of held tasks.
func holds(held []list.Held, task string) bool {
	for _, h := range held {
		if h.Task.ID == task || holds(h.Held, task) {
			return true
		}
	}
	return false
}

// A8: the history of a parent replays its children's lines; at detail each
// line carries the status after it; at provenance a line names its committer
// exactly where the commit's committer differs from its author, and its
// model is the commit's first Model trailer.
func TestIntegrateA8History(t *testing.T) {
	both(t, func(t *testing.T, p project) {
		h, err := p.src.History(t.Context(), source.ListRequest{Task: p.parent, Level: "provenance"})
		if err != nil {
			t.Fatal(err)
		}
		own, others := 0, 0
		for _, l := range h.Lines {
			if l.Taken() != nil {
				continue
			}
			if l.Task.ID == p.parent {
				own++
			} else {
				others++
			}
			if l.After == nil || l.Commit == nil {
				t.Errorf("line %+v lacks its status after or its commit", l.Task)
				continue
			}
			differs := l.Commit.Committer.Email != l.Commit.Author.Email
			if differs != (l.Committer != "") || differs && l.Committer != l.Commit.Committer.Email {
				t.Errorf("%s %s: committer %q, commit by %s and %s", l.Task.ID, l.Commit.Hash[:7], l.Committer, l.Commit.Author.Email, l.Commit.Committer.Email)
			}
			model := ""
			for _, tr := range l.Commit.Trailers {
				if v, ok := strings.CutPrefix(tr, "Model: "); ok {
					model = v
					break
				}
			}
			if l.Model != model {
				t.Errorf("%s %s: model %q, trailer %q", l.Task.ID, l.Commit.Hash[:7], l.Model, model)
			}
		}
		if own == 0 || others == 0 || h.Subject.ID != p.parent {
			t.Errorf("the history of %s holds %d lines of its own and %d of its children", p.parent, own, others)
		}
	})
}

// A9: the queue holds the fields of list.QueueItem for the viewer, who is its
// person; each item names a task of the tableau and a gate of the legend,
// and at detail a ready item names its next junction.
func TestIntegrateA9Queue(t *testing.T) {
	both(t, func(t *testing.T, p project) {
		q, err := p.src.Queue(t.Context(), source.ListRequest{Level: "detail", Viewer: p.viewer})
		if err != nil {
			t.Fatal(err)
		}
		if q.Params.Person != p.viewer || len(q.Items) == 0 {
			t.Fatalf("the queue of %s: %+v, %d items", p.viewer, q.Params, len(q.Items))
		}
		gates := map[string]bool{}
		for _, g := range q.Legend.Gates {
			gates[g.Key] = true
		}
		for _, it := range q.Items {
			if !slices.Contains([]string{"review", "authorisation", "ready", "reaffirmation", "waiting"}, it.Kind) ||
				!slices.Contains(p.tasks, it.Task.ID) || it.Task.Title == "" || !gates[it.Gate] {
				t.Errorf("item %+v", it)
			}
			if it.Kind == "ready" && it.Junction == nil && it.Gate != q.Legend.Gates[len(q.Legend.Gates)-1].Key {
				t.Errorf("a ready item names no junction: %+v", it)
			}
			if it.Kind == "waiting" && len(it.Causes) == 0 {
				t.Errorf("a waiting item names no cause: %+v", it)
			}
		}
	})
}

// moveBound bounds the median move of the row cursor with four panes open:
// the grid's selection, three views derived from the stored snapshot and
// decoded, and one frame drawn. A move costs less than the one load per
// settled change that 171b's A8 bounds at 100 ms, and a keystroke answered
// within it reads as immediate. A plain run takes about 3 ms on the
// umbrella's plan and one under the race detector about 25 ms.
const moveBound = 100 * time.Millisecond

// A5: with the four panes open, one move of the row cursor derives three
// views from the stored snapshot, loads nothing, and stays under moveBound
// on the umbrella's plan. The test logs each project's times.
func TestIntegrateA5CursorMove(t *testing.T) {
	both(t, func(t *testing.T, p project) {
		src := &stub{next: p.src}
		set := New(Options{Source: src})
		g := grid.New(grid.Options{Source: p.src, Start: grid.GlanceStart()})
		m := ui.New(ui.Options{
			Panes:    append([]ui.Pane{g}, set.Panes...),
			Commands: set.Commands,
			Layout:   set.Layout,
			Styles:   ui.DefaultStyles(),
		})
		r := &rig{t: t, m: m, src: src}
		r.send(tea.WindowSizeMsg{Width: 160, Height: 45})
		r.run(m.Init())
		r.send(ui.ReadingMsg{Viewer: p.viewer})
		r.keys("E 1 2 3 4 esc")
		if !strings.Contains(r.screen(), "4 History") || strings.Contains(r.screen(), "error:") {
			t.Fatalf("the four panes do not stand:\n%s", r.screen())
		}
		var times []time.Duration
		for range len(p.tasks) - 1 {
			n := src.count()
			start := time.Now()
			r.keys("down")
			_ = r.m.(ui.Model).View()
			times = append(times, time.Since(start))
			if got := src.count() - n; got != 3 {
				t.Fatalf("a move asks %d views, want 3", got)
			}
		}
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		median, worst := times[len(times)/2], times[len(times)-1]
		t.Logf("%s: %d moves with four panes open: median %v, worst %v, bound %v", p.name, len(times), median, worst, moveBound)
		if median > moveBound {
			t.Errorf("a move takes %v at the median, over %v", median, moveBound)
		}
	})
}
