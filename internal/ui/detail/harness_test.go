package detail

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/ui/grid"
	"github.com/nbyoung/tablotui/internal/view/list"
)

// stub wraps a source.Lists, records each request and fails on demand.
type stub struct {
	mu   sync.Mutex
	next source.Lists
	reqs []source.ListRequest
	fail func(source.ListRequest) error
}

func newStub(dir string) *stub { return &stub{next: source.File{Dir: dir}} }

func (s *stub) record(r source.ListRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reqs = append(s.reqs, r)
	if s.fail != nil {
		return s.fail(r)
	}
	return nil
}

func (s *stub) Task(ctx context.Context, r source.ListRequest) (list.Task, error) {
	if err := s.record(r); err != nil {
		return list.Task{}, err
	}
	return s.next.Task(ctx, r)
}

func (s *stub) Queue(ctx context.Context, r source.ListRequest) (list.Queue, error) {
	if err := s.record(r); err != nil {
		return list.Queue{}, err
	}
	return s.next.Queue(ctx, r)
}

func (s *stub) Blockage(ctx context.Context, r source.ListRequest) (list.Blockage, error) {
	if err := s.record(r); err != nil {
		return list.Blockage{}, err
	}
	return s.next.Blockage(ctx, r)
}

func (s *stub) History(ctx context.Context, r source.ListRequest) (list.History, error) {
	if err := s.record(r); err != nil {
		return list.History{}, err
	}
	return s.next.History(ctx, r)
}

// count returns how many requests the stub saw.
func (s *stub) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.reqs)
}

// since returns the requests after the first n.
func (s *stub) since(n int) []source.ListRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]source.ListRequest(nil), s.reqs[n:]...)
}

var errBoom = errors.New("boom")

// rig drives the real frame with the real grid and the four panes: it starts
// no terminal.
type rig struct {
	t        *testing.T
	m        tea.Model
	src      *stub
	selected []ui.SelectionMsg
	notices  []ui.NoticeMsg
	sent     []tea.Msg // every message a command delivered, in order
}

// newRig builds the frame at w by h and runs the first loads. The grid draws
// from the grid's own fixtures, the panes from the stub over dir.
func newRig(t *testing.T, dir string, w, h int) *rig {
	t.Helper()
	src := newStub(dir)
	set := New(Options{Source: src})
	return newRigWith(t, src, set, w, h)
}

func newRigWith(t *testing.T, src *stub, set Set, w, h int) *rig {
	t.Helper()
	g := grid.New(grid.Options{Source: source.File{Dir: "../grid/testdata"}, Start: grid.GlanceStart()})
	m := ui.New(ui.Options{
		Panes:    append([]ui.Pane{g}, set.Panes...),
		Commands: set.Commands,
		Layout:   set.Layout,
		Styles:   ui.DefaultStyles(),
	})
	r := &rig{t: t, m: m, src: src}
	r.send(tea.WindowSizeMsg{Width: w, Height: h})
	r.run(m.Init())
	return r
}

func (r *rig) send(msg tea.Msg) {
	r.t.Helper()
	switch msg := msg.(type) {
	case ui.SelectionMsg:
		r.selected = append(r.selected, msg)
	case ui.NoticeMsg:
		r.notices = append(r.notices, msg)
	}
	m, cmd := r.m.Update(msg)
	r.m = m
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
	case tea.QuitMsg:
	default:
		r.sent = append(r.sent, msg)
		r.send(msg)
	}
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
	"ctrl+c":    {Code: 'c', Mod: tea.ModCtrl},
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

// content returns the drawn lines, ANSI included.
func (r *rig) content() []string { return strings.Split(r.m.(ui.Model).View().Content, "\n") }

// screen returns the view with ANSI and trailing spaces removed.
func (r *rig) screen() string {
	lines := r.content()
	for i, l := range lines {
		lines[i] = strings.TrimRight(ansi.Strip(l), " ")
	}
	return strings.Join(lines, "\n")
}

func (r *rig) line(i int) string { return strings.TrimRight(ansi.Strip(r.content()[i]), " ") }

func (r *rig) status() string { return r.line(len(r.content()) - 2) }

// measure switches the frame to the grapheme method, as a terminal that
// answers the mode query does.
func (r *rig) grapheme() {
	r.send(tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeSet})
}

// golden reads a file of testdata.
func golden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// lastSelected returns the newest SelectionMsg the grid sent.
func (r *rig) lastSelected() ui.SelectionMsg { return r.selected[len(r.selected)-1] }

// lastNotice returns the newest notice, or the zero value.
func (r *rig) lastNotice() ui.NoticeMsg {
	if len(r.notices) == 0 {
		return ui.NoticeMsg{}
	}
	return r.notices[len(r.notices)-1]
}

