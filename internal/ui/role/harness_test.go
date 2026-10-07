package role

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/ui/grid"
	"github.com/nbyoung/tablotui/internal/view"
)

const (
	fixtures = "../grid/testdata"
	owner    = "nbyoung@nbyoung.com"
	agent    = "noreply@anthropic.com"
	stranger = "eve@example.org"
)

// spy records every request and can fail or script the viewer.
type spy struct {
	source.File
	reqs    []source.Request
	asked   int
	roles   []string
	failing bool
	lenient bool // serve the window 1 file whatever the window and the switch
}

func (s *spy) Tableau(ctx context.Context, r source.Request) (view.Tableau, error) {
	s.reqs = append(s.reqs, r)
	if s.lenient {
		r.Window, r.Historical = 1, false
	}
	return s.File.Tableau(ctx, r)
}

func (s *spy) Viewer(ctx context.Context, ref, email string) (view.Viewer, error) {
	s.asked++
	if s.failing {
		return view.Viewer{}, errors.New("the project does not load")
	}
	if s.roles != nil {
		return view.Viewer{Email: email, Roles: s.roles}, nil
	}
	if roles, ok := held[email]; ok {
		return view.Viewer{Email: email, Roles: roles}, nil
	}
	return s.File.Viewer(ctx, ref, email)
}

// last returns the newest request.
func (s *spy) last() source.Request { return s.reqs[len(s.reqs)-1] }

// held is the roles the two emails of the Tableaux tooling plan hold, as
// PLAN.md states them under Roles.
var held = map[string][]string{
	owner: {"owner", "authority", "assignee", "reviewer"},
	agent: {"authority", "assignee", "contributor", "agent"},
}

// rig drives the real frame with the real grid and the role pane: it starts
// no terminal.
type rig struct {
	t    *testing.T
	m    tea.Model
	spy  *spy
	sent []tea.Msg // every message a command delivered, in order
}

// newRig builds the frame as the command does, sizes it and runs the first
// loads. An empty settings path keeps no column choice.
func newRig(t *testing.T, viewer, settingsPath string, w, h int) *rig {
	t.Helper()
	src := &spy{File: source.File{Dir: fixtures}}
	g := grid.New(grid.Options{Source: src, Settings: settingsPath})
	who := New(Options{Source: src, Viewer: viewer})
	m := ui.New(ui.Options{
		Panes:    []ui.Pane{g, who},
		Commands: []ui.Command{Command()},
		Styles:   ui.DefaultStyles(),
	})
	return launch(t, m, src, w, h)
}

func launch(t *testing.T, m tea.Model, src *spy, w, h int) *rig {
	t.Helper()
	r := &rig{t: t, m: m, spy: src}
	r.send(tea.WindowSizeMsg{Width: w, Height: h})
	r.run(m.Init())
	return r
}

func (r *rig) send(msg tea.Msg) {
	r.t.Helper()
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
	default:
		r.sent = append(r.sent, msg)
		r.send(msg)
	}
}

var named = map[string]tea.KeyPressMsg{
	"up":    {Code: tea.KeyUp},
	"down":  {Code: tea.KeyDown},
	"left":  {Code: tea.KeyLeft},
	"right": {Code: tea.KeyRight},
	"tab":   {Code: tea.KeyTab},
	"end":   {Code: tea.KeyEnd},
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

// grapheme switches the frame to the grapheme method, as a terminal that
// answers the mode query does.
func (r *rig) grapheme() {
	r.send(tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeSet})
}

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

func (r *rig) context() string { return r.line(0) }

func (r *rig) status() string { return r.line(len(r.content()) - 2) }

// starts returns the StartMsgs the commands delivered so far.
func (r *rig) starts() []grid.Start {
	var out []grid.Start
	for _, m := range r.sent {
		if s, ok := m.(grid.StartMsg); ok {
			out = append(out, s.Start)
		}
	}
	return out
}

// golden reads a file of testdata, without its final newline.
func golden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(string(b), "\n")
}

// lastSel returns the task of the newest SelectionMsg.
func (r *rig) lastSel() string {
	for i := len(r.sent) - 1; i >= 0; i-- {
		if s, ok := r.sent[i].(ui.SelectionMsg); ok {
			return s.Task
		}
	}
	return ""
}
