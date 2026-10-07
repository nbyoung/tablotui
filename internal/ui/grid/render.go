package grid

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/nbyoung/tablotui/internal/ui"
)

// seg is one piece of a line: padded text and the style it takes.
type seg struct {
	text  string
	style *lipgloss.Style // nil draws plain text
}

// join draws segments with one plain space between them.
func join(segs []seg) string {
	var b strings.Builder
	for i, s := range segs {
		if i > 0 {
			b.WriteByte(' ')
		}
		if s.style == nil {
			b.WriteString(s.text)
		} else {
			b.WriteString(s.style.Render(s.text))
		}
	}
	return b.String()
}

// View implements ui.Pane: exactly h lines of exactly w cells. It settles a
// copy of the grid and changes no state.
func (g Grid) View(w, h int, c ui.Context) string {
	g = g.settled(w, h, c.Measure)
	m, st := c.Measure, c.Styles
	lines := make([]string, 0, h)
	if g.data == nil {
		text, style := "loading…", lipgloss.NewStyle()
		if g.loadErr != "" {
			text, style = "error: "+g.loadErr, st.Error
		}
		lines = append(lines, style.Render(m.Fit(text, w)))
		return pad(lines, w, h, m)
	}
	geo := g.plan(w, h)
	cols := g.m.cols[geo.first : geo.first+geo.n]

	head := []seg{{m.Fit("Id", g.m.idW), &st.Header}, {m.Fit("Task", geo.taskW), &st.Header}}
	rule := []seg{{strings.Repeat("─", g.m.idW), nil}, {strings.Repeat("─", geo.taskW), nil}}
	if g.col == 0 {
		head[1].style = &st.Cursor
	}
	for i, col := range cols {
		style := &st.Header
		if g.col == geo.first+i+1 {
			style = &st.Cursor
		}
		head = append(head, seg{m.Centre(col.header, col.w), style})
		rule = append(rule, seg{strings.Repeat("─", col.w), nil})
	}
	lines = append(lines, join(head), join(rule))

	area := make([]string, geo.rows)
	if len(g.vis) == 0 && len(area) > 0 {
		area[0] = m.Fit("no task in view", w)
	}
	for k := range area {
		p := g.top + k
		if p >= len(g.vis) {
			break
		}
		area[k] = g.rowLine(g.vis[p], geo, cols, c)
	}
	lines = append(lines, area...)
	lines = append(lines, g.strip(w, geo.strip, c)...)
	return pad(lines, w, h, m)
}

// rowLine draws one row.
func (g Grid) rowLine(i int, geo geometry, cols []column, c ui.Context) string {
	m, st := c.Measure, c.Styles
	r := g.data.Rows[i]
	selected := r.ID == g.sel
	var idStyle, taskStyle *lipgloss.Style
	if r.Parent {
		taskStyle = &st.Parent
	}
	if selected {
		idStyle, taskStyle = &st.Cursor, &st.Cursor
		if r.Parent {
			both := st.Cursor.Inherit(st.Parent)
			taskStyle = &both
		}
	}
	text := label(r, r.Parent && !g.open[r.ID], g.m.below[i])
	segs := []seg{{m.Fit(r.ID, g.m.idW), idStyle}, {m.Fit(text, geo.taskW), taskStyle}}
	for k, col := range cols {
		var s seg
		s.text = m.Centre("", col.w)
		if !col.folded {
			cell := r.Cells[col.gates[0]]
			s.text = m.Centre(cellText(cell, m), col.w)
			if cell.Acts {
				s.style = &st.Acts
			}
		}
		if selected && g.col == geo.first+k+1 {
			s.style = &st.Cursor
		}
		segs = append(segs, s)
	}
	return join(segs)
}

// pad cuts or pads lines to exactly h lines of exactly w cells.
func pad(lines []string, w, h int, m ui.Measure) string {
	out := make([]string, max(h, 0))
	for i := range out {
		if i < len(lines) {
			out[i] = m.Fit(lines[i], w)
		} else {
			out[i] = strings.Repeat(" ", max(w, 0))
		}
	}
	return strings.Join(out, "\n")
}

// Status implements ui.Pane.
func (g Grid) Status(c ui.Context) string {
	g = g.settled(c.Width, c.Height, c.Measure)
	if g.data == nil {
		return ""
	}
	geo := g.plan(c.Width, c.Height)
	var parts []string
	rows := "rows 0 of 0"
	if n := len(g.vis); n > 0 {
		shown := max(0, min(geo.rows, n-g.top))
		rows = fmt.Sprintf("rows %d–%d of %d", g.top+1, g.top+shown, n)
		id := g.sel
		switch col := g.cursorColumn(); {
		case col == nil:
		case col.folded:
			id += " × " + col.header
		default:
			gt := g.data.Gates[col.gates[0]]
			id += " × " + c.Measure.Clean(gt.Symbol) + " " + gt.Key
		}
		parts = append(parts, id)
	}
	if geo.scrolled {
		parts = append(parts, fmt.Sprintf("columns %d–%d of %d", geo.first+1, geo.first+geo.n, len(g.m.cols)))
	}
	window := fmt.Sprintf("window %d", g.data.Window)
	if k := g.changedCount(); k > 0 {
		window += fmt.Sprintf(", %d changed", k)
	}
	historical := "historical off"
	if g.data.Historical {
		historical = "historical on"
	}
	parts = append(parts, rows, window, historical)
	return strings.Join(parts, " · ")
}