// ph drives one pane with messages and no frame. Its Load runs a request as
// the frame does, but the command that carries it waits in held until the
// test releases it, so a test orders the answers.
type ph struct {
	t       *testing.T
	p       ui.Pane
	src     *stub
	c       ui.Context
	held    []tea.Cmd
	sent    []tea.Msg // what commands delivered, other than answers
	notices []ui.NoticeMsg
	hold    bool
}

// pane builds the pane of the identified kind, closed, over a stub on dir,
// with a context of w by h cells and the focus.
func pane(t *testing.T, id, dir string, w, h int) *ph {
	t.Helper()
	src := newStub(dir)
	var p ui.Pane
	for _, q := range New(Options{Source: src}).Panes {
		if q.ID() == id {
			p = q
		}
	}
	if p == nil {
		t.Fatalf("no pane %q", id)
	}
	x := &ph{t: t, p: p, src: src}
	x.c = ui.Context{
		Styles:  ui.DefaultStyles(),
		Focused: true,
		Width:   w,
		Height:  h,
		Load: func(pane string, seq uint64, f func(context.Context) (any, error)) tea.Cmd {
			return func() tea.Msg {
				v, err := f(context.Background())
				return ui.LoadedMsg{Pane: pane, Seq: seq, Value: v, Err: err}
			}
		},
	}
	return x
}

// send delivers a message to the pane and queues its commands.
func (x *ph) send(msg tea.Msg) *ph {
	x.t.Helper()
	p, cmd := x.p.Update(msg, x.c)
	x.p = p
	x.queue(cmd)
	if !x.hold {
		x.release()
	}
	return x
}

func (x *ph) queue(cmd tea.Cmd) {
	if cmd != nil {
		x.held = append(x.held, cmd)
	}
}

// release runs the held commands in order and feeds their answers back.
func (x *ph) release(order ...int) *ph {
	x.t.Helper()
	for len(x.held) > 0 {
		held := x.held
		x.held = nil
		idx := make([]int, 0, len(held))
		idx = append(idx, order...)
		order = nil
		for i := range held {
			if !slices.Contains(idx, i) {
				idx = append(idx, i)
			}
		}
		for _, i := range idx {
			x.run(held[i])
		}
	}
	return x
}

func (x *ph) run(cmd tea.Cmd) {
	switch msg := cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			x.run(c)
		}
	case ui.LoadedMsg:
		p, next := x.p.Update(msg, x.c)
		x.p = p
		x.queue(next)
	case ui.NoticeMsg:
		x.notices = append(x.notices, msg)
		x.sent = append(x.sent, msg)
	default:
		x.sent = append(x.sent, msg)
	}
}

// open opens the pane through its digit command's message.
func (x *ph) open() *ph {
	x.t.Helper()
	return x.send(toggleMsg{pane: x.p.ID()})
}

// sel sends a selection of the grid.
func (x *ph) sel(task, gate string) *ph {
	x.t.Helper()
	return x.send(ui.SelectionMsg{Task: task, Gate: gate})
}

func (x *ph) keys(script string) *ph {
	x.t.Helper()
	for _, k := range strings.Fields(script) {
		x.send(keyMsg(k))
	}
	return x
}

func (x *ph) pane() Pane { return x.p.(Pane) }

// view returns the drawn pane, stripped, one string per line.
func (x *ph) view() []string {
	lines := strings.Split(x.p.View(x.c.Width, x.c.Height, x.c), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(ansi.Strip(l), " ")
	}
	return lines
}

func (x *ph) status() string { return x.p.Status(x.c) }

// doc returns the title and every row of the pane's document wrapped to w
// cells, as the golden documents hold them.
func (x *ph) doc(w int) string {
	p := x.pane()
	body, _ := p.body()
	out := []string{p.title()}
	for _, r := range wrap(body, w, x.c.Measure) {
		out = append(out, r.text)
	}
	return strings.Join(out, "\n") + "\n"
}

// requests returns the requests the stub saw.
func (x *ph) requests() []source.ListRequest { return x.src.since(0) }

// runHeld runs the i-th held command alone and feeds its answer back.
func (x *ph) runHeld(i int) *ph {
	x.t.Helper()
	c := x.held[i]
	x.held = slices.Delete(slices.Clone(x.held), i, i+1)
	x.run(c)
	return x
}

// roleSource answers a request that states no level at glance, as tablo does
// for a role that opens its views at glance, and any other request in full.
type roleSource struct{ source.File }

func (s roleSource) Task(ctx context.Context, r source.ListRequest) (list.Task, error) {
	d, err := s.File.Task(ctx, r)
	if r.Level == "" {
		d.Level = list.Glance
	}
	return d, err
}

// withSource replaces the source under the stub, which keeps counting.
func (x *ph) withSource(next source.Lists) *ph {
	x.src.next = next
	return x
}

func ansiStrip(s string) string { return ansi.Strip(s) }
