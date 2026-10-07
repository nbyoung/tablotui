package detail

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view/list"
)

// The identifiers of the four panes.
const (
	TaskID     = "task"
	QueueID    = "queue"
	BlockageID = "blockage"
	HistoryID  = "history"
)

// kind is what tells the four panes apart.
type kind struct {
	id, name, digit string
	follows         bool       // the request names the selected task
	deepest         list.Level // the deepest level the pane draws
	tail            bool       // the cursor arrives on the last row
}

var kinds = []kind{
	{id: TaskID, name: "Task", digit: "1", follows: true, deepest: list.Provenance},
	{id: QueueID, name: "Queue", digit: "2", deepest: list.Detail},
	{id: BlockageID, name: "Blockage", digit: "3", follows: true, deepest: list.Provenance},
	{id: HistoryID, name: "History", digit: "4", follows: true, deepest: list.Provenance, tail: true},
}

// toggleMsg is what a digit command sends: the pane it names opens focused,
// takes the focus when open, and closes when it has the focus.
type toggleMsg struct{ pane string }

// Messages the pane's own words use.
const (
	noSelection   = "no task selected"
	noPersonText  = "The work queue needs a person, and the viewer states none."
	noProvenance  = "the work queue's provenance is the brief of one item, which this pane does not draw"
	noTaskOnLine  = "this line names no task"
	loadingText   = "loading…"
	loadingSuffix = " · loading…"
)

// Pane is one detail pane. It implements ui.Pane.
type Pane struct {
	k    kind
	src  source.Lists
	keys keyMap

	open bool
	sel  ui.SelectionMsg // the latest selection of the grid

	seq       uint64
	asked     source.ListRequest // the request in flight
	pending   bool
	have      source.ListRequest // the request the data answers
	data      any                // *list.Task and so on; nil before the first answer
	stale     bool               // a ReloadMsg arrived while the pane was closed
	failed    bool               // the newest load failed and left no data
	noPerson  bool               // it failed because the viewer has no email
	errText   string             // why it failed
	failedFor *source.ListRequest

	viewer, role string // whom the pane reads for and as which role; the role pane sets them

	level  *list.Level // the level the viewer chose; nil draws the level the data states
	prov   bool        // the viewer has opened provenance
	unfold bool

	subject, counts string
	lines           []Line
	cur, sub        int // the cursor: a line and a row within it
	top, topSub     int // the first row drawn
}

func newPane(k kind, src source.Lists) Pane {
	return Pane{k: k, src: src, keys: newKeyMap()}
}

// ID implements ui.Pane.
func (p Pane) ID() string { return p.k.id }

// Init implements ui.Pane: a closed pane asks for nothing.
func (p Pane) Init(ui.Context) tea.Cmd { return nil }

// Keys implements ui.Pane.
func (p Pane) Keys() []key.Binding { return helpKeys() }

func notice(text string, isErr bool) tea.Cmd {
	return func() tea.Msg { return ui.NoticeMsg{Text: text, Err: isErr} }
}

// hasRow says that the pane has what it follows: the queue follows the viewer,
// and the other three follow the task the grid selects.
func (p Pane) hasRow() bool { return !p.k.follows || p.sel.Task != "" }

// request derives what the pane asks for from the selection and the level.
func (p Pane) request() (source.ListRequest, bool) {
	r := source.ListRequest{View: p.k.id, Viewer: p.viewer, Role: p.role}
	if p.level != nil {
		r.Level = "detail"
		if p.prov {
			r.Level = "provenance"
		}
	}
	if p.k.follows {
		if p.sel.Task == "" {
			return r, false
		}
		r.Task = p.sel.Task
	}
	return r, true
}

// dataLevel is the level the data states; glance with no data.
func (p Pane) dataLevel() list.Level {
	switch d := p.data.(type) {
	case *list.Task:
		return d.Level
	case *list.Queue:
		return d.Level
	case *list.Blockage:
		return d.Level
	case *list.History:
		return d.Level
	}
	return list.Glance
}

