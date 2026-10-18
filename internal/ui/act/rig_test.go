package act

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nbyoung/tabloio/action"

	"github.com/nbyoung/tablotui/internal/ui"
)

// The compile-time assertions of A1 and A2: the package's own types satisfy
// the two interfaces.
var (
	_ Repo  = (*action.Repo)(nil)
	_ Facts = &action.Tablo{Dir: "."}
)

// The facts of fixtures.md.
const (
	ada      = "ada@example.org"
	ben      = "ben@example.org"
	base     = "3cdae52f0a1b2c3d4e5f60718293a4b5c6d7e8f9"
	commit   = "0881b37c2f9d4e1a8b7c6d5e4f3a2b1c0d9e8f7a"
	note9f31 = "The supplier slips the prototype boards to November"
	headline = "Weather station · working tree on main at 3cdae52"
	diff9f31 = `diff --git a/.tableaux/status/9f31.yaml b/.tableaux/status/9f31.yaml
index 49edd55..0881b37 100644
--- a/.tableaux/status/9f31.yaml
+++ b/.tableaux/status/9f31.yaml
@@ -1,4 +1,3 @@
-gate: function
-state: at_risk
-reason: blocked
-note: The supplier slips the prototype boards to November
+gate: design
+state: nominal
+note: The boards arrive and pass the bench test
`
)

var gates = []string{"undefined", "defined", "mockup", "function", "design", "implementation", "unit", "integrate", "validate", "release"}

// fakeFacts implements Facts over the project and the tasks of fixtures.md.
type fakeFacts struct {
	project action.Project
	tasks   map[string]action.Task
	err     error // every call fails with it
	calls   int
}

func newFacts() *fakeFacts {
	junctions := func(set ...action.Junction) []action.Junction {
		var js []action.Junction
		for _, g := range gates[1:] {
			j := action.Junction{Gate: g, Kind: "plain", Contributor: ada}
			for _, s := range set {
				if s.Gate == g {
					j = s
				}
			}
			js = append(js, j)
		}
		return js
	}
	return &fakeFacts{
		project: action.Project{
			Gates:   gates,
			States:  []string{"undefined", "nominal", "at_risk", "stalled", "complete"},
			Reasons: []string{"overloaded", "blocked", "review"},
			Trunk:   "main",
		},
		tasks: map[string]action.Task{
			"9f31": {ID: "9f31", Title: "Sensor board", Assignee: ada,
				Junctions: junctions(action.Junction{Gate: "design", Kind: "plain", Contributor: ada, Reviewer: ben}),
				Status:    &action.Status{Gate: "function", State: "at_risk", Reason: "blocked", Note: note9f31}},
			"c07d": {ID: "c07d", Title: "Node firmware", Assignee: ada,
				Junctions: junctions(action.Junction{Gate: "mockup", Kind: "not-applicable"},
					action.Junction{Gate: "implementation", Kind: "recursive", URL: "firmware", Submodule: true}),
				Status: &action.Status{Gate: "design"}},
			"b618": {ID: "b618", Title: "Write commands", Assignee: ada, Junctions: junctions()},
			"e0aa": {ID: "e0aa", Title: "Enclosure", Assignee: ada, Junctions: junctions(),
				Status: &action.Status{Gate: "validate", State: "nominal"}},
			"07e0": {ID: "07e0", Title: "Sensor node", Assignee: ada, Junctions: junctions(),
				Children: []action.Child{{ID: "9f31", Order: 1}}},
		},
	}
}

func (f *fakeFacts) Project(context.Context) (action.Project, error) {
	f.calls++
	return f.project, f.err
}

func (f *fakeFacts) Task(_ context.Context, id string) (action.Task, error) {
	f.calls++
	if f.err != nil {
		return action.Task{}, f.err
	}
	t, ok := f.tasks[id]
	if !ok {
		return action.Task{}, fmt.Errorf("task %s: no such task", id)
	}
	return t, nil
}

// call is one call of fakeRepo.
type call struct {
	method string
	ids    []string
	gate   string
	rec    action.Recording
	o      action.Options
	ctxErr error // the context's error when Apply ran
}

// fakeRepo implements Repo with the plans of fixtures.md and records every
// call. Each error field makes its method fail; record, when set, answers
// Record in place of the plan.
type fakeRepo struct {
	mu        sync.Mutex
	calls     []call
	record    func(rec action.Recording) (*action.Plan, error)
	reviewErr error
	reaffirm  []action.Warning
	planErr   error // Reaffirm and Authorise fail with it
	applyRes  *action.Result
	applyErr  error
	release   chan struct{} // when set, Apply waits for it
	applied   chan struct{} // when set, Apply closes it on entry
}

func (r *fakeRepo) add(c call) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, c)
}

// count returns the calls of a method.
func (r *fakeRepo) count(method string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.calls {
		if c.method == method {
			n++
		}
	}
	return n
}

// last returns the last call.
func (r *fakeRepo) last() call {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return call{}
	}
	return r.calls[len(r.calls)-1]
}

