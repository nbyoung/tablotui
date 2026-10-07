// Package grid is the tableau grid pane of the tablotui program: the task tree
// with fold and unfold, a gate window whose columns the viewer hides or shows,
// and a strip beneath it that discloses the detail and the provenance of the
// selection. The grid draws what its source sends and derives nothing from it;
// a change of parameter is a new request.
package grid

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/nbyoung/tablotui/internal/settings"
	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view"
)

// ID is the identifier of the grid pane.
const ID = "grid"

// Level is how much the strip beneath the rows discloses.
type Level int

// The levels: no strip, the detail of the selected row, and its provenance.
const (
	Glance Level = iota
	Detail
	Provenance
)

// Start is an arrival state. Open lists the unfolded parents; nil unfolds
// depth 0 alone. Select names a row; "" selects the first.
type Start struct {
	Request source.Request
	Open    []string
	Select  string
	Level   Level
}

// GlanceStart is the arrival of an observer and of the owner: the global
// tableau, window 1, historical off, the root and its children, no strip.
func GlanceStart() Start {
	return Start{
		Request: source.Request{View: "tableau", Window: 1, Level: "detail"},
		Level:   Glance,
	}
}

// StartMsg applies a start to a running grid. It leaves the column choices alone.
type StartMsg struct{ Start Start }

// Options configures the grid. An empty Settings path keeps no choice.
type Options struct {
	Source   source.Tableaux
	Settings string
	Start    Start
}

// Grid is the grid pane. It implements ui.Pane and ui.Headliner.
type Grid struct {
	src  source.Tableaux
	path string // the settings file; "" keeps no choice
	keys keyMap

	req      source.Request // what the grid asks for next
	goodReq  source.Request // what the data on screen answers
	seq      uint64         // the newest request
	wantProv bool           // the viewer has opened provenance
	data     *view.Tableau  // the last good view; nil before the first answer
	loadErr  string         // why the first load failed
	pending  *Start         // an arrival waiting for its data

	open  map[string]bool // the unfolded parents
	sel   string          // the selected task
	col   int             // the column cursor: 0 is the task, then each column in view
	level Level

	choices map[string]settings.Choice
	frozen  bool   // the settings file did not load: write nothing
	note    string // the notice that says so

	m     *metrics
	vis   []int // the rows in view, as indexes into data.Rows
	top   int   // the first row drawn, as an index into vis
	first int   // the first gate column drawn
	sent  ui.SelectionMsg
}

// New builds the grid pane, with the ID "grid".
func New(o Options) Grid {
	start := o.Start
	g := Grid{
		src:     o.Source,
		path:    o.Settings,
		keys:    newKeyMap(),
		req:     start.Request,
		seq:     1,
		level:   start.Level,
		pending: &start,
		open:    map[string]bool{},
		choices: map[string]settings.Choice{},
	}
	g.wantProv = start.Level == Provenance || start.Request.Level == "provenance"
	g.req.Level = g.askLevel()
	g.goodReq = g.req
	f, err := settings.Load(o.Settings)
	if err != nil {
		g.frozen = true
		g.note = fmt.Sprintf("settings: %v; this session keeps no column choice", err)
	} else {
		g.choices = f.Columns
	}
	return g
}

// ID implements ui.Pane.
func (g Grid) ID() string { return ID }

// Keys implements ui.Pane.
func (g Grid) Keys() []key.Binding { return helpKeys() }

// askLevel is the level the grid asks tablo for: every row at detail, and the
// facts from the first time the viewer opens provenance.
func (g Grid) askLevel() string {
	if g.wantProv {
		return "provenance"
	}
	return "detail"
}

// Init implements ui.Pane: it asks for the first view.
func (g Grid) Init(c ui.Context) tea.Cmd {
	cmds := []tea.Cmd{g.load(c, g.seq, g.req)}
	if g.note != "" {
		note := g.note
		cmds = append(cmds, func() tea.Msg { return ui.NoticeMsg{Text: note, Err: true} })
	}
	return tea.Batch(cmds...)
}