// shown is the level the pane draws: the level the data states, lowered to
// the level the viewer chose and to the deepest level the pane draws.
func (p Pane) shown() list.Level {
	l := p.dataLevel()
	if p.level != nil {
		l = min(l, *p.level)
	}
	return min(l, p.k.deepest)
}

// serves says that the data in hand answers the request: it is for the same
// task and holds the level wanted.
func (p Pane) serves(req source.ListRequest) bool {
	if p.data == nil || p.have.Task != req.Task {
		return false
	}
	if p.level == nil || p.have.Level == req.Level {
		return true
	}
	return p.dataLevel() >= *p.level
}

// sync asks for the request the pane now derives, unless the data in hand or
// the load in flight already answers it. A closed pane asks for nothing, and
// force asks again whatever answers.
func (p Pane) sync(c ui.Context, force bool) (Pane, tea.Cmd) {
	if !p.open {
		return p, nil
	}
	req, ok := p.request()
	if !ok {
		if p.pending {
			p.seq++ // the answer in flight is for a row the grid left
			p.pending = false
		}
		return p, nil
	}
	if !force {
		switch {
		case p.pending && p.asked == req:
			return p, nil
		case !p.pending && p.failedFor != nil && *p.failedFor == req:
			return p, nil
		case !p.pending && p.serves(req):
			return p, nil
		}
	}
	return p.ask(c, req)
}

// ask issues a request under the next sequence number.
func (p Pane) ask(c ui.Context, req source.ListRequest) (Pane, tea.Cmd) {
	p.seq++
	p.asked, p.pending = req, true
	p.failed, p.noPerson, p.errText, p.failedFor = false, false, "", nil
	src, id := p.src, p.k.id
	return p, c.Load(id, p.seq, func(ctx context.Context) (any, error) {
		if src == nil {
			return nil, errors.New("no source")
		}
		switch id {
		case TaskID:
			return src.Task(ctx, req)
		case QueueID:
			return src.Queue(ctx, req)
		case BlockageID:
			return src.Blockage(ctx, req)
		default:
			return src.History(ctx, req)
		}
	})
}

// loaded takes the answer to the newest request.
func (p Pane) loaded(msg ui.LoadedMsg) (Pane, tea.Cmd) {
	if msg.Seq != p.seq {
		return p, nil
	}
	p.pending = false
	req := p.asked
	data, err := p.accept(msg)
	if err != nil {
		p.failedFor = &req
		if !errors.Is(err, source.ErrNoPerson) && p.data != nil && p.have.Task == req.Task {
			return p, notice(fmt.Sprintf("%s: %s (showing the last good view)", p.k.id, err), true)
		}
		p.data, p.lines = nil, nil
		p.failed, p.errText = true, err.Error()
		p.noPerson = errors.Is(err, source.ErrNoPerson)
		return p, nil
	}
	same := p.data != nil && p.have.Task == req.Task
	p.data, p.have, p.failedFor = data, req, nil
	return p.rebuilt(same), nil
}

// accept returns the data of an answer, or the error it holds.
func (p Pane) accept(msg ui.LoadedMsg) (any, error) {
	if msg.Err != nil {
		return nil, msg.Err
	}
	switch v := msg.Value.(type) {
	case list.Task:
		if p.k.id == TaskID {
			return &v, nil
		}
	case list.Queue:
		if p.k.id == QueueID {
			return &v, nil
		}
	case list.Blockage:
		if p.k.id == BlockageID {
			return &v, nil
		}
	case list.History:
		if p.k.id == HistoryID {
			return &v, nil
		}
	}
	return nil, fmt.Errorf("unexpected data of type %T", msg.Value)
}

