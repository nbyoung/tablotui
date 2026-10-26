package refresh

import (
	"context"
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/watch"
)

// counter is the home pane of the frame test: it counts the ReloadMsg the
// frame broadcasts and draws the count.
type counter struct{ reloads int }

func (c counter) ID() string              { return "home" }
func (c counter) Init(ui.Context) tea.Cmd { return nil }
func (c counter) Update(msg tea.Msg, _ ui.Context) (ui.Pane, tea.Cmd) {
	if _, ok := msg.(ui.ReloadMsg); ok {
		c.reloads++
	}
	return c, nil
}
func (c counter) View(w, h int, _ ui.Context) string {
	return strings.Repeat("x", c.reloads) + strings.Repeat("\n", max(h-1, 0))
}
func (c counter) Status(ui.Context) string { return "home status" }
func (c counter) Keys() []key.Binding      { return nil }

// frame drives the frame with messages and runs each command; the script
// answers each wait at once from the states the test feeds it. The first
// change, which Init reads, goes to the frame; a later one, which a wait
// returns, goes nowhere, since the test sends each change itself.
type frame struct {
	t       *testing.T
	m       tea.Model
	started bool
}

func (f *frame) send(msg tea.Msg) {
	f.t.Helper()
	m, cmd := f.m.Update(msg)
	f.m = m
	f.run(cmd)
}

func (f *frame) run(cmd tea.Cmd) {
	f.t.Helper()
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			f.run(c)
		}
	case changed:
		if !f.started {
			f.started = true
			f.send(msg)
		}
	default:
		f.send(msg)
	}
}

func (f *frame) line(i int) string { return strings.TrimRight(ansi.Strip(f.raw(i)), " ") }

func (f *frame) raw(i int) string { return strings.Split(f.m.View().Content, "\n")[i] }

// The pane stores each good load and broadcasts one ReloadMsg after it; a
// failed load stores nothing, broadcasts nothing, and shows its notice on a
// line of its own above the status line, in the error style, which the next
// good load clears. The status line ends with Where (171b A5).
func TestPaneStoresAndReloads(t *testing.T) {
	w := newScript(state("aaaa111"))
	for range 4 {
		w.feed <- state("bbbb222")
	}
	fail := false
	var stored []string
	load := func(_ context.Context, st watch.State) (string, error) {
		if fail {
			return "", errors.New("boom\nsecond line")
		}
		return "data of " + st.Commit, nil
	}
	m := New(t.Context(), live, w, newStepper().now, load)
	p := NewPane(m, func(s string) { stored = append(stored, s) })
	if p.ID() != PaneID || p.Keys() != nil || p.Status(ui.Context{}) != "" || p.View(3, 2, ui.Context{}) != "   \n   " {
		t.Error("the pane is not blank")
	}
	opts := ui.Options{Panes: []ui.Pane{counter{}, p}, Styles: ui.DefaultStyles()}
	if err := ui.CheckKeys(opts); err != nil {
		t.Fatal(err)
	}
	f := &frame{t: t, m: ui.New(opts)}
	f.send(tea.WindowSizeMsg{Width: 120, Height: 8})
	f.run(f.m.Init())
	if len(stored) != 1 || stored[0] != "data of aaaa111" || f.line(1) != "x" {
		t.Fatalf("after the first load: stored %q, body %q", stored, f.line(1))
	}
	fail = true
	f.send(changed{state("bbbb222")})
	if len(stored) != 1 || f.line(1) != "x" {
		t.Errorf("a failed load stored %q, body %q", stored, f.line(1))
	}
	if got := f.line(5); !strings.HasPrefix(got, "Reload failed at ") || !strings.HasSuffix(got, ": boom") {
		t.Errorf("notice line = %q", got)
	}
	if got := f.raw(5); !strings.Contains(got, "\x1b[") {
		t.Errorf("the notice line %q carries no style", got)
	}
	if got := f.line(6); got != "home status · working tree on main at aaaa111" {
		t.Errorf("status line = %q", got)
	}
	fail = false
	f.send(changed{state("cccc333")})
	if len(stored) != 2 || stored[1] != "data of cccc333" || f.line(1) != "xx" {
		t.Errorf("after the fix: stored %q, body %q", stored, f.line(1))
	}
	if got, body := f.line(6), f.line(5); got != "home status · working tree on main at cccc333" || body != "" {
		t.Errorf("status line = %q, the last line of the body %q", got, body)
	}
}

