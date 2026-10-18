package act

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nbyoung/tabloio/action"

	"github.com/nbyoung/tablotui/internal/ui"
)

// grapheme switches the frame to the grapheme method, as a terminal that
// reports the Unicode core mode does.
func (r *rig) grapheme() *rig {
	r.send(tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeSet})
	return r
}

// failedCommit is the hook's failure of the golden failed-80x16.txt.
var failedCommit = &action.ExecError{Tool: "git", Args: []string{"commit", "--cleanup=verbatim"}, Exit: 1,
	Stderr: "pre-commit: tablo validate reports 1 error\n.tableaux/status/9f31.yaml:1:1: error: S11 the status passes design without a review\n"}

// T1: the eight goldens of fixtures.md, under both methods, each line
// exactly the width.
func TestGoldens(t *testing.T) {
	unreviewed := refusal("unreviewed", "9f31", "design", "S11", "the status passes design, which ben@example.org reviews and has not accepted; record the reason review to hand the work off")
	cases := []struct {
		golden string
		w, h   int
		edit   func(o *Options)
		repo   func(r *fakeRepo)
		drive  func(r *rig)
	}{
		{"form-80x16.txt", 80, 16, nil, nil, func(r *rig) { r.selects("9f31", "design").keys("r") }},
		{"form-refused-80x16.txt", 80, 16, nil,
			func(f *fakeRepo) { f.record = func(action.Recording) (*action.Plan, error) { return nil, unreviewed } },
			func(r *rig) {
				r.selects("9f31", "design").keys("r down down down").text("The boards arrive").keys("enter")
			}},
		{"form-note-60x12.txt", 60, 12, nil, nil, func(r *rig) { r.selects("9f31", "").keys("r down down down") }},
		{"form-held-40x8.txt", 40, 8, nil, nil, func(r *rig) { r.selects("c07d", "").keys("r") }},
		{"confirm-80x24.txt", 80, 24, nil, nil, func(r *rig) {
			r.selects("9f31", "design").keys("r down down down").text("The boards arrive and pass the bench test").keys("enter")
		}},
		{"confirm-warning-60x10.txt", 60, 10,
			func(o *Options) {
				o.Viewer = "noreply@anthropic.com"
				o.Uncommitted = func(context.Context, string) ([]string, error) { return []string{".tableaux/tasks/c07d.yaml"}, nil }
			},
			func(f *fakeRepo) {
				f.reaffirm = []action.Warning{{Code: "not-contributor", Task: "9f31", Text: "ada@example.org is neither the contributor at design nor the assignee"}}
			},
			func(r *rig) { r.selects("9f31", "").keys("a") }},
		{"refused-80x12.txt", 80, 12, nil,
			func(f *fakeRepo) {
				f.reviewErr = refusal("wrong-hand", "9f31", "design", "H2", "ben@example.org reviews this junction, and the commit is ada@example.org's")
			},
			func(r *rig) { r.selects("9f31", "design").keys("v") }},
		{"failed-80x16.txt", 80, 16, nil,
			func(f *fakeRepo) { f.applyErr = failedCommit },
			func(r *rig) { r.selects("9f31", "").keys("u y") }},
	}
	for _, c := range cases {
		for _, method := range []ansi.Method{ansi.WcWidth, ansi.GraphemeWidth} {
			t.Run(fmt.Sprintf("%s/%d", c.golden, method), func(t *testing.T) {
				r := newRig(t, c.w, c.h, c.edit)
				if method == ansi.GraphemeWidth {
					r.grapheme()
				}
				if c.repo != nil {
					c.repo(r.repo)
				}
				c.drive(r)
				r.golden(c.golden)
				r.widths(c.w, c.h, method)
			})
		}
	}
}

