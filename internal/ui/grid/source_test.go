package grid

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"errors"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"sync"
	"testing"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view"
)

// cached serves the fixtures and decodes each file once.
type cached struct {
	mu    sync.Mutex
	files map[string]view.Tableau
	errs  map[string]error
}

func newCached() *cached {
	return &cached{files: map[string]view.Tableau{}, errs: map[string]error{}}
}

func (c *cached) Tableau(ctx context.Context, r source.Request) (view.Tableau, error) {
	name := source.File{}.Name(r)
	c.mu.Lock()
	defer c.mu.Unlock()
	if t, ok := c.files[name]; ok {
		return t, nil
	}
	if err, ok := c.errs[name]; ok {
		return view.Tableau{}, err
	}
	t, err := source.File{Dir: "testdata"}.Tableau(ctx, r)
	if err != nil {
		c.errs[name] = err
		return t, err
	}
	c.files[name] = t
	return t, nil
}

// recorder answers each request through fn and keeps the requests it sees.
type recorder struct {
	fn   func(r source.Request) (view.Tableau, error)
	reqs []source.Request
}

func (s *recorder) Tableau(_ context.Context, r source.Request) (view.Tableau, error) {
	s.reqs = append(s.reqs, r)
	return s.fn(r)
}

func (s *recorder) last() source.Request { return s.reqs[len(s.reqs)-1] }

// load reads a fixture by file name.
func load(t *testing.T, name string) view.Tableau {
	t.Helper()
	tb, err := source.File{Dir: "testdata"}.Tableau(context.Background(), source.Request{View: strings.TrimSuffix(name, ".json"), Window: 1})
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

// asFixture answers every request with the main fixture, the window and the
// historical switch taken from the request, as tablo would state them.
func asFixture(t *testing.T) *recorder {
	t.Helper()
	base := load(t, "tableau.json")
	return &recorder{fn: func(r source.Request) (view.Tableau, error) {
		tb := base
		w := r.Window
		tb.Params.Window = &w
		tb.Params.Historical = r.Historical
		return tb, nil
	}}
}

// windowed returns tb with the columns tablo would state for a window that
// shows the gates window marks, each gate with its count: one column per gate
// shown, and one folded column per run of gates not shown.
func windowed(tb view.Tableau, window []bool, counts []int) view.Tableau {
	gates := tb.Legend.Gates
	tb.Columns = nil
	for i := 0; i < len(gates); {
		j := i + 1
		for !window[i] && j < len(gates) && !window[j] {
			j++
		}
		c := view.Column{Folded: !window[i]}
		for k := i; k < j; k++ {
			c.Gates = append(c.Gates, gates[k].Key)
			c.ByGate = append(c.ByGate, counts[k])
			c.Count += counts[k]
		}
		tb.Columns = append(tb.Columns, c)
		i = j
	}
	return tb
}

var errBoom = errors.New("the source failed")

func ui_ReloadMsg() ui.ReloadMsg { return ui.ReloadMsg{} }

func stripANSI(s string) string { return ansi.Strip(s) }

func sizeMsg(w, h int) tea.WindowSizeMsg { return tea.WindowSizeMsg{Width: w, Height: h} }