// A good load beside a diagnostic shows it on the notice line in the error
// style; a key leaves it, and the next clean load clears it (171b decision 15).
func TestPaneDiagnosticBesideTheView(t *testing.T) {
	w := newScript(state("aaaa111"))
	for range 4 {
		w.feed <- state("aaaa111")
	}
	var next error = beside{"status/9f31.yaml:3:8: S6 bad"}
	m := New(t.Context(), live, w, newStepper().now, func(context.Context, watch.State) (int, error) { return 1, next })
	f := &frame{t: t, m: ui.New(ui.Options{Panes: []ui.Pane{counter{}, NewPane(m, nil)}, Styles: ui.DefaultStyles()})}
	f.send(tea.WindowSizeMsg{Width: 60, Height: 8})
	f.run(f.m.Init())
	want := "The project has errors: status/9f31.yaml:3:8: S6 bad"
	if f.line(5) != want || !strings.Contains(f.raw(5), "\x1b[") || f.line(1) != "x" {
		t.Errorf("notice %q (raw %q), body %q", f.line(5), f.raw(5), f.line(1))
	}
	f.send(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if f.line(5) != want {
		t.Errorf("after a key the notice reads %q", f.line(5))
	}
	next = nil
	f.send(changed{state("bbbb222")})
	if f.line(5) != "" {
		t.Errorf("after a clean load the notice reads %q", f.line(5))
	}
}

// A first load that fails says there is no view yet, and a nil store is no
// fault.
func TestPaneFirstLoadFails(t *testing.T) {
	w := newScript(watch.State{Err: "not a git repository"})
	for range 4 {
		w.feed <- state("aaaa111")
	}
	m := New(t.Context(), live, w, newStepper().now, func(context.Context, watch.State) (int, error) { return 1, nil })
	f := &frame{t: t, m: ui.New(ui.Options{Panes: []ui.Pane{counter{}, NewPane(m, nil)}})}
	f.send(tea.WindowSizeMsg{Width: 60, Height: 8})
	f.run(f.m.Init())
	if got := f.line(5); got != "No view yet: not a git repository" || f.line(1) != "" || f.line(6) != "home status · working tree" {
		t.Errorf("notice line %q, body %q, status line %q", got, f.line(1), f.line(6))
	}
	f.send(changed{state("aaaa111")})
	if f.line(1) != "x" || f.line(5) != "" || f.line(6) != "home status · working tree on main at aaaa111" {
		t.Errorf("body %q, notice %q, status line %q", f.line(1), f.line(5), f.line(6))
	}
}

// The first state loads with Loading on its own line, and with --ref the
// status line names the ref and the commit: the rows of 171b's table.
func TestPaneLoadingAndARef(t *testing.T) {
	w := newScript(state("aaaa111"))
	for range 4 {
		w.feed <- state("aaaa111")
	}
	block := make(chan struct{})
	m := New(t.Context(), watch.Source{Dir: ".", Ref: "v1.0"}, w, newStepper().now, func(context.Context, watch.State) (int, error) {
		<-block
		return 1, nil
	})
	p := NewPane(m, nil)
	if n := p.Notice(); n != (ui.NoticeMsg{Text: "Loading"}) || p.Where() != "v1.0" {
		t.Errorf("before the first load: notice %+v, where %q", n, p.Where())
	}
	f := &frame{t: t, m: ui.New(ui.Options{Panes: []ui.Pane{counter{}, p}})}
	f.send(tea.WindowSizeMsg{Width: 60, Height: 8})
	if f.line(5) != "Loading" || f.line(6) != "home status · v1.0" {
		t.Errorf("notice %q, status %q", f.line(5), f.line(6))
	}
	close(block)
	f.run(f.m.Init())
	if f.line(5) != "" || f.line(6) != "home status · v1.0 at aaaa111" {
		t.Errorf("notice %q, status %q", f.line(5), f.line(6))
	}
}

// A6: the Force key, ctrl+r, reaches the pane through the frame as ForceMsg
// and pokes the watcher once; it clashes with no other key.
func TestForceKeyPokes(t *testing.T) {
	w := newScript(state("aaaa111"))
	for range 4 {
		w.feed <- state("aaaa111")
	}
	m := New(t.Context(), live, w, newStepper().now, func(context.Context, watch.State) (int, error) { return 1, nil })
	opts := ui.Options{Panes: []ui.Pane{counter{}, NewPane(m, nil)}, Commands: []ui.Command{Command()}}
	if err := ui.CheckKeys(opts); err != nil {
		t.Fatal(err)
	}
	if h := Command().Binding.Help(); h.Key != "ctrl+r" || h.Desc != "reload" {
		t.Errorf("the binding's help %+v", h)
	}
	f := &frame{t: t, m: ui.New(opts)}
	f.send(tea.WindowSizeMsg{Width: 60, Height: 8})
	f.run(f.m.Init())
	f.send(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	if got := w.pokes.Load(); got != 1 {
		t.Errorf("ctrl+r poked %d times, want once", got)
	}
}