// T2: the notices before a plan, each on the status line, with no plan call;
// a nil Repo answers all four keys.
func TestNoticesBeforeAPlan(t *testing.T) {
	for _, k := range []string{"r", "v", "a", "u"} {
		r := newRig(t, 80, 16, func(o *Options) { o.Repo = nil }).selects("9f31", "design").keys(k)
		if got := r.status(); got != "no action: this build draws fixtures" {
			t.Errorf("%s with no Repo: %q", k, got)
		}
	}
	uncommitted := func(paths []string, err error) func(o *Options) {
		return func(o *Options) {
			o.Uncommitted = func(context.Context, string) ([]string, error) { return paths, err }
		}
	}
	cases := []struct {
		name, task, gate, key string
		edit                  func(o *Options)
		facts                 error
		want                  string
	}{
		{"ref", "9f31", "design", "a", func(o *Options) { o.Ref = "HEAD" }, nil, "--ref HEAD shows a commit; nothing writes there"},
		{"no task", "", "", "u", nil, nil, "select a task first"},
		{"no gate", "9f31", "", "v", nil, nil, "move the cursor to the gate you review"},
		{"git fails", "9f31", "", "a", uncommitted(nil, errors.New("git diff: fatal: not a git repository\nmore")), nil, "error: git diff: fatal: not a git repository"},
		{"status file", "9f31", "", "r", uncommitted([]string{".tableaux/status/9f31.yaml", ".tableaux/tasks/9f31.yaml"}, nil), nil, "status/9f31.yaml holds an uncommitted edit; commit or restore it"},
		{"task file", "9f31", "design", "v", uncommitted([]string{".tableaux/gates.yaml", ".tableaux/tasks/9f31.yaml"}, nil), nil, "tasks/9f31.yaml holds an uncommitted edit; commit or restore it"},
		{"facts fail", "9f31", "", "r", nil, errors.New("tablo: no view\nmore"), "error: tablo: no view"},
		{"no such task", "ffff", "", "r", nil, nil, "error: task ffff: no such task"},
		{"a parent", "07e0", "", "r", nil, nil, "07e0 is a parent; its children hold the status"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, 80, 16, c.edit)
			r.facts.err = c.facts
			r.selects(c.task, c.gate).keys(c.key)
			if got := r.status(); got != c.want {
				t.Errorf("status %q, want %q", got, c.want)
			}
			if len(r.repo.calls) != 0 || r.modeOpen() {
				t.Errorf("calls %v, mode open %v", r.repo.calls, r.modeOpen())
			}
		})
	}
}

// T3: the empty commits: the call, the pager, enter and y.
func TestEmptyCommits(t *testing.T) {
	for _, c := range []struct {
		key, method, gate, subject string
	}{
		{"v", "Review", "design", "Accept sensor board (9f31) at design"},
		{"a", "Reaffirm", "", "Reaffirm sensor board (9f31)"},
		{"u", "Authorise", "", "Authorise sensor board (9f31)"},
	} {
		t.Run(c.method, func(t *testing.T) {
			r := newRig(t, 80, 24, nil).selects("9f31", "design").keys(c.key)
			got := r.repo.last()
			if got.method != c.method || !slices.Equal(got.ids, []string{"9f31"}) || got.gate != c.gate || got.o != (action.Options{}) {
				t.Errorf("the call: %+v", got)
			}
			var p *action.Plan
			switch c.key {
			case "v":
				p, _ = r.repo.Review(context.Background(), []string{"9f31"}, "design", action.Options{})
			case "a":
				p, _ = r.repo.Reaffirm(context.Background(), []string{"9f31"}, action.Options{})
			default:
				p, _ = r.repo.Authorise(context.Background(), []string{"9f31"}, action.Options{})
			}
			want := strings.Split(strings.TrimSuffix(p.String(), "\n"), "\n")
			if l := r.lines(); !slices.Equal(l[1:1+len(want)], want) {
				t.Errorf("the pager:\n%s\nwant:\n%s", strings.Join(l, "\n"), strings.Join(want, "\n"))
			}
			r.keys("enter enter")
			if r.repo.count("Apply") != 0 || !r.modeOpen() {
				t.Fatal("enter commits")
			}
			r.keys("y")
			if r.repo.count("Apply") != 1 || r.pokes != 1 || r.reloads != 1 {
				t.Errorf("Apply %d, Poke %d, reloads %d", r.repo.count("Apply"), r.pokes, r.reloads)
			}
			if got := r.status(); got != "committed 0881b37 "+c.subject {
				t.Errorf("status %q", got)
			}
			if r.modeOpen() {
				t.Error("the mode stays open")
			}
		})
	}
}

// T4: n, q and esc at the confirmation commit nothing and close the mode.
func TestCancel(t *testing.T) {
	for _, k := range []string{"n", "q", "esc"} {
		r := newRig(t, 80, 24, nil).selects("9f31", "").keys("a " + k)
		if r.repo.count("Apply") != 0 || r.reloads != 0 || r.modeOpen() {
			t.Errorf("%s: Apply %d, reloads %d, open %v", k, r.repo.count("Apply"), r.reloads, r.modeOpen())
		}
		want := "not committed"
		if k == "esc" {
			want = ""
		}
		if got := r.status(); got != want {
			t.Errorf("%s: status %q", k, got)
		}
	}
}

