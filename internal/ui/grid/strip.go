package grid

import (
	"fmt"
	"strings"

	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view"
)

// gateOf finds a gate by key.
func (g Grid) gateOf(key string) (view.Gate, int, bool) {
	for i, gt := range g.data.Gates {
		if gt.Key == key {
			return gt, i, true
		}
	}
	return view.Gate{}, -1, false
}

// symbolGate writes a gate as its symbol and its key, or its key alone when
// the data names no such gate.
func (g Grid) symbolGate(key string, m ui.Measure) string {
	if gt, _, ok := g.gateOf(key); ok {
		return m.Clean(gt.Symbol) + " " + gt.Key
	}
	return key
}

// words joins the non-empty fields with single spaces.
func words(fields ...string) string {
	var out []string
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return strings.Join(out, " ")
}

// strip draws the lines beneath the rows: the detail of the selected row, and
// at the provenance level the facts of the cell under the cursor. It returns
// exactly n lines, 0, 5 or 10.
func (g Grid) strip(w, n int, c ui.Context) []string {
	if n == 0 {
		return nil
	}
	m := c.Measure
	lines := make([]string, n)
	p := g.selPos()
	if p < 0 {
		lines[0] = strings.Repeat("─", w)
		return lines
	}
	r := g.data.Rows[g.vis[p]]
	col := g.cursorColumn()

	lines[0] = strings.Repeat("─", w)
	cell := ""
	if _, i, ok := g.gateOf(r.Status.Gate); ok {
		cell = m.Clean(r.Cells[i].Symbols)
	}
	state := r.Status.State
	if r.Status.Reason != "" {
		state += ", " + r.Status.Reason
	}
	lines[1] = words(r.ID, r.Title, "·", g.symbolGate(r.Status.Gate, m), cell, state, "·", r.Status.Date)
	note := m.Wrap(r.Status.Note, w, 2)
	lines[2], lines[3] = note[0], note[1]
	switch {
	case col != nil && col.folded:
		var parts []string
		for _, i := range col.gates {
			gt := g.data.Gates[i]
			parts = append(parts, fmt.Sprintf("%s %s %d", m.Clean(gt.Symbol), gt.Key, gt.Count))
		}
		lines[4] = "folded: " + strings.Join(parts, " · ")
	case r.Status.From != nil && r.Status.From.Kind == "rollup":
		lines[4] = words("rolled up from", r.Status.From.ID, r.Status.From.Title)
	case r.Status.From != nil && r.Status.From.Kind == "snapshot":
		f := r.Status.From
		lines[4] = words("snapshot of", f.ID, f.Title, "at pin", f.Pin+",", g.symbolGate(f.Gate, m), "there")
	}
	if n < 10 {
		return lines
	}
	switch {
	case col == nil || col.folded:
		lines[5] = "move the column cursor to a gate cell"
	default:
		facts := r.Cells[col.gates[0]].Facts
		if len(facts) == 0 {
			lines[5] = "this cell holds no fact"
			break
		}
		lines[5] = "provenance of " + r.ID + " at " + g.symbolGate(g.data.Gates[col.gates[0]].Key, m)
		shown := facts
		if len(facts) > 4 {
			shown = facts[:3]
		}
		for k, f := range shown {
			lines[6+k] = "  " + f.Label + ": " + f.Text
		}
		if len(facts) > 4 {
			lines[9] = fmt.Sprintf("  … %d more", len(facts)-3)
		}
	}
	return lines
}
