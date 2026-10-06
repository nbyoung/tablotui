package grid

import (
	"fmt"
	"maps"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/nbyoung/tablotui/internal/settings"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view"
)

// Widths of the task column: its floor when the data names no longer label,
// and the width below which the gate columns scroll sideways.
const (
	minTask    = 4
	scrollTask = 12
)

// column is one drawn gate column: a gate the viewer sees, or a maximal run of
// gates folded to a count.
type column struct {
	gates  []int // indexes into the tableau's gates
	header string
	w      int
	folded bool
	count  int // the sum of the run's counts, for a folded column
}

// metrics holds what the grid derives from the data, the choices and the
// measure, and so recomputes only when one of them changes.
type metrics struct {
	measure ui.Measure
	cols    []column
	idW     int
	natural int   // the widest label any row can have, with its count
	below   []int // the rows folded beneath each row
}

// shows reports whether the column of gate g draws: the viewer's choice
// decides, and a gate with no choice follows the window.
func shows(choices map[string]settings.Choice, g view.Gate) bool {
	switch choices[g.Key] {
	case settings.Show:
		return true
	case settings.Hide:
		return false
	}
	return g.Window
}

// descendants counts, for each row, the rows beneath it.
func descendants(rows []view.Row) []int {
	out := make([]int, len(rows))
	for i := range rows {
		for j := i + 1; j < len(rows) && rows[j].Depth > rows[i].Depth; j++ {
			out[i]++
		}
	}
	return out
}

// label returns the task text of a row: the indent, the marker, the title, the
// spine or sibling label, and the count on a folded parent.
func label(r view.Row, folded bool, below int) string {
	var b strings.Builder
	b.WriteString(strings.Repeat("  ", r.Depth))
	switch {
	case r.Parent && folded:
		b.WriteString("▸ ")
	case r.Parent:
		b.WriteString("▾ ")
	default:
		b.WriteString("  ")
	}
	b.WriteString(r.Title)
	if r.Label != "" {
		b.WriteString(" · " + r.Label)
	}
	if r.Parent && folded {
		fmt.Fprintf(&b, " (%d)", below)
	}
	return b.String()
}

func buildMetrics(t *view.Tableau, choices map[string]settings.Choice, m ui.Measure) *metrics {
	mt := &metrics{measure: m, idW: 2, natural: minTask, below: descendants(t.Rows)}
	for i, r := range t.Rows {
		mt.idW = max(mt.idW, m.Width(r.ID))
		mt.natural = max(mt.natural, m.Width(label(r, r.Parent, mt.below[i])))
	}
	for i := 0; i < len(t.Gates); {
		if shows(choices, t.Gates[i]) {
			c := column{gates: []int{i}, header: m.Clean(t.Gates[i].Symbol)}
			c.w = max(2, m.Width(c.header))
			for _, r := range t.Rows {
				c.w = max(c.w, cellWidth(r.Cells[i], m))
			}
			mt.cols = append(mt.cols, c)
			i++
			continue
		}
		j := i
		count := 0
		for j < len(t.Gates) && !shows(choices, t.Gates[j]) {
			count += t.Gates[j].Count
			j++
		}
		c := column{folded: true, count: count}
		for k := i; k < j; k++ {
			c.gates = append(c.gates, k)
		}
		first, last := m.Clean(t.Gates[i].Symbol), m.Clean(t.Gates[j-1].Symbol)
		if j-i == 1 {
			c.header = fmt.Sprintf("%s ×%d", first, count)
		} else {
			c.header = fmt.Sprintf("%s…%s ×%d", first, last, count)
		}
		c.w = m.Width(c.header)
		mt.cols = append(mt.cols, c)
		i = j
	}
	return mt
}

// cellText is the text one gate cell draws: its symbols, in brackets where the
// person acts.
func cellText(c view.Cell, m ui.Measure) string {
	s := m.Clean(c.Symbols)
	if c.Acts && s != "" {
		return "[" + s + "]"
	}
	return s
}

func cellWidth(c view.Cell, m ui.Measure) int { return m.Width(cellText(c, m)) }

// geometry is how the pane places its parts at one size.
type geometry struct {
	taskW    int
	first, n int  // the gate columns drawn: first, first+1, ... first+n-1
	scrolled bool // fewer gate columns than the data holds
	rows     int  // the lines the rows take
	strip    int  // the lines the strip takes
}

// stripLines gives the lines of the strip at each level.
func stripLines(l Level) int {
	switch l {
	case Detail:
		return 5
	case Provenance:
		return 10
	}
	return 0
}