// build draws the data as lines at the level shown.
func (p Pane) build() (subject, counts string, lines []Line) {
	lvl := p.shown()
	switch d := p.data.(type) {
	case *list.Task:
		return taskLines(*d, lvl, p.unfold)
	case *list.Queue:
		return queueLines(*d, lvl, p.unfold, p.sel.Task)
	case *list.Blockage:
		return blockageLines(*d, lvl)
	case *list.History:
		return historyLines(*d, lvl)
	}
	return "", "", nil
}

// rebuilt builds the lines again from the data. With keep the cursor stays on
// the item it stood on; otherwise it arrives on the first line, or on the
// last for the history.
func (p Pane) rebuilt(keep bool) Pane {
	if p.data == nil {
		return p
	}
	item := -1
	if keep && p.cur < len(p.lines) {
		item = p.lines[p.cur].Item
	}
	p.subject, p.counts, p.lines = p.build()
	at := -1
	if item >= 0 {
		at = slices.IndexFunc(p.lines, func(l Line) bool { return l.Item == item })
	}
	if at >= 0 {
		p.cur, p.sub = at, 0
		return p
	}
	p.top, p.topSub = 0, 0
	p.cur, p.sub = 0, 0
	if p.k.tail {
		p.cur, p.sub = max(0, len(p.lines)-1), 1<<30
	}
	return p
}

// showData says that the pane draws its data: it has some, and the row it
// follows still stands.
func (p Pane) showData() bool { return p.data != nil && p.hasRow() }

// body is the lines the pane draws: its document, or the one line that
// stands for it.
func (p Pane) body() (lines []Line, isErr bool) {
	one := func(text string) []Line { return []Line{{Text: text, Item: -1}} }
	switch {
	case !p.hasRow():
		return one(noSelection), false
	case p.data != nil:
		return p.lines, false
	case p.noPerson:
		return one(noPersonText), false
	case p.failed:
		return one("error: " + p.errText), true
	}
	return one(loadingText), false
}

// title is what the title rule says.
func (p Pane) title() string {
	t := p.k.digit + " " + p.k.name
	if p.showData() {
		t = join(" · ", join(" ", t, p.subject), p.counts, p.shown().String())
	}
	if p.pending {
		t += loadingSuffix
	}
	return t
}

// window is where the cursor and the first row stand among the rows.
type window struct {
	rows     []row
	cur, top int
}

// locate returns the index of the row at the line and the row within it,
// or of the last row of that line when it has fewer, or of the last row.
func locate(rows []row, line, sub int) int {
	at := 0
	for i, r := range rows {
		if r.line > line || r.line == line && r.sub > sub {
			break
		}
		at = i
	}
	return at
}

// settle wraps the body for a pane of w by h cells and holds the cursor and
// the first row in range, the cursor in view. View calls it and stores
// nothing; Update stores its result.
func (p Pane) settle(w, h int, m ui.Measure) window {
	body, _ := p.body()
	if w < 4 {
		return window{}
	}
	win := window{rows: wrap(body, w-2, m)}
	if len(win.rows) == 0 {
		return win
	}
	win.cur = locate(win.rows, p.cur, p.sub)
	win.top = locate(win.rows, p.top, p.topSub)
	if rowsH := max(h-1, 0); rowsH > 0 {
		if win.cur < win.top {
			win.top = win.cur
		}
		if win.cur >= win.top+rowsH {
			win.top = win.cur - rowsH + 1
		}
		win.top = min(win.top, max(0, len(win.rows)-rowsH))
	}
	win.top = max(0, win.top)
	return win
}

// stored returns the pane with the window's cursor and first row kept.
func (p Pane) stored(win window) Pane {
	if len(win.rows) == 0 {
		return p
	}
	r, t := win.rows[win.cur], win.rows[win.top]
	p.cur, p.sub, p.top, p.topSub = r.line, r.sub, t.line, t.sub
	return p
}