func plan(command, id, subject string, trailers ...action.Trailer) *action.Plan {
	who := action.Identity{Name: "Ada Lovelace", Email: ada}
	return &action.Plan{Command: command, Tasks: []string{id}, Branch: "main", Base: base,
		Author: who, Committer: who, Subject: subject, Trailers: trailers}
}

func (r *fakeRepo) Record(_ context.Context, rec action.Recording, o action.Options) (*action.Plan, error) {
	r.add(call{method: "Record", ids: []string{rec.ID}, rec: rec, o: o})
	if r.record != nil {
		return r.record(rec)
	}
	p := plan("record", rec.ID, "Record sensor board ("+rec.ID+") at "+rec.Gate)
	p.Diff = diff9f31
	return p, nil
}

func (r *fakeRepo) Review(_ context.Context, ids []string, gate string, o action.Options) (*action.Plan, error) {
	r.add(call{method: "Review", ids: ids, gate: gate, o: o})
	if r.reviewErr != nil {
		return nil, r.reviewErr
	}
	return plan("review", ids[0], "Accept sensor board ("+ids[0]+") at "+gate, action.Trailer{Key: "Reviewed", Value: ids[0] + " " + gate}), nil
}

func (r *fakeRepo) Reaffirm(_ context.Context, ids []string, o action.Options) (*action.Plan, error) {
	r.add(call{method: "Reaffirm", ids: ids, o: o})
	if r.planErr != nil {
		return nil, r.planErr
	}
	p := plan("reaffirm", ids[0], "Reaffirm sensor board ("+ids[0]+")", action.Trailer{Key: "Reaffirmed", Value: ids[0]})
	p.Warnings = r.reaffirm
	return p, nil
}

func (r *fakeRepo) Authorise(_ context.Context, ids []string, o action.Options) (*action.Plan, error) {
	r.add(call{method: "Authorise", ids: ids, o: o})
	if r.planErr != nil {
		return nil, r.planErr
	}
	return plan("authorise", ids[0], "Authorise sensor board ("+ids[0]+")", action.Trailer{Key: "Authorised", Value: ids[0]}), nil
}

func (r *fakeRepo) Apply(ctx context.Context, _ *action.Plan) (action.Result, error) {
	if r.applied != nil {
		close(r.applied)
	}
	if r.release != nil {
		<-r.release
	}
	r.add(call{method: "Apply", ctxErr: ctx.Err()})
	if r.applyRes != nil || r.applyErr != nil {
		res := action.Result{}
		if r.applyRes != nil {
			res = *r.applyRes
		}
		return res, r.applyErr
	}
	return action.Result{Commit: commit}, nil
}

// home is the stub home pane: it draws nothing, supplies the context line
// and counts each ReloadMsg.
type home struct{ reloads *int }

func (h home) ID() string              { return "home" }
func (h home) Init(ui.Context) tea.Cmd { return nil }
func (h home) View(w, n int, _ ui.Context) string {
	return strings.Repeat("\n", max(0, n-1))
}
func (h home) Status(ui.Context) string   { return "" }
func (h home) Keys() []key.Binding        { return nil }
func (h home) Headline(ui.Context) string { return headline }
func (h home) Update(msg tea.Msg, _ ui.Context) (ui.Pane, tea.Cmd) {
	if _, ok := msg.(ui.ReloadMsg); ok {
		*h.reloads++
	}
	return h, nil
}

// rig drives the frame with the actions as its commands: a rig runs each
// returned command and feeds its message back, and holds the next command
// a key returns when hold is set.
type rig struct {
	t       *testing.T
	m       tea.Model
	repo    *fakeRepo
	facts   *fakeFacts
	acts    *Actions
	reloads int
	pokes   int
	hold    bool
	held    tea.Cmd
	cancel  context.CancelFunc
}

// newRig builds the frame at w×h with the stub home pane and the actions
// over the fakes; edit changes the options before New.
func newRig(t *testing.T, w, h int, edit func(o *Options)) *rig {
	t.Helper()
	r := &rig{t: t, repo: &fakeRepo{}, facts: newFacts()}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	r.cancel = cancel
	o := Options{
		Repo: r.repo, Facts: r.facts, Dir: "project", Context: ctx,
		Uncommitted: func(context.Context, string) ([]string, error) { return nil, nil },
		Poke:        func() { r.pokes++ },
	}
	if edit != nil {
		edit(&o)
	}
	r.acts = New(o)
	r.m = ui.New(ui.Options{Panes: []ui.Pane{home{&r.reloads}}, Commands: r.acts.Commands(), Styles: ui.DefaultStyles()})
	r.send(tea.WindowSizeMsg{Width: w, Height: h})
	return r
}

func (r *rig) send(msg tea.Msg) {
	r.t.Helper()
	m, cmd := r.m.Update(msg)
	r.m = m
	if r.hold && cmd != nil {
		r.held, r.hold = cmd, false
		return
	}
	r.run(cmd)
}