// load runs one request off the event loop.
func (g Grid) load(c ui.Context, seq uint64, req source.Request) tea.Cmd {
	src := g.src
	return c.Load(ID, seq, func(ctx context.Context) (any, error) {
		if src == nil {
			return nil, fmt.Errorf("no source")
		}
		return src.Tableau(ctx, req)
	})
}

// ask issues the current request under the next sequence number.
func (g Grid) ask(c ui.Context) (Grid, tea.Cmd) {
	g.seq++
	g.req.Level = g.askLevel()
	return g, g.load(c, g.seq, g.req)
}

func notice(text string, isErr bool) tea.Cmd {
	return func() tea.Msg { return ui.NoticeMsg{Text: text, Err: isErr} }
}

// Update implements ui.Pane. It settles both scroll offsets after every message.
func (g Grid) Update(msg tea.Msg, c ui.Context) (ui.Pane, tea.Cmd) {
	var cmd tea.Cmd
	loaded := false
	switch msg := msg.(type) {
	case ui.LoadedMsg:
		if msg.Pane != ID {
			return g, nil
		}
		g, cmd, loaded = g.loaded(msg)
		if !loaded && cmd == nil {
			return g, nil
		}
	case ui.ReloadMsg:
		g, cmd = g.ask(c)
	case StartMsg:
		g, cmd = g.start(msg.Start, c)
	case ui.GotoMsg:
		g, cmd = g.goTo(msg, c)
	case tea.KeyPressMsg:
		g, cmd = g.key(msg, c)
	}
	g = g.settled(c.Width, c.Height, c.Measure)
	if sel := g.selection(); loaded || sel != g.sent {
		g.sent = sel
		cmd = tea.Batch(cmd, func() tea.Msg { return sel })
	}
	return g, cmd
}

// start applies an arrival.
func (g Grid) start(s Start, c ui.Context) (Grid, tea.Cmd) {
	g.req = s.Request
	g.level = s.Level
	g.wantProv = g.wantProv || s.Level == Provenance || s.Request.Level == "provenance"
	g.pending = &s
	g.col = 0
	return g.ask(c)
}

// goTo selects the task a pane names: it unfolds the task's ancestors, moves
// the row cursor, and moves the column cursor when the gate shows as a column
// of its own.
func (g Grid) goTo(msg ui.GotoMsg, c ui.Context) (Grid, tea.Cmd) {
	if g.data == nil {
		return g, nil
	}
	rows := g.data.Rows
	at := slices.IndexFunc(rows, func(r view.Row) bool { return r.ID == msg.Task })
	if at < 0 {
		return g, notice(msg.Task+" is not in the tableau in view", false)
	}
	g.open = maps.Clone(g.open)
	depth := rows[at].Depth
	for i := at - 1; i >= 0 && depth > 0; i-- {
		if rows[i].Depth < depth {
			g.open[rows[i].ID] = true
			depth = rows[i].Depth
		}
	}
	g.sel = msg.Task
	g = g.settled(c.Width, c.Height, c.Measure)
	if i := slices.IndexFunc(g.data.Gates, func(gt view.Gate) bool { return gt.Key == msg.Gate }); i >= 0 {
		for k, col := range g.m.cols {
			if !col.folded && col.gates[0] == i {
				g.col = k + 1
			}
		}
	}
	return g, nil
}

// loaded takes the answer to a request. The bool says that a view arrived.
func (g Grid) loaded(msg ui.LoadedMsg) (Grid, tea.Cmd, bool) {
	if msg.Seq != g.seq {
		return g, nil, false
	}
	var t view.Tableau
	err := msg.Err
	if err == nil {
		var ok bool
		if t, ok = msg.Value.(view.Tableau); !ok {
			err = fmt.Errorf("unexpected data of type %T", msg.Value)
		}
	}
	if err != nil {
		if g.data == nil {
			g.loadErr = err.Error()
			return g, nil, false
		}
		g.req = g.goodReq
		return g, notice(err.Error()+" (showing the last good view)", true), false
	}
	old := g.data
	g.data = &t
	g.goodReq = g.req
	g.loadErr = ""
	g.m = nil
	if g.pending != nil {
		g = g.arrive(*g.pending)
		g.pending = nil
	} else if old != nil {
		g.sel = survivor(old.Rows, t.Rows, g.sel)
	}
	return g, nil, true
}

