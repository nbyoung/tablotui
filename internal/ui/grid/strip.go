package grid

import (
	"fmt"
	"strings"

	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view"
)

// gateOf finds a gate of the legend by key.
func (g Grid) gateOf(key string) (view.Gate, int, bool) {
	for i, gt := range g.data.Legend.Gates {
		if gt.Key == key {
			return gt, i, true
		}
	}
	return view.Gate{}, -1, false
}

// symbolGate writes a gate as its symbol and its key, or its key alone when
// the data names no such gate.
func (g Grid) symbolGate(key string, m ui.Measure) string {
	if gt, _, ok := g.gateOf(key); ok && gt.Symbol != "" {
		return m.Clean(gt.Symbol) + " " + gt.Key
	}
	return key
}

// words joins the non-empty fields with single spaces.
func words(fields ...string) string { return strings.Join(nonEmpty(fields...), " ") }

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
		c := r.Cells[i]
		c.Acts = false
		cell = g.m.cellText(c, m)
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
			gt := g.data.Legend.Gates[i]
			parts = append(parts, fmt.Sprintf("%s %s %d", m.Clean(gt.Symbol), gt.Key, g.m.count[i]))
		}
		lines[4] = "folded: " + strings.Join(parts, " · ")
	case r.Status.From != nil && r.Status.From.Kind == "rollup":
		leaf := r.Status.From.Task
		if r.Leaf != nil {
			leaf = r.Leaf.Task
		}
		lines[4] = words("rolled up from", leaf.ID, leaf.Title)
	case r.Status.From != nil && r.Status.From.Kind == "snapshot":
		f := r.Status.From
		lines[4] = words("snapshot of", f.Task.ID, f.Task.Title, "at pin", short(f.Pin)+",", g.symbolGate(f.Gate, m), "there")
	}
	if n < 10 {
		return lines
	}
	switch {
	case col == nil || col.folded:
		lines[5] = "move the column cursor to a gate cell"
	default:
		facts := g.facts(r, col.gates[0], m)
		if len(facts) == 0 {
			lines[5] = "this cell holds no fact"
			break
		}
		lines[5] = "provenance of " + r.ID + " at " + g.symbolGate(g.data.Legend.Gates[col.gates[0]].Key, m)
		shown := facts
		if len(facts) > 4 {
			shown = facts[:3]
		}
		for k, f := range shown {
			lines[6+k] = "  " + f.label + ": " + f.text
		}
		if len(facts) > 4 {
			lines[9] = fmt.Sprintf("  … %d more", len(facts)-3)
		}
	}
	return lines
}

// fact is one line of a cell's provenance: a label and its text.
type fact struct{ label, text string }

// statedBy words the sources of a junction field that are no task, as
// tabloio does.
var statedBy = map[string]string{
	"default":  "the plain default",
	"own":      "each task's own file",
	"assignee": "the assignee",
}

// markFrom words where a field comes from, after its value: ", from <id>",
// ", by default", or ", from " and the phrase of statedBy.
func markFrom(src string) string {
	switch src {
	case "":
		return ""
	case "default":
		return ", by default"
	}
	if w, ok := statedBy[src]; ok {
		return ", from " + w
	}
	return ", from " + src
}

// actor reads a person of a commit: the name and the email, or the email.
func actor(p view.Person) string {
	if p.Name == "" || p.Email == "" {
		return p.Name + p.Email
	}
	return p.Name + ", " + p.Email
}

// phrase reads a status as its gate, state and reason, each as symbol and
// key; a status with no state reads as its gate alone.
func (g Grid) phrase(st view.Status, m ui.Measure) string {
	key := func(table map[string]string, k string) string {
		if s := table[k]; s != "" {
			return m.Clean(s) + " " + k
		}
		return k
	}
	out := g.symbolGate(st.Gate, m)
	if st.State == "" {
		return out
	}
	out += " " + key(g.m.symbols.states, st.State)
	if st.Reason != "" {
		out += " " + key(g.m.symbols.reasons, st.Reason)
	}
	return out
}

// facts words the provenance of row r at gate k. tablo gives each row the
// task definition's facts, and the strip takes those of the cell under the
// cursor, under the headings and in the words of tabloio's grid (a3cc
// decision 9): a status cell gives the deciding commit of the status the row
// records, and for a roll-up or a snapshot where it comes from; a cell of
// marks, or an exempt one, gives its junction and the task that supplies
// each field. An empty cell gives nothing.
func (g Grid) facts(r view.Row, k int, m ui.Measure) []fact {
	var out []fact
	add := func(label, text string) {
		if text != "" {
			out = append(out, fact{label, text})
		}
	}
	key := g.data.Legend.Gates[k].Key
	switch r.Cells[k].Kind {
	case "status":
		from := r.Status.From
		c := r.StatusCommit
		if from != nil {
			c = nil
			if from.Kind == "snapshot" {
				c = r.OwnCommit
			}
		}
		if c != nil {
			add("Date", c.Date)
			add("Recorder", actor(c.Author))
			add("Deciding commit", words(short(c.Hash), c.Subject))
			add("Committer", actor(c.Committer))
			add("Trailers", strings.Join(c.Trailers, ", "))
		}
		if from == nil {
			break
		}
		// The second status of the chain is the direct child's, or the
		// subproject task's that the snapshot reads.
		next, nextStatus := from.Task, r.Status
		if len(r.Chain) > 1 {
			next, nextStatus = r.Chain[1].Task, r.Chain[1].Status
		}
		switch from.Kind {
		case "rollup":
			add("Shows", g.phrase(r.Status, m))
			add("Rolls up from", words(next.ID, next.Title))
			add("Date", r.Status.Date)
		case "snapshot":
			add("Subproject", from.URL)
			add("Pin", short(from.Pin))
			for _, l := range r.Linkages {
				if s := l.SetBy; l.URL == from.URL && s != nil {
					add("Pin set by", strings.Join(nonEmpty(short(s.Hash), s.Date, s.Author.Email), ", "))
				}
			}
			add("Root task at the pin", words(next.ID, next.Title)+": "+g.phrase(nextStatus, m))
			if nextStatus.From != nil {
				add("Rolls up from", words(nextStatus.From.Task.ID, nextStatus.From.Task.Title))
			}
		}
	case "marks", "exempt":
		var j *view.Junction
		for i := range r.Junctions {
			if r.Junctions[i].Gate == key {
				j = &r.Junctions[i]
			}
		}
		if j == nil {
			break
		}
		supplied := func(field string) string {
			for _, s := range r.Sources {
				if s.Gate == key && s.Field == field {
					return s.By
				}
			}
			return ""
		}
		if r.Cells[k].Kind == "exempt" {
			add("Contributor and model", "Exempt"+markFrom(supplied("applies")))
			break
		}
		if cm := strings.Join(nonEmpty(j.Contributor, j.Model), ", "); cm != "" {
			add("Contributor and model", cm+markFrom(supplied("contributor")))
		}
		if j.Reviewer != "" {
			add("Reviewer", j.Reviewer+markFrom(supplied("reviewer")))
		}
	}
	return out
}

// nonEmpty returns the fields that are not empty.
func nonEmpty(fields ...string) []string {
	var out []string
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}