// T8: a refusal in the form, then a plan, then the commit.
func TestRefusalInTheForm(t *testing.T) {
	r := newRig(t, 80, 16, nil)
	refuse := true
	r.repo.record = func(rec action.Recording) (*action.Plan, error) {
		if refuse {
			return nil, refusal("unreviewed", "9f31", "design", "S11", "the reviewer has not accepted design")
		}
		p := plan("record", rec.ID, "Record sensor board (9f31) at design")
		p.Diff = diff9f31
		return p, nil
	}
	r.selects("9f31", "design").keys("r enter")
	if l := r.lines(); l[10] != "  refused:" || l[11] != "  9f31 design: the reviewer has not accepted design (unreviewed, S11)" {
		t.Errorf("the form:\n%s", r.screen())
	}
	refuse = false
	r.keys("enter")
	if l := r.lines(); l[len(l)-3] != promptLine || strings.Contains(r.screen(), "refused:") {
		t.Errorf("the confirmation:\n%s", r.screen())
	}
	r.keys("y")
	if r.repo.count("Apply") != 1 || r.status() != "committed 0881b37 Record sensor board (9f31) at design" {
		t.Errorf("y: Apply %d, %q", r.repo.count("Apply"), r.status())
	}
}

// T9: a plan that stops: a refusal on v gives the stopped screen, refused;
// an ExecError and a plain error give no plan, for v and inside the form.
func TestPlanThatStops(t *testing.T) {
	exec := &action.ExecError{Tool: "tablo", Args: []string{"--ref", "HEAD"}, Exit: -1, Stderr: "no view data"}
	for _, c := range []struct {
		err  error
		head string
		line string
	}{
		{refusal("no-reviewer", "9f31", "design", "", "the junction states no reviewer"), "review 9f31 design: refused", "9f31 design: the junction states no reviewer (no-reviewer)"},
		{exec, "review 9f31 design: no plan", "tablo --ref"},
		{errPlain, "review 9f31 design: no plan", "the plain error"},
	} {
		r := newRig(t, 80, 16, nil)
		r.repo.reviewErr = c.err
		r.selects("9f31", "design").keys("v")
		if l := r.lines(); l[1] != c.head || l[3] != c.line {
			t.Errorf("v:\n%s", r.screen())
		}
		if r.lines()[len(r.lines())-1] != "enter close  ↑↓ scroll  ? help  q quit" {
			t.Errorf("help %q", r.lines()[len(r.lines())-1])
		}
		r.keys("y")
		if !r.modeOpen() || r.repo.count("Apply") != 0 {
			t.Error("y acts on a stopped plan")
		}
		r.keys("enter")
		if r.modeOpen() {
			t.Error("enter does not close")
		}
	}
	for _, err := range []error{exec, errPlain} {
		r := newRig(t, 80, 16, nil)
		r.repo.record = func(action.Recording) (*action.Plan, error) { return nil, err }
		r.selects("9f31", "").keys("r enter")
		if l := r.lines(); l[10] != "  no plan:" || !strings.HasPrefix(l[11], "  "+errLines(err)[0]) {
			t.Errorf("the form:\n%s", r.screen())
		}
	}
}

// T10: what Apply returns: the head, the notice, the closing line, one
// reload each; y applies nothing more and enter closes.
func TestWhatApplyReturns(t *testing.T) {
	stale := action.Refusal{Code: "stale", Text: "HEAD stands at 1111111, not at 3cdae52; plan again"}
	altered := action.Refusal{Code: "altered", Text: "a hook changed the tree of commit 0881b37 from the planned 2222222"}
	for _, c := range []struct {
		name   string
		res    *action.Result
		err    error
		head   string
		notice string
		last   string
	}{
		{"stale", nil, stale, "reaffirm 9f31: refused", "error: not committed: HEAD stands at 1111111, not at 3cdae52; plan again (stale)", "nothing is written; close this and press the key again"},
		{"altered", &action.Result{Commit: commit}, altered, "reaffirm 9f31: committed 0881b37, and a hook altered the commit", "error: committed 0881b37, and a hook altered the commit", altered.Error()},
		{"hook", nil, failedCommit, "reaffirm 9f31: not committed", "error: not committed: git commit: exit 1", "the checkout stands as it stood before the commit"},
		{"plain", nil, errPlain, "reaffirm 9f31: not committed", "error: not committed: the plain error", "the checkout stands as it stood before the commit"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, 100, 20, nil)
			r.repo.applyRes, r.repo.applyErr = c.res, c.err
			r.selects("9f31", "").keys("a y")
			l := r.lines()
			if l[1] != c.head || r.status() != c.notice || r.reloads != 1 || r.pokes != 1 {
				t.Errorf("head %q, notice %q, reloads %d, pokes %d", l[1], r.status(), r.reloads, r.pokes)
			}
			if !slices.Contains(l, c.last) {
				t.Errorf("no line %q:\n%s", c.last, r.screen())
			}
			r.keys("y")
			if r.repo.count("Apply") != 1 {
				t.Error("y applies again")
			}
			r.keys("enter")
			if r.modeOpen() {
				t.Error("enter does not close")
			}
		})
	}
}