// arrive applies a start's folds and selection to the data.
func (g Grid) arrive(s Start) Grid {
	rows := g.data.Rows
	g.open = map[string]bool{}
	if s.Open == nil {
		for _, r := range rows {
			if r.Parent && r.Depth == 0 {
				g.open[r.ID] = true
			}
		}
	} else {
		for _, id := range s.Open {
			g.open[id] = true
		}
	}
	g.sel = ""
	if len(rows) > 0 {
		g.sel = rows[0].ID
	}
	if s.Select != "" && slices.ContainsFunc(rows, func(r view.Row) bool { return r.ID == s.Select }) {
		g.sel = s.Select
	}
	return g
}

// survivor keeps the selected id when the new rows hold it, and otherwise
// takes the nearest row above it, in the old order, that the new rows hold.
func survivor(old, rows []view.Row, sel string) string {
	has := func(id string) bool { return slices.ContainsFunc(rows, func(r view.Row) bool { return r.ID == id }) }
	if has(sel) {
		return sel
	}
	at := slices.IndexFunc(old, func(r view.Row) bool { return r.ID == sel })
	for i := at - 1; i >= 0; i-- {
		if has(old[i].ID) {
			return old[i].ID
		}
	}
	if len(rows) > 0 {
		return rows[0].ID
	}
	return ""
}

// selection is what the grid tells the frame it selects.
func (g Grid) selection() ui.SelectionMsg {
	s := ui.SelectionMsg{Task: g.sel}
	if g.data != nil && g.col >= 1 && g.col <= len(g.m.cols) && !g.m.cols[g.col-1].folded {
		s.Gate = g.data.Gates[g.m.cols[g.col-1].gates[0]].Key
	}
	return s
}

// settled returns the grid with its derived state in step with the data, the
// choices and the measure, its selection in view and both scroll offsets in
// range, for a pane of w by h cells. A size of zero leaves the scroll alone.
func (g Grid) settled(w, h int, m ui.Measure) Grid {
	if g.data == nil {
		return g
	}
	if g.m == nil || g.m.measure != m {
		g.m = buildMetrics(g.data, g.choices, m)
	}
	rows := g.data.Rows
	g.vis = g.vis[:0:0]
	skip := -1
	for i, r := range rows {
		if skip >= 0 && r.Depth > skip {
			continue
		}
		skip = -1
		g.vis = append(g.vis, i)
		if r.Parent && !g.open[r.ID] {
			skip = r.Depth
		}
	}
	// A selection that left the data or folded away moves to the nearest row above.
	at := g.selPos()
	if at < 0 && len(g.vis) > 0 {
		at = 0
		if i := slices.IndexFunc(rows, func(r view.Row) bool { return r.ID == g.sel }); i >= 0 {
			at = max(0, sortedBelow(g.vis, i))
		}
		g.sel = rows[g.vis[at]].ID
	}
	g.col = min(max(g.col, 0), len(g.m.cols))
	if w <= 0 || h <= 0 {
		return g
	}
	geo := g.plan(w, h)
	g.first = geo.first
	if at >= 0 {
		if at < g.top {
			g.top = at
		}
		if geo.rows > 0 && at >= g.top+geo.rows {
			g.top = at - geo.rows + 1
		}
	}
	g.top = max(0, min(g.top, len(g.vis)-geo.rows))
	return g
}

// sortedBelow returns the position of the last element of vis that is at most i.
func sortedBelow(vis []int, i int) int {
	pos, _ := slices.BinarySearch(vis, i+1)
	return pos - 1
}

// selPos returns the position of the selected row in the rows in view, or -1.
func (g Grid) selPos() int {
	for p, i := range g.vis {
		if g.data.Rows[i].ID == g.sel {
			return p
		}
	}
	return -1
}

