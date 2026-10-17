package detail

import (
	"fmt"
	"strings"

	"github.com/nbyoung/tablotui/internal/view/list"
)

// words speaks tabloio's words: it turns the keys of a view's data into the
// symbol-and-key form the legend fixes, and the status, the requirement and
// the effect into the phrases tabloio's renderers print.
type words struct{ legend list.Legend }

// gate reads a gate as its symbol, a space and its key; the key alone when the
// legend holds no symbol.
func (w words) gate(key string) string {
	for _, g := range w.legend.Gates {
		if g.Key == key {
			return symbolKey(g.Symbol, key)
		}
	}
	return key
}

func (w words) state(key string) string {
	for _, s := range w.legend.States {
		if s.Key == key {
			return symbolKey(s.Symbol, key)
		}
	}
	return key
}

func (w words) reason(key string) string {
	for _, r := range w.legend.Reasons {
		if r.Key == key {
			return symbolKey(r.Symbol, key)
		}
	}
	return key
}

// symbol returns the symbol of a mark, or "" when the legend holds none.
func (w words) symbol(markKey string) string {
	for _, m := range w.legend.Marks {
		if m.Key == markKey {
			return m.Symbol
		}
	}
	return ""
}

// marks returns the symbols of the keys with no space between them.
func (w words) marks(keys []string) string {
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(w.symbol(k))
	}
	return b.String()
}

func symbolKey(symbol, key string) string {
	if symbol == "" {
		return key
	}
	return symbol + " " + key
}

// phrase reads a status as its gate, then its state and its reason; a
// status with no state reads as its gate alone (tabloio R3).
func (w words) phrase(s list.Status) string {
	if s.State == "" {
		return w.gate(s.Gate)
	}
	return join(" ", w.gate(s.Gate), w.state(s.State), w.reason(s.Reason))
}

// status reads a status: its phrase, then where a derived status comes from.
func (w words) status(s list.Status) string {
	out := w.phrase(s)
	if o := s.From; o != nil {
		switch o.Kind {
		case "rollup":
			out += ", rolled up from " + task(o.Task)
		case "snapshot":
			out += ", " + join(" ", w.symbol("subproject"), "from the subproject", o.URL)
		}
	}
	return out
}

// recorded reads when a status was recorded: the date and the recorder; for
// a roll-up the date, the oldest among its children; for a snapshot the
// date, from the subproject.
func recorded(s list.Status) string {
	if s.Date == "" {
		return ""
	}
	if s.From != nil {
		switch s.From.Kind {
		case "rollup":
			return s.Date + ", the oldest among its children"
		case "snapshot":
			return s.Date + ", from the subproject"
		}
	}
	return join(" by ", s.Date, s.Recorder)
}

// plain drops the backticks with which tabloio marks code in a phrase, as
// its text renderer does.
func plain(s string) string { return strings.ReplaceAll(s, "`", "") }

// statedBy words the sources of a junction field that are no task.
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

// commitLine reads a commit on one line: the short hash, the subject, the
// date, and the author and the committer, as one person when they agree.
func commitLine(c list.Commit) string {
	out := join(", ", join(" ", short(c.Hash), c.Subject), c.Date)
	if c.Author == c.Committer {
		if a := person(c.Author); a != "" {
			out += ", author and committer " + a
		}
		return out
	}
	if a := person(c.Author); a != "" {
		out += ", author " + a
	}
	if a := person(c.Committer); a != "" {
		out += ", committer " + a
	}
	return out
}

// condition reads the state of a requirement's edge.
func condition(r list.Requirement) string {
	s := "unmet"
	if r.Met {
		s = "met"
	}
	if !r.Due {
		s += ", not yet due"
	}
	return s
}

// edge reads the keys of a requirement's gates: the gate of the required task,
// an arrow, the gate of the dependent.
func (w words) edge(r list.Requirement) string { return r.From + " → " + r.To }

// pin reads a subproject pin that moves, or that first appears.
func pin(p *list.PinMove) string {
	if p == nil {
		return ""
	}
	if p.Old == "" {
		return join(" ", p.URL, "new", short(p.New))
	}
	return join(" ", p.URL, short(p.Old), "→", short(p.New))
}

// task reads a task as its id, a space and its title; the title alone when
// the id is empty, as on a fold line.
func task(t list.TaskRef) string { return join(" ", t.ID, t.Title) }

// subTask reads the subproject task an event comes from as a task: its id,
// and its title where the subproject's files state one, as tabloio's
// subTask does.
func subTask(s list.SubEvent) string { return task(list.TaskRef{ID: s.Task, Title: s.Title}) }

// short is the first seven characters of a commit hash.
func short(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// count writes n and the word that agrees with it: "1 cause", "2 causes".
func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// counted writes a heading with its count, or "none".
func counted(name string, n int) string {
	if n == 0 {
		return name + ": none"
	}
	return fmt.Sprintf("%s: %d", name, n)
}

// join joins the non-empty parts.
func join(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// cells joins the non-empty cells of what the Markdown draws as a table row.
func cells(parts ...string) string { return join(" · ", parts...) }

// entry is one row of a list that may fold: the task and the cells that
// follow it.
type entry struct {
	task  list.TaskRef
	text  string // the first cell where it is not the task
	rest  string
	item  int
	gate  string
	ids   []string // on a fold line: the ids the line stands for
	alone bool     // the entry carries a cause of its task's, so it never folds
}

// first is the cell that opens the entry.
func (e entry) first() string {
	if e.text != "" {
		return e.text
	}
	return task(e.task)
}

// line draws the entry as one line at the indent of a list.
func (e entry) line(indent, hang int) Line {
	return Line{
		Text: cells(e.first(), e.rest), Indent: indent, Hang: hang,
		Item: e.item, Task: e.task.ID, Gate: e.gate,
	}
}

// foldRuns applies the run fold: a run of more than eight consecutive entries
// that agree in everything but the task keeps its first three, and one entry
// with an empty task id stands for the others, its title listing their ids.
// An entry that stands alone belongs to no run.
func foldRuns(in []entry, unfold bool) []entry {
	if unfold {
		return in
	}
	var out []entry
	for i := 0; i < len(in); {
		j := i + 1
		for j < len(in) && !in[i].alone && !in[j].alone && in[j].rest == in[i].rest {
			j++
		}
		if j-i <= 8 {
			out = append(out, in[i:j]...)
			i = j
			continue
		}
		out = append(out, in[i:i+3]...)
		ids := make([]string, 0, j-i-3)
		for _, e := range in[i+3 : j] {
			ids = append(ids, e.task.ID)
		}
		out = append(out, entry{
			task: list.TaskRef{Title: fmt.Sprintf("… and %d more: %s", j-i-3, strings.Join(ids, " "))},
			rest: in[i].rest,
			item: in[i+3].item,
			ids:  ids,
		})
		i = j
	}
	return out
}