// T11: esc and a quit during a commit: the commit runs on, under a live
// context, and Wait waits for it.
func TestEscAndQuitDuringACommit(t *testing.T) {
	r := newRig(t, 80, 24, nil)
	r.selects("9f31", "").keys("a")
	r.hold = true
	r.keys("y")
	if r.held == nil || r.status() != "reaffirm 9f31 · committing…" {
		t.Fatalf("held %v, status %q", r.held != nil, r.status())
	}
	if l := r.lines(); l[len(l)-1] != "? help  q quit" {
		t.Errorf("help line %q", l[len(l)-1])
	}
	r.keys("y n enter")
	if r.status() != "reaffirm 9f31 · committing…" || r.repo.count("Apply") != 0 {
		t.Errorf("a key acts while the commit runs: %q", r.status())
	}
	r.keys("esc")
	if r.modeOpen() {
		t.Error("esc does not close the mode")
	}
	r.cancel()
	r.repo.release = make(chan struct{})
	r.repo.applied = make(chan struct{})
	done := make(chan struct{})
	go func() {
		<-r.repo.applied
		if !r.acts.Busy() {
			t.Error("Busy is false while Apply runs")
		}
		close(r.repo.release)
		close(done)
	}()
	r.release()
	<-done
	r.acts.Wait()
	if c := r.repo.last(); c.method != "Apply" || c.ctxErr != nil {
		t.Errorf("Apply ran under %v", c.ctxErr)
	}
	if r.reloads != 1 || r.status() != "committed 0881b37 Reaffirm sensor board (9f31)" || r.acts.Busy() {
		t.Errorf("reloads %d, status %q, busy %v", r.reloads, r.status(), r.acts.Busy())
	}
}

// T12: the plan of a form closed with esc arrives at a new form of the same
// task and changes nothing.
func TestAnOldAnswer(t *testing.T) {
	r := newRig(t, 80, 16, nil).selects("9f31", "")
	r.keys("r")
	r.hold = true
	r.keys("enter")
	old := r.held
	r.held = nil
	r.keys("esc r")
	before := r.screen()
	r.run(old)
	if got := r.screen(); got != before || !strings.HasPrefix(r.status(), "record 9f31 · gate") {
		t.Errorf("the old plan reached the new form:\n%s", got)
	}
}

// T13: the pager wraps and scrolls, clamped.
func TestPager(t *testing.T) {
	r := newRig(t, 60, 10, nil).selects("9f31", "design").keys("r enter")
	for _, c := range []struct{ keys, want string }{
		{"", "lines 1–5 of 19"},
		{"down", "lines 2–6 of 19"},
		{"G", "lines 15–19 of 19"},
		{"pgdown", "lines 15–19 of 19"},
		{"up", "lines 14–18 of 19"},
		{"g", "lines 1–5 of 19"},
		{"k", "lines 1–5 of 19"},
		{"space end pgup", "lines 10–14 of 19"},
		{"home j", "lines 2–6 of 19"},
	} {
		r.keys(c.keys)
		if got := r.status(); got != "record 9f31 · "+c.want {
			t.Errorf("%q: %q", c.keys, got)
		}
	}
	r.keys("g")
	l := r.lines()
	if l[1] != "record 9f31 on main at 3cdae52" || l[6] != strings.Repeat("─", 60) || l[7] != promptLine {
		t.Errorf("the confirmation:\n%s", r.screen())
	}
	r.keys("g j j j j j j")
	if l := r.lines(); l[1] != "diff --git a/.tableaux/status/9f31.yaml" || l[2] != "b/.tableaux/status/9f31.yaml" {
		t.Errorf("the wrap:\n%s", r.screen())
	}
}

