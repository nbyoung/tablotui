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

func (f *frame) line(i int) string {
	return strings.TrimRight(ansi.Strip(strings.Split(f.m.View().Content, "\n")[i]), " ")
}

// The pane stores each good load and broadcasts one ReloadMsg after it; a
// failed load stores nothing, broadcasts nothing, and shows its notice,
// which the next good load clears.
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
	opts := ui.Options{Panes: []ui.Pane{counter{}, p}}
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
	if got := f.line(6); !strings.HasPrefix(got, "error: Reload failed at ") || !strings.HasSuffix(got, ": boom") {
		t.Errorf("status line = %q", got)
	}
	fail = false
	f.send(changed{state("cccc333")})
	if len(stored) != 2 || stored[1] != "data of cccc333" || f.line(1) != "xx" {
		t.Errorf("after the fix: stored %q, body %q", stored, f.line(1))
	}
	if got := f.line(6); got != "home status" {
		t.Errorf("status line = %q", got)
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
	if got := f.line(6); got != "error: No view yet: not a git repository" || f.line(1) != "" {
		t.Errorf("status line %q, body %q", got, f.line(1))
	}
	f.send(changed{state("aaaa111")})
	if f.line(1) != "x" || f.line(6) != "home status" {
		t.Errorf("body %q, status line %q", f.line(1), f.line(6))
	}
}
