package detail

import (
	"fmt"
	"slices"
	"strings"

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

// queueEntry reads an item as an entry that opens with its task. An item
// with a cause never folds, as in tabloio: the cause belongs to its task, and
// an authorisation always carries one.
func (w words) queueEntry(it list.QueueItem, i int) entry {
	date := it.Since
	if it.Kind == "reaffirmation" {
		date = join(", ", it.Since, count(it.Age, "day", "days"))
	}
	e := entry{task: it.Task, item: i, gate: it.Gate}
	if c := w.queueCause(it); c != "" {
		e.text, e.alone = task(it.Task)+"; "+c, true
	}
	e.rest = cells(w.gate(it.Gate), it.Model, date)
	return e
}

// authorisationOwed words why an authorisation is owed, by the reason no
// commit decides the task; "" for a deciding commit by no authority, as
// tabloio has it.
var authorisationOwed = map[string]string{
	"":           "proposed; the deciding commit is not by an authority",
	"no_history": "proposed; the source has no history",
	"no_trunk":   "proposed; the trunk is undetermined",
	"off_trunk":  "proposed; the ref stands off the trunk",
	"no_commit":  "proposed; no commit on the trunk decides it",
}

// queueCause words why an item waits, or why an authorisation is owed, in
// tabloio's words, from the item's own facts at every level (886d decision
// 15): each cause of a waiting item, joined by "; ", a reason with its
// wait's note; for an authorisation, why its task stands proposed, from the
// item's why. It reads nothing for a waiting item with no cause.
func (w words) queueCause(it list.QueueItem) string {
	if it.Kind == "authorisation" {
		return authorisationOwed[it.Why]
	}
	var runs []string
	for _, c := range it.Causes {
		switch c.Kind {
		case "requirement":
			if r := c.Requires; r != nil {
				x := "waits for " + task(r.Task)
				if r.Subproject != "" {
					x += " in " + r.Subproject
				}
				runs = append(runs, x+" at "+r.From)
			}
		case "reason":
			x := c.Reason
			if c.Note != "" {
				x += ": " + c.Note
			}
			runs = append(runs, x)
		case "authorisation":
			runs = append(runs, join("", "proposed; waits for its authorisation by ", c.By))
		case "review":
			runs = append(runs, join("", "handed off; waits for the review by ", c.By))
		}
	}
	return strings.Join(runs, "; ")
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

// edgeText reads an entry of a queue item: the task, in its project for a
// cross-project entry, the edge, the text and the condition.
func (w words) edgeText(r list.Requirement) string {
	t := task(r.Task)
	if r.Subproject != "" {
		t += " in " + r.Subproject
	}
	return join(", ", t, w.edge(r), r.Text) + ": " + condition(r)
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
		if j.Contributor != "" || j.Model != "" {
			var sym string
			for _, m := range j.Marks {
				if m == "person" || m == "agent" {
					sym = w.symbol(m)
					break
				}
			}
			text := join(" ", sym, j.Contributor)
			if j.Model != "" {
				text = join(", ", text, "model "+j.Model)
			}
			add(4, "Contributor: "+text+markFrom(sourceOf(it, "contributor")))
		}
		if j.Reviewer != "" {
			text := join(" ", w.symbol("reviewer"), j.Reviewer)
			if j.ByDefault {
				text += ", the assignee"
			} else {
				text += markFrom(sourceOf(it, "reviewer"))
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
				Text:   task(pe.Parent) + " at " + w.gate(pe.Requires.From) + " is required by " + w.edgeText(pe.Requires),
				Indent: 6, Hang: 2, Item: item, Task: pe.Requires.Task.ID, Gate: pe.Requires.To,
			})
		}
	}
	if s := it.Status; s != nil {
		var hash string
		if it.StatusCommit != nil {
			hash = short(it.StatusCommit.Hash)
		}
		text := join(": ", join(", ", w.phrase(*s), s.Date, s.Recorder, hash), s.Note)
		add(4, "Status: "+text)
	}
	if it.Kind == "waiting" {
		add(4, "Do not start: the cause stands.")
	}
	return out
}
