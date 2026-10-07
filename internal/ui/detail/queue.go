package detail

import (
	"fmt"
	"slices"

	"github.com/nbyoung/tablotui/internal/view/list"
)

// queueKinds are the kinds of a work queue and their headings, in order.
var queueKinds = []struct{ kind, heading string }{
	{"review", "Reviews owed"},
	{"authorisation", "Authorisations owed"},
	{"ready", "Work ready"},
	{"reaffirmation", "Reaffirmations"},
	{"waiting", "Work waiting"},
}

// sentenceEmpty is what an empty queue reads.
const sentenceEmpty = "The queue is empty."

// queueLines builds the work queue at a level. sel is the task the grid
// selects; the entries that name it carry the mark.
func queueLines(d list.Queue, lvl list.Level, unfold bool, sel string) (subject, counts string, lines []Line) {
	w := words{d.Legend}
	subject = d.Params.Person
	counts = count(len(d.Items), "item", "items")
	if len(d.Items) == 0 {
		return subject, counts, []Line{{Text: sentenceEmpty, Item: -1}}
	}

	// The kinds in order, then any kind the table does not know.
	order := make([]struct{ kind, heading string }, len(queueKinds))
	copy(order, queueKinds)
	for _, it := range d.Items {
		if !slices.ContainsFunc(order, func(k struct{ kind, heading string }) bool { return k.kind == it.Kind }) {
			order = append(order, struct{ kind, heading string }{it.Kind, it.Kind})
		}
	}
	for _, k := range order {
		var es []entry
		var items []list.QueueItem
		first := -1
		for i, it := range d.Items {
			if it.Kind != k.kind {
				continue
			}
			if first < 0 {
				first = i
			}
			items = append(items, it)
			es = append(es, w.queueEntry(it, i))
		}
		if len(es) == 0 {
			continue
		}
		if len(lines) > 0 {
			lines = append(lines, Line{Item: -1})
		}
		lines = append(lines, Line{Text: fmt.Sprintf("%s: %d", k.heading, len(es)), Head: true, Item: first})
		byItem := map[int]list.QueueItem{}
		for j, e := range es {
			byItem[e.item] = items[j]
		}
		for _, e := range foldRuns(es, unfold) {
			l := e.line(2, 2)
			l.Marked = sel != "" && (e.task.ID == sel || slices.Contains(e.ids, sel))
			lines = append(lines, l)
			if lvl >= list.Detail && e.task.ID != "" {
				lines = append(lines, w.queueDetail(byItem[e.item], e.item)...)
			}
		}
	}
	return subject, counts, lines
}

// queueEntry reads an item as an entry that opens with its task.
func (w words) queueEntry(it list.QueueItem, i int) entry {
	date := it.Since
	if it.Kind == "reaffirmation" {
		date = join(", ", it.Since, count(it.Age, "day", "days"))
	}
	e := entry{task: it.Task, item: i, gate: it.Gate}
	if it.Cause != "" {
		e.text = task(it.Task) + "; " + it.Cause
	}
	e.rest = cells(w.gate(it.Gate), it.Model, date)
	return e
}

// sourceOf returns who supplies a field of the item's gate: an id, or "default".
func sourceOf(it list.QueueItem, field string) string {
	for _, s := range it.Sources {
		if s.Gate == it.Gate && s.Field == field {
			return s.By
		}
	}
	return ""
}

// from reads where a junction field comes from.
func from(by string) string {
	switch by {
	case "":
		return ""
	case "default":
		return "by default"
	}
	return "from " + by
}

// edgeText reads an entry of a queue item: the task, the edge, the text and
// the condition.
func (w words) edgeText(r list.Requirement) string {
	return join(", ", task(r.Task), w.edge(r), r.Text) + ": " + condition(r)
}

// queueDetail draws what lies beneath an entry at the detail level.
func (w words) queueDetail(it list.QueueItem, item int) []Line {
	var out []Line
	id, gate := it.Task.ID, it.Gate
	add := func(indent int, text string) {
		out = append(out, Line{Text: text, Indent: indent, Hang: 2, Item: item, Task: id, Gate: gate})
	}
	if it.Gate != "" {
		g := ""
		for _, lg := range w.legend.Gates {
			if lg.Key == it.Gate {
				g = join(": ", join(", ", w.gate(it.Gate), lg.Name), lg.Criteria)
			}
		}
		if g == "" {
			g = w.gate(it.Gate)
		}
		add(4, "Gate: "+g)
	}
	if j := it.Junction; j != nil {
		if j.Contributor != "" {
			sym := ""
			if len(j.Marks) > 0 {
				sym = w.symbol(j.Marks[0])
			}
			text := join(" ", sym, j.Contributor)
			if j.Model != "" {
				text += ", model " + j.Model
			}
			if f := from(sourceOf(it, "contributor")); f != "" {
				text += ", " + f
			}
			add(4, "Contributor: "+text)
		}
		switch {
		case j.Reviewer != "" && j.ByDefault:
			add(4, "Reviewer: the assignee, "+j.Reviewer)
		case j.Reviewer != "":
			text := join(" ", w.symbol("reviewer"), j.Reviewer)
			if f := from(sourceOf(it, "reviewer")); f != "" {
				text += ", " + f
			}
			add(4, "Reviewer: "+text)
		}
	}
	var refs []list.Reference
	if it.Junction != nil {
		refs = append(refs, it.Junction.References...)
	}
	refs = append(refs, it.TaskReferences...)
	if len(refs) > 0 {
		out = append(out, Line{Text: "References:", Indent: 4, Item: item, Task: id, Gate: gate})
		for _, r := range refs {
			out = append(out, Line{Text: join(", ", r.Text, r.URL), Indent: 6, Hang: 2, Item: item, Task: id, Gate: gate})
		}
	}
	edges := func(label string, rs []list.Requirement, byFrom bool) {
		if len(rs) == 0 {
			return
		}
		out = append(out, Line{Text: label, Indent: 4, Item: item, Task: id, Gate: gate})
		for _, r := range rs {
			g := r.To
			if byFrom {
				g = r.From
			}
			out = append(out, Line{Text: w.edgeText(r), Indent: 6, Hang: 2, Item: item, Task: r.Task.ID, Gate: g})
		}
	}
	edges("Requires:", it.Requires, true)
	edges("Unblocks:", it.Unblocks, false)
	edges("Also required by:", it.AlsoRequiredBy, false)
	if len(it.ParentUnblocks) > 0 {
		out = append(out, Line{Text: "Its parent unblocks:", Indent: 4, Item: item, Task: id, Gate: gate})
		for _, pe := range it.ParentUnblocks {
			out = append(out, Line{
				Text:   task(pe.Parent) + " at " + w.gate(pe.Gate) + " · " + w.edgeText(pe.Requires),
				Indent: 6, Hang: 2, Item: item, Task: pe.Requires.Task.ID, Gate: pe.Requires.To,
			})
		}
	}
	if s := it.Status; s != nil {
		var hash string
		if it.StatusCommit != nil {
			hash = short(it.StatusCommit.Hash)
		}
		text := join(": ", join(", ", w.status(*s), s.Date, s.Recorder, hash), s.Note)
		add(4, "Status: "+text)
	}
	if it.Kind == "waiting" {
		add(4, "Do not start: the cause stands.")
	}
	return out
}
