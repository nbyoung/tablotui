package detail

import (
	"fmt"
	"strings"

	"github.com/nbyoung/tablotui/internal/view/list"
)

// sentenceNoCause is what a tree with no cause reads.
const sentenceNoCause = "No cause holds any task."

// cause reads a cause as one sentence, by its kind.
func (w words) cause(c list.Cause) string {
	t := task(c.Task)
	switch c.Kind {
	case "requirement":
		if c.URL != "" {
			t += " in " + c.URL
		}
		return t + " has not passed " + w.gate(c.Gate)
	case "status":
		return t + " stands at " + join(": ", w.phrase(c.Status), c.Status.Note)
	case "review":
		return t + " awaits review at " + w.gate(c.Gate)
	case "authorisation":
		return t + " is proposed"
	case "snapshot":
		return t + " waits on " + c.URL + " at " + short(c.Pin)
	}
	return join(" ", t, c.Kind)
}

// action reads the one action that resolves a cause, by its act, in
// tabloio's words; "" where the files name no resolver.
func action(c list.Cause) string {
	if c.Resolver == "" {
		return ""
	}
	id := c.Task.ID
	var rest string
	switch c.Act {
	case "contributes":
		rest = " takes " + id + " through " + c.Gate + "."
	case "records":
		rest = " records a new status."
		if c.Status.Reason != "" {
			rest = " clears the " + c.Status.Reason + " reason, or records a new status."
		}
	case "reviews":
		rest = " accepts the " + c.Gate + " with a commit that carries Reviewed: " + id + " " + c.Gate + "."
	case "authorises":
		rest = " commits the trailer Authorised: " + id + "."
	case "advances":
		switch c.Advance {
		case "pin":
			rest = " advances the pin of " + c.URL + " to the tip of its trunk"
			if c.Tip != "" {
				rest += ", " + short(c.Tip)
			}
			rest += "."
		case "gate":
			rest = " records " + id + " past " + c.Gate + ", where " + c.URL + " already stands."
		}
	}
	if rest == "" {
		return ""
	}
	return c.Resolver + rest
}

// authorisationWhy words why no commit decides a proposed task, as tabloio
// does.
var authorisationWhy = map[string]string{
	"no_history": "no commit decides it: the source has no history",
	"no_trunk":   "no commit decides it: the trunk is undetermined",
	"off_trunk":  "no commit decides it: the ref stands off the trunk",
	"no_commit":  "no commit on the trunk decides it",
}

// facts reads the typed facts behind a cause at provenance, each as a name
// and its value, in tabloio's words: the status, the next junction, the
// authorisation and the linkage, each where the cause holds it.
func (w words) facts(c list.Cause) [][2]string {
	var out [][2]string
	add := func(name, value string) {
		if value != "" {
			out = append(out, [2]string{name, value + "."})
		}
	}
	if st := c.Status; st.Gate != "" {
		line := join(", recorded ", join(": ", w.status(st), st.Note), recorded(st))
		if c.StatusCommit != nil {
			line += ", by the commit " + commitLine(*c.StatusCommit)
		}
		add("The status", line)
	}
	if j := c.Junction; j != nil {
		add("The junction", c.Task.ID+" at "+w.junctionFields(*j))
	}
	if a := c.Authorisation; a != nil {
		add("The authorisation", authorisationLine(*a))
	}
	if l := c.Linkage; l != nil {
		line := task(l.Task) + " in " + l.URL
		if l.Commit != "" {
			line += " at " + short(l.Commit)
		}
		if l.Trunk != "" {
			line += ", on the trunk " + l.Trunk
			if l.Tip != "" {
				line += " whose tip is " + short(l.Tip)
			}
			if b := linkBehind[l.Behind]; b != "" {
				line += ", " + b
			}
		}
		add("The linkage", line)
	}
	return out
}

// junctionFields reads a junction after "at": its gate, then its
// contributor, model and reviewer, and the tasks whose entries supply it.
func (w words) junctionFields(j list.Junction) string {
	var fields []string
	if j.Contributor != "" {
		fields = append(fields, "contributor "+j.Contributor)
	}
	if j.Model != "" {
		fields = append(fields, "model "+j.Model)
	}
	if j.Reviewer != "" {
		r := "reviewer " + j.Reviewer
		if j.ByDefault {
			// The line names no assignee and no supplier, so it says the
			// default supplies the reviewer, the assignee or the nearest
			// person (cdb0 decision 2).
			r += ", " + reviewerDefaults[""]
		}
		fields = append(fields, r)
	}
	if j.Subproject != nil {
		fields = append(fields, "the subproject "+j.Subproject.URL+" "+task(j.Subproject.Task))
	}
	out := join(": ", w.gate(j.Gate), strings.Join(fields, ", "))
	if len(j.From) > 0 {
		out += ", from " + strings.Join(j.From, " ")
	}
	return out
}

// authorisationLine reads an authorisation on one line: its deciding commit
// and how it accepts, or why none decides.
func authorisationLine(a list.Authorisation) string {
	if a.Commit == nil {
		return authorisationWhy[a.Why]
	}
	out := commitLine(*a.Commit)
	if a.By == "" {
		out += ", not by an authority"
	}
	if way := acceptsBy[a.Way]; way != "" {
		out += ", accepting by " + plain(way)
	}
	if by := authorityIs[a.By]; by != "" {
		out += ", the authority being " + by
	}
	return out
}

// blockageSubject is the title's subject: the task of the request, or the
// root where the view covers it.
func blockageSubject(h list.Head) string {
	if h.Params.Task != "" {
		return h.Params.Task
	}
	return h.Project.Root
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
		gate := c.Gate
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
		if a := action(c); a != "" {
			add(Line{Text: "Action: " + a, Indent: 2, Hang: 2, Item: i, Task: c.Task.ID, Gate: gate})
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
			for _, f := range w.facts(c) {
				add(Line{Text: f[0] + ". " + f[1], Indent: 2, Hang: 2, Item: i, Task: c.Task.ID, Gate: gate})
			}
			lines = append(lines, commandLines(c.Resolve, 4, 2, i, c.Task.ID, gate)...)
		}
	}
	if lvl >= list.Detail && len(d.NotDue) > 0 {
		add(Line{Item: -1})
		add(Line{Text: "Next: " + count(d.NotDueN, "requirement", "requirements") + " not yet due", Head: true, Item: n})
		for _, wt := range d.NotDue {
			add(Line{
				Text:   task(wt.Task) + " stands at " + w.phrase(wt.Status) + "; next " + w.gate(wt.Next) + ".",
				Indent: 2, Hang: 2, Item: n, Task: wt.Task.ID, Gate: wt.Next,
			})
			for _, r := range wt.Requires {
				t := task(r.Task)
				if r.Subproject != "" {
					t += " in " + r.Subproject
				}
				text := w.gate(r.To) + ": requires " + join(", ", join(" ", t, "at", w.gate(r.From)), r.Text) + ": " + condition(r) + "."
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