// scripts are the key scripts of T14 and T17: the form, its note and its
// messages, the confirmation with every line of its footer, and the stopped
// screen.
var scripts = []struct {
	name, sel, gate, keys string
	repo                  func(f *fakeRepo)
}{
	{"form", "9f31", "design", "r", nil},
	{"note", "9f31", "", "r down down down home right", nil},
	{"held", "c07d", "", "r", nil},
	{"refused", "9f31", "", "r enter", func(f *fakeRepo) {
		f.record = func(action.Recording) (*action.Plan, error) {
			return nil, action.Refusals{
				{Code: "unreviewed", Task: "9f31", Gate: "design", Rule: "S11", Text: "the reviewer ben@example.org has not accepted design ⚙️ 🧱; hand the work off"},
				{Code: "bad-field", Text: "a reason and no reason at once"},
				{Code: "no-reviewer", Gate: "unit", Text: "the reason review needs a reviewer at the next junction, and none stands there"},
			}
		}
	}},
	{"confirm", "9f31", "design", "r enter end", nil},
	{"warnings", "9f31", "", "a", func(f *fakeRepo) {
		f.reaffirm = []action.Warning{{Code: "not-contributor", Text: "⚙️ the hand is not the contributor"}, {Code: "model-missing", Text: "🧱 no Model:"}}
	}},
	{"stopped", "9f31", "", "u y down", func(f *fakeRepo) {
		f.applyErr = &action.ExecError{Tool: "git", Args: []string{"commit"}, Exit: 1, Stderr: "pre-commit ⚙️: tablo validate 🧱 reports an error at .tableaux/status/9f31.yaml whose line is long\nsecond line\n"}
	}},
}

// T14: every line fits, over every size from 40×8 to 120×30, under both
// methods.
func TestEveryLineFits(t *testing.T) {
	sizes := 0
	for _, s := range scripts {
		for w := 40; w <= 120; w += 7 {
			for h := 8; h <= 30; h++ {
				for _, method := range []ansi.Method{ansi.WcWidth, ansi.GraphemeWidth} {
					r := newRig(t, w, h, func(o *Options) {
						o.Viewer = "noreply@anthropic.com"
						o.Uncommitted = func(context.Context, string) ([]string, error) { return []string{".tableaux/gates.yaml"}, nil }
					})
					if method == ansi.GraphemeWidth {
						r.grapheme()
					}
					if s.repo != nil {
						s.repo(r.repo)
					}
					r.selects(s.sel, s.gate).keys(s.keys)
					if !r.modeOpen() {
						t.Fatalf("%s at %d×%d: no mode:\n%s", s.name, w, h, r.screen())
					}
					r.widths(w, h, method)
					sizes++
				}
			}
		}
	}
	t.Logf("%d screens", sizes)
}

// T16: the four commands clash with no key of the frame, and the help mode
// lists them under Commands.
func TestKeysAndHelp(t *testing.T) {
	r := newRig(t, 80, 30, nil)
	cs := r.acts.Commands()
	if err := ui.CheckKeys(ui.Options{Panes: []ui.Pane{home{new(int)}}, Commands: cs}); err != nil {
		t.Error(err)
	}
	var keys []string
	for _, c := range cs {
		keys = append(keys, c.Binding.Help().Key+" "+c.Binding.Help().Desc)
	}
	if want := []string{"r record a status", "v review the gate", "a reaffirm the status", "u authorise the task"}; !slices.Equal(keys, want) {
		t.Errorf("the commands: %q", keys)
	}
	r.keys("?")
	screen := r.screen()
	at := strings.Index(screen, "Commands")
	if at < 0 {
		t.Fatalf("no Commands section:\n%s", screen)
	}
	for _, want := range []string{"r record a status", "v review the gate", "a reaffirm the status", "u authorise the task"} {
		if !strings.Contains(strings.Join(strings.Fields(screen[at:]), " "), want) {
			t.Errorf("the help lacks %q:\n%s", want, screen)
		}
	}
}

// T17: with the default styles the cursor cell of the note carries reverse
// and the status line states the cursor; with the zero styles the view
// holds no escape sequence.
func TestColourAndItsAbsence(t *testing.T) {
	r := newRig(t, 60, 12, nil).selects("9f31", "").keys("r down down down")
	if l := r.content()[8]; !strings.Contains(l, "\x1b[7m") {
		t.Errorf("the cursor cell carries no reverse: %q", l)
	}
	if r.status() != "record 9f31 · note · cursor 51 of 51" {
		t.Errorf("status %q", r.status())
	}
	for _, s := range scripts {
		r := newRig(t, 80, 24, func(o *Options) { o.Viewer = "noreply@anthropic.com" })
		r.m = ui.New(ui.Options{Panes: []ui.Pane{home{&r.reloads}}, Commands: r.acts.Commands()})
		r.send(tea.WindowSizeMsg{Width: 80, Height: 24})
		if s.repo != nil {
			s.repo(r.repo)
		}
		r.selects(s.sel, s.gate).keys(s.keys)
		view := r.m.(ui.Model).View().Content
		if strings.Contains(view, "\x1b") || view != ansi.Strip(view) {
			t.Errorf("%s: the zero styles draw an escape sequence", s.name)
		}
	}
}