// plan places the parts in a pane of w by h cells. The stored first column
// moves by the least that keeps the cursor column in view.
func (g Grid) plan(w, h int) geometry {
	mt := g.m
	var geo geometry
	avail := max(0, h-2)
	for want := stripLines(g.level); ; {
		if want == 0 || avail-want >= 3 {
			geo.strip = want
			break
		}
		if want == 10 {
			want = 5
		} else {
			want = 0
		}
	}
	geo.rows = avail - geo.strip
	total := 0
	for _, c := range mt.cols {
		total += 1 + c.w
	}
	remain := w - mt.idW - 1 - total
	if mt.natural <= remain {
		geo.taskW, geo.first, geo.n = mt.natural, 0, len(mt.cols)
		return geo
	}
	if remain >= scrollTask {
		geo.taskW, geo.first, geo.n = remain, 0, len(mt.cols)
		return geo
	}
	geo.taskW = scrollTask
	room := w - mt.idW - 1 - scrollTask
	fit := func(first int) int {
		n, used := 0, 0
		for i := first; i < len(mt.cols); i++ {
			if used += 1 + mt.cols[i].w; used > room {
				break
			}
			n++
		}
		return max(n, min(1, len(mt.cols)-first))
	}
	tail := func(first int) int {
		used := 0
		for i := first; i < len(mt.cols); i++ {
			used += 1 + mt.cols[i].w
		}
		return used
	}
	first := min(max(g.first, 0), max(len(mt.cols)-1, 0))
	for first > 0 && tail(first-1) <= room {
		first--
	}
	if c := g.col - 1; c >= 0 && c < len(mt.cols) {
		if c < first {
			first = c
		}
		for first < c && c >= first+fit(first) {
			first++
		}
	}
	geo.first, geo.n = first, fit(first)
	geo.scrolled = geo.n < len(mt.cols)
	return geo
}

// cursorColumn returns the column under the column cursor, or nil when the
// cursor stands on the task column.
func (g Grid) cursorColumn() *column {
	if g.m == nil || g.col < 1 || g.col > len(g.m.cols) {
		return nil
	}
	return &g.m.cols[g.col-1]
}

// choose records or forgets the choice for one gate and saves the settings.
// An empty choice forgets.
func (g Grid) choose(key string, c settings.Choice) (Grid, tea.Cmd) {
	g.choices = maps.Clone(g.choices)
	if c == "" {
		delete(g.choices, key)
	} else {
		g.choices[key] = c
	}
	return g.changed()
}

// changed rebuilds the columns after a choice and saves the settings before
// the update returns.
func (g Grid) changed() (Grid, tea.Cmd) {
	g.m = nil
	if g.path == "" || g.frozen {
		return g, nil
	}
	f := settings.File{Version: settings.Version, Columns: g.choices}
	if err := settings.Save(g.path, f); err != nil {
		return g, notice(fmt.Sprintf("settings: %v", err), true)
	}
	return g, nil
}

// hide hides the gate column at the cursor: it records hide when the window
// shows the gate, and otherwise forgets the show that made it appear.
func (g Grid) hide() (Grid, tea.Cmd) {
	c := g.cursorColumn()
	if c == nil || c.folded {
		return g, notice("the cursor is not on a gate column", false)
	}
	gt := g.data.Gates[c.gates[0]]
	if gt.Window {
		return g.choose(gt.Key, settings.Hide)
	}
	return g.choose(gt.Key, "")
}

// show shows a gate of the folded column at the cursor. It takes the last gate
// of the run when the column is the first gate column of the grid, else the
// first gate of the run; it records show when the window folds that gate, and
// otherwise forgets the hide.
func (g Grid) show() (Grid, tea.Cmd) {
	c := g.cursorColumn()
	if c == nil || !c.folded {
		return g, notice("the cursor is not on a folded column", false)
	}
	i := c.gates[0]
	if g.col == 1 {
		i = c.gates[len(c.gates)-1]
	}
	gt := g.data.Gates[i]
	if gt.Window {
		return g.choose(gt.Key, "")
	}
	return g.choose(gt.Key, settings.Show)
}

// forget returns to the window: it forgets the choice for every gate of the
// project and keeps the keys of any other project.
func (g Grid) forget() (Grid, tea.Cmd) {
	g.choices = maps.Clone(g.choices)
	for _, gt := range g.data.Gates {
		delete(g.choices, gt.Key)
	}
	return g.changed()
}

// changedCount counts the gates of the project that carry a choice.
func (g Grid) changedCount() int {
	n := 0
	for _, gt := range g.data.Gates {
		if _, ok := g.choices[gt.Key]; ok {
			n++
		}
	}
	return n
}