// run executes a command and feeds its message back, as the program does.
func (r *rig) run(cmd tea.Cmd) {
	r.t.Helper()
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			r.run(c)
		}
	default:
		r.send(msg)
	}
}

// release runs the held command.
func (r *rig) release() {
	r.t.Helper()
	cmd := r.held
	r.held = nil
	r.run(cmd)
}

// selects selects a task, and the gate when it is not empty.
func (r *rig) selects(task, gate string) *rig {
	r.send(ui.SelectionMsg{Task: task, Gate: gate})
	return r
}

var named = map[string]tea.KeyPressMsg{
	"up":        {Code: tea.KeyUp},
	"down":      {Code: tea.KeyDown},
	"left":      {Code: tea.KeyLeft},
	"right":     {Code: tea.KeyRight},
	"enter":     {Code: tea.KeyEnter},
	"pgup":      {Code: tea.KeyPgUp},
	"pgdown":    {Code: tea.KeyPgDown},
	"home":      {Code: tea.KeyHome},
	"end":       {Code: tea.KeyEnd},
	"space":     {Code: tea.KeySpace, Text: " "},
	"tab":       {Code: tea.KeyTab},
	"shift+tab": {Code: tea.KeyTab, Mod: tea.ModShift},
	"esc":       {Code: tea.KeyEscape},
	"backspace": {Code: tea.KeyBackspace},
	"delete":    {Code: tea.KeyDelete},
	"ctrl+a":    {Code: 'a', Mod: tea.ModCtrl},
	"ctrl+b":    {Code: 'b', Mod: tea.ModCtrl},
	"ctrl+d":    {Code: 'd', Mod: tea.ModCtrl},
	"ctrl+e":    {Code: 'e', Mod: tea.ModCtrl},
	"ctrl+f":    {Code: 'f', Mod: tea.ModCtrl},
	"ctrl+h":    {Code: 'h', Mod: tea.ModCtrl},
	"ctrl+k":    {Code: 'k', Mod: tea.ModCtrl},
	"ctrl+u":    {Code: 'u', Mod: tea.ModCtrl},
}

func keyMsg(name string) tea.KeyPressMsg {
	if k, ok := named[name]; ok {
		return k
	}
	rs := []rune(name)
	if len(rs) != 1 {
		panic("unknown key " + name)
	}
	return tea.KeyPressMsg{Code: rs[0], Text: name}
}

// keys sends the space-separated key names.
func (r *rig) keys(script string) *rig {
	r.t.Helper()
	for _, k := range strings.Fields(script) {
		r.send(keyMsg(k))
	}
	return r
}

// text types each code point of s as a key.
func (r *rig) text(s string) *rig {
	r.t.Helper()
	for _, c := range s {
		r.send(tea.KeyPressMsg{Code: c, Text: string(c)})
	}
	return r
}

// content returns the drawn lines.
func (r *rig) content() []string { return strings.Split(r.m.(ui.Model).View().Content, "\n") }

// lines returns the drawn lines with ANSI and trailing spaces removed.
func (r *rig) lines() []string {
	out := r.content()
	for i, l := range out {
		out[i] = strings.TrimRight(ansi.Strip(l), " ")
	}
	return out
}

func (r *rig) screen() string { return strings.Join(r.lines(), "\n") }

// status returns the status line, stripped.
func (r *rig) status() string {
	l := r.lines()
	return l[len(l)-2]
}

// modeOpen reports whether the frame holds a mode: the help line then
// names the mode's keys, or the status line says the mode plans or commits.
func (r *rig) modeOpen() bool {
	l := r.lines()
	st := l[len(l)-2]
	return l[len(l)-1] != "? help  q quit" || strings.HasSuffix(st, "planning…") || strings.HasSuffix(st, "committing…")
}

// golden compares the screen with a file of testdata.
func (r *rig) golden(name string) {
	r.t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		r.t.Fatal(err)
	}
	if got, want := r.screen(), strings.TrimSuffix(string(b), "\n"); got != want {
		r.t.Errorf("%s:\n got:\n%s\nwant:\n%s", name, got, want)
	}
}

// widths checks that every line is exactly w cells wide and that the
// screen holds h lines.
func (r *rig) widths(w, h int, method ansi.Method) {
	r.t.Helper()
	lines := r.content()
	if len(lines) != h {
		r.t.Errorf("%d×%d: %d lines", w, h, len(lines))
	}
	for i, l := range lines {
		if n := method.StringWidth(ansi.Strip(l)); n != w {
			r.t.Errorf("%d×%d: line %d is %d cells: %q", w, h, i+1, n, ansi.Strip(l))
		}
	}
}

// refusal builds a Refusals of one.
func refusal(code, task, gate, rule, text string) error {
	return action.Refusals{{Code: code, Task: task, Gate: gate, Rule: rule, Text: text}}
}

var errPlain = errors.New("the plain error\nand its second line")
