package detail

import (
	"fmt"

	"github.com/nbyoung/tablotui/internal/view/list"
)

// sentenceNoCause is what a tree with no cause reads.
const sentenceNoCause = "No cause holds any task."

// cause reads a cause as one sentence, by its kind.
func (w words) cause(c list.Cause) string {
	t := task(c.Task)
	switch c.Kind {
	case "requirement":
		return t + " has not passed " + w.gate(c.Gate)
	case "status":
		var status, note string
		if c.Status != nil {
			status, note = w.status(*c.Status), c.Status.Note
		}
		return t + " stands at " + join(": ", status, note)
	case "review":
		return t + " awaits review at " + w.gate(c.Gate)
	case "authorisation":
		return t + " is proposed"
	case "snapshot":
		return t + " waits on " + c.URL + " at " + short(c.Pin)
	}
	return join(" ", t, c.Kind)
}

// causeGate is the gate a cause names: the gate not passed, or the gate of
// the status it states.
func causeGate(c list.Cause) string {
	if c.Gate == "" && c.Status != nil {
		return c.Status.Gate
	}
	return c.Gate
}

// blockageSubject is the title's subject: the task of the request, or the
// root where the view covers it.
func blockageSubject(h list.Head) string {
	if h.Params.Task != "" {
		return h.Params.Task
	}
	return h.Project.ID
}

// blockageLines builds the work-blockage tree at a level.
func blockageLines(d list.Blockage, lvl list.Level) (subject, counts string, lines []Line) {
	w := words{d.Legend}
	subject = blockageSubject(d.Head)
	counts = count(len(d.Causes), "cause", "causes")
	if len(d.Causes) == 0 {
		return subject, counts, []Line{{Text: sentenceNoCause, Item: -1}}
	}
	add := func(l Line) { lines = append(lines, l) }
	n := len(d.Causes)
	for i, c := range d.Causes {
		gate := causeGate(c)
		resolver := join(" ", w.symbol(c.Mark), c.Resolver, c.Act)
		if lvl < list.Detail {
			add(Line{
				Text:   cells(w.cause(c), resolver, fmt.Sprintf("holds %d", c.Holds)),
				Indent: 0, Hang: 2, Item: i, Task: c.Task.ID, Gate: gate,
			})
			continue
		}
		if i > 0 {
			add(Line{Item: -1})
		}
		add(Line{
			Text: cells(w.cause(c), resolver, fmt.Sprintf("holds %d", c.Holds)),
			Hang: 2, Head: true, Item: i, Task: c.Task.ID, Gate: gate,
		})
		if c.Action != "" {
			add(Line{Text: "Action: " + c.Action, Indent: 2, Hang: 2, Item: i, Task: c.Task.ID, Gate: gate})
		}
		var held func(hs []list.Held, depth int)
		held = func(hs []list.Held, depth int) {
			for _, h := range hs {
				var what string
				switch h.Via {
				case "self":
					what = "the cause holds the task itself."
				case "parent":
					what = "the parent of " + h.Child + "."
				case "requirement":
					if r := h.Requires; r != nil {
						what = "requires " + join(", ", join(" ", r.Task.ID, "at", w.gate(r.From)), r.Text) + "."
					}
				}
				text := task(h.Task) + " at " + w.gate(h.Gate) + ": " + what
				for _, a := range h.Also {
					text += fmt.Sprintf(" Also under cause %d.", a)
				}
				add(Line{Text: text, Indent: 2 + 2*depth, Hang: 2, Item: i, Task: h.Task.ID, Gate: h.Gate})
				held(h.Held, depth+1)
			}
		}
		held(c.Held, 0)
		if lvl >= list.Provenance {
			for _, f := range c.Facts {
				add(Line{Text: f.Name + ". " + f.Value, Indent: 2, Hang: 2, Item: i, Task: c.Task.ID, Gate: gate})
			}
			lines = append(lines, commandLines(c.Resolve, 4, 2, i, c.Task.ID, gate)...)
		}
	}
	if lvl >= list.Detail && len(d.NotDue) > 0 {
		add(Line{Item: -1})
		add(Line{Text: "Next: " + count(d.NotDueN, "requirement", "requirements") + " not yet due", Head: true, Item: n})
		for _, wt := range d.NotDue {
			add(Line{
				Text:   task(wt.Task) + " stands at " + w.status(wt.Status) + "; next " + w.gate(wt.Next) + ".",
				Indent: 2, Hang: 2, Item: n, Task: wt.Task.ID, Gate: wt.Next,
			})
			for _, r := range wt.Requires {
				text := w.gate(r.To) + ": requires " + join(", ", join(" ", task(r.Task), "at", w.gate(r.From)), r.Text) + ": " + condition(r) + "."
				add(Line{Text: text, Indent: 4, Hang: 2, Item: n, Task: r.Task.ID, Gate: r.From})
			}
		}
	} else if d.NotDueN != 0 {
		add(Line{Item: -1})
		add(Line{
			Text: fmt.Sprintf("Next: %s not yet due, %d unmet.", count(d.NotDueN, "requirement", "requirements"), d.NotDueUnmet),
			Item: n,
		})
	}
	if lvl >= list.Provenance && len(d.Commands) > 0 {
		add(Line{Item: -1})
		add(Line{Text: "Commands", Head: true, Item: n + 1})
		lines = append(lines, commandLines(d.Commands, 2, 2, n+1, "", "")...)
	}
	return subject, counts, lines
}