// key handles one key press.
func (g Grid) key(k tea.KeyPressMsg, c ui.Context) (Grid, tea.Cmd) {
	if g.data == nil {
		return g, nil
	}
	g = g.settled(c.Width, c.Height, c.Measure)
	km := g.keys
	pos := max(g.selPos(), 0)
	page := max(1, g.plan(max(c.Width, 1), max(c.Height, 1)).rows)
	switch {
	case key.Matches(k, km.Up):
		g = g.moveTo(pos - 1)
	case key.Matches(k, km.Down):
		g = g.moveTo(pos + 1)
	case key.Matches(k, km.PageUp):
		g = g.moveTo(pos - page)
	case key.Matches(k, km.PageDown):
		g = g.moveTo(pos + page)
	case key.Matches(k, km.First):
		g = g.moveTo(0)
	case key.Matches(k, km.Last):
		g = g.moveTo(len(g.vis) - 1)
	case key.Matches(k, km.Left):
		g.col = max(0, g.col-1)
	case key.Matches(k, km.Right):
		g.col = min(len(g.m.cols), g.col+1)
	case key.Matches(k, km.Fold):
		g = g.toggle()
	case key.Matches(k, km.Unfold):
		g.open = map[string]bool{}
		for _, r := range g.data.Rows {
			if r.Parent {
				g.open[r.ID] = true
			}
		}
	case key.Matches(k, km.Glance):
		g.open = map[string]bool{}
		for _, r := range g.data.Rows {
			if r.Parent && r.Depth == 0 {
				g.open[r.ID] = true
			}
		}
	case key.Matches(k, km.Detail):
		if g.level == Glance {
			g.level = Detail
		} else {
			g.level = Glance
		}
	case key.Matches(k, km.Provenance):
		if g.level == Provenance {
			g.level = Detail
			break
		}
		g.level = Provenance
		if !g.wantProv {
			g.wantProv = true
			return g.ask(c)
		}
	case key.Matches(k, km.Hide):
		return g.hide()
	case key.Matches(k, km.Show):
		return g.show()
	case key.Matches(k, km.Window):
		return g.forget()
	case key.Matches(k, km.Wider):
		if g.data != nil && !slices.ContainsFunc(g.data.Gates, func(gt view.Gate) bool { return !gt.Window }) {
			return g, notice("the window already shows every gate", false)
		}
		g.req.Window++
		return g.ask(c)
	case key.Matches(k, km.Narrower):
		if g.req.Window <= 0 {
			return g, notice("the window is at its narrowest, window 0", false)
		}
		g.req.Window--
		return g.ask(c)
	case key.Matches(k, km.Historical):
		g.req.Historical = !g.req.Historical
		return g.ask(c)
	}
	return g, nil
}

// moveTo selects the row at position p of the rows in view, held in range.
func (g Grid) moveTo(p int) Grid {
	if len(g.vis) == 0 {
		return g
	}
	p = min(max(p, 0), len(g.vis)-1)
	g.sel = g.data.Rows[g.vis[p]].ID
	return g
}

// toggle folds or unfolds the selected parent.
func (g Grid) toggle() Grid {
	p := g.selPos()
	if p < 0 {
		return g
	}
	r := g.data.Rows[g.vis[p]]
	if !r.Parent {
		return g
	}
	g.open = maps.Clone(g.open)
	if g.open[r.ID] {
		delete(g.open, r.ID)
	} else {
		g.open[r.ID] = true
	}
	return g
}

// Headline implements ui.Headliner: the context line.
func (g Grid) Headline(c ui.Context) string {
	t := g.data
	if t == nil {
		return ""
	}
	s := fmt.Sprintf("%s · %s at %s, %s", t.Project, t.Ref, t.Commit, t.Date)
	switch {
	case t.Viewer != "" && t.Role != "":
		s += fmt.Sprintf(" · viewer %s, %s", t.Viewer, t.Role)
	case t.Viewer != "":
		s += " · viewer " + t.Viewer
	}
	return s
}