// settled keeps the window for the size the frame gives the pane; a pane that
// no rectangle holds keeps what it has.
func (p Pane) settled(c ui.Context) Pane {
	if c.Width <= 0 || c.Height <= 0 {
		return p
	}
	return p.stored(p.settle(c.Width, c.Height, c.Measure))
}

// Update implements ui.Pane.
func (p Pane) Update(msg tea.Msg, c ui.Context) (ui.Pane, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case toggleMsg:
		if msg.pane != p.k.id {
			return p, nil
		}
		p, cmd = p.toggle(c)
	case ui.SelectionMsg:
		moved := msg.Task != p.sel.Task
		p.sel = msg
		if moved && p.k.id == QueueID {
			p = p.rebuilt(true)
		}
		p, cmd = p.sync(c, false)
	case ui.ReloadMsg:
		if p.open {
			p, cmd = p.sync(c, true)
		} else {
			p.stale = true
		}
	case ui.ReadingMsg:
		p, cmd = p.reading(msg, c)
	case ui.LoadedMsg:
		if msg.Pane != p.k.id {
			return p, nil
		}
		p, cmd = p.loaded(msg)
	case tea.KeyPressMsg:
		p, cmd = p.key(msg, c)
	default:
		return p, nil
	}
	return p.settled(c), cmd
}

// reading takes the viewer and the role the role pane names. An open pane asks
// again when either changed, since the role decides the level a view opens at;
// a closed pane asks when it opens.
func (p Pane) reading(msg ui.ReadingMsg, c ui.Context) (Pane, tea.Cmd) {
	if msg.Viewer == p.viewer && msg.Role == p.role {
		return p, nil
	}
	p.viewer, p.role = msg.Viewer, msg.Role
	if p.open {
		return p.sync(c, true)
	}
	p.stale = true
	return p, nil
}

// toggle answers a digit command: a closed pane opens with the focus, an open
// pane takes the focus, and the pane that has the focus closes.
func (p Pane) toggle(c ui.Context) (Pane, tea.Cmd) {
	id := p.k.id
	switch {
	case !p.open:
		p.open = true
		force := p.stale
		p.stale = false
		var ask tea.Cmd
		p, ask = p.sync(c, force)
		focus := func() tea.Msg { return ui.FocusMsg{Pane: id} }
		return p, tea.Batch(focus, ask)
	case !c.Focused:
		return p, func() tea.Msg { return ui.FocusMsg{Pane: id} }
	}
	p.open = false
	return p, func() tea.Msg { return ui.ShowPaneMsg{Pane: id, Show: false} }
}

// key handles one key press while the pane has the focus.
func (p Pane) key(k tea.KeyPressMsg, c ui.Context) (Pane, tea.Cmd) {
	km := p.keys
	switch {
	case key.Matches(k, km.Go):
		return p.enter()
	case key.Matches(k, km.Fold):
		p.unfold = !p.unfold
		return p.rebuilt(true), nil
	case key.Matches(k, km.Detail):
		next := list.Glance
		if p.shown() == list.Glance {
			next = list.Detail
		}
		return p.choose(next, c)
	case key.Matches(k, km.Provenance):
		if p.k.deepest < list.Provenance {
			return p, notice(noProvenance, false)
		}
		next := list.Provenance
		if p.shown() == list.Provenance {
			next = list.Detail
		} else {
			p.prov = true
		}
		return p.choose(next, c)
	}
	if c.Width <= 0 || c.Height <= 0 {
		return p, nil
	}
	win := p.settle(c.Width, c.Height, c.Measure)
	if len(win.rows) == 0 {
		return p, nil
	}
	page := max(1, c.Height-1)
	i := win.cur
	switch {
	case key.Matches(k, km.Up):
		i--
	case key.Matches(k, km.Down):
		i++
	case key.Matches(k, km.PageUp):
		i -= page
	case key.Matches(k, km.PageDown):
		i += page
	case key.Matches(k, km.First):
		i = 0
	case key.Matches(k, km.Last):
		i = len(win.rows) - 1
	default:
		return p, nil
	}
	i = min(max(i, 0), len(win.rows)-1)
	p = p.stored(win)
	p.cur, p.sub = win.rows[i].line, win.rows[i].sub
	return p, nil
}

// choose sets the level the viewer wants for this pane and draws it, asking
// for it when the data in hand does not hold it.
func (p Pane) choose(l list.Level, c ui.Context) (Pane, tea.Cmd) {
	p.level = &l
	p.failedFor = nil
	p = p.rebuilt(true)
	return p.sync(c, false)
}

// enter selects the task of the cursor line in the grid.
func (p Pane) enter() (Pane, tea.Cmd) {
	body, _ := p.body()
	if p.cur >= len(body) || body[p.cur].Task == "" {
		return p, notice(noTaskOnLine, false)
	}
	l := body[p.cur]
	return p, func() tea.Msg { return ui.GotoMsg{Task: l.Task, Gate: l.Gate} }
}

// Status implements ui.Pane.
func (p Pane) Status(c ui.Context) string {
	s := p.k.id
	if p.showData() {
		s += " " + p.subject + " · " + p.shown().String()
	}
	win := p.settle(c.Width, c.Height, c.Measure)
	if len(win.rows) == 0 {
		return s + " · lines 0 of 0"
	}
	last := min(len(win.rows), win.top+max(c.Height-1, 0))
	s += fmt.Sprintf(" · lines %d–%d of %d", win.top+1, last, len(win.rows))
	body, _ := p.body()
	if l := body[win.rows[win.cur].line]; l.Task != "" {
		s += " · enter selects " + l.Task
		if l.Gate != "" {
			s += " × " + l.Gate
		}
	}
	return s
}

// View implements ui.Pane: the title rule, then h−1 rows behind a gutter.
func (p Pane) View(w, h int, c ui.Context) string {
	if h <= 0 {
		return ""
	}
	m, st := c.Measure, c.Styles
	blank := strings.Repeat(" ", max(w, 0))
	out := make([]string, 0, h)
	if w < 4 {
		for range h {
			out = append(out, blank)
		}
		return strings.Join(out, "\n")
	}
	win := p.settle(w, h, m)
	title := st.Title
	if c.Focused {
		title = st.Cursor
	}
	var rng string
	if rowsH := h - 1; len(win.rows) > rowsH {
		rng = fmt.Sprintf("%d–%d of %d", win.top+1, win.top+max(rowsH, 0), len(win.rows))
	}
	out = append(out, rule(w, p.title(), rng, m, title.Render))

	body, isErr := p.body()
	for k := range h - 1 {
		i := win.top + k
		if i >= len(win.rows) {
			out = append(out, "│ "+m.Fit("", w-2))
			continue
		}
		r := win.rows[i]
		text := "│ " + m.Fit(r.text, w-2)
		switch {
		case c.Focused && i == win.cur:
			text = st.Cursor.Render(text)
		case isErr:
			text = st.Error.Render(text)
		case body[r.line].Head:
			text = st.Header.Render(text)
		}
		out = append(out, text)
	}
	return strings.Join(out, "\n")
}

// rule draws the title rule of width w: a corner, the title, and a line that
// ends in the range of rows shown where it fits. The title takes style.
func rule(w int, title, rng string, m ui.Measure, style func(...string) string) string {
	title = m.Clean(title)
	if m.Width(title) > w-4 {
		title = m.Method.Truncate(title, max(w-4, 0), "…")
	}
	tw := m.Width(title)
	var tail string
	if rw := m.Width(rng); rng != "" && w-tw-rw-7 >= 1 {
		tail = strings.Repeat("─", w-tw-rw-7) + " " + rng + " ─"
	} else {
		tail = strings.Repeat("─", max(w-tw-4, 0))
	}
	return "┌─ " + style(title) + " " + tail
}
