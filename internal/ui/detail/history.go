package detail

import (
	"fmt"
	"strings"

	"github.com/nbyoung/tablotui/internal/view/list"
)

// sentenceNoEvent is what a history with no line reads.
const sentenceNoEvent = "No event in view."

// historyEffects words the keys of a line's effect, as tabloio's
// historyEffects does (8ed1 decision 4). A key outside the table reads as it
// is.
var historyEffects = map[string]string{
	"authorised_commit":  "an authority commits the task file on the trunk, so the task is authorised",
	"authorised_merge":   "an authority merges the change, so the task is authorised",
	"authorised_trailer": "an authority's trailer authorises the task",
	"proposed":           "no authority accepts the change, so the task stands proposed",
	"handoff":            "the contributor hands the work to the reviewer",
	"accepts":            "the reviewer accepts the work at the gate",
	"stands":             "none; the authorisation stands as the review",
	"none":               "none",
}

// model reads a line's Model: trailer by its check's reading, as tabloio
// does. With no check it is the model alone.
func (w words) model(l list.Line) string {
	c := l.Check
	if c == nil {
		return l.Model
	}
	var stated string
	switch {
	case c.Stated == "":
	case c.Gate == "":
		stated = "; the junction states " + c.Stated
	default:
		stated = "; the " + w.gate(c.Gate) + " junction states " + c.Stated
	}
	switch c.Reading {
	case "agrees":
		if l.Model != "" {
			return l.Model + stated
		}
	case "differs":
		if l.Model != "" {
			return l.Model + stated + ", which differs"
		}
	case "missing":
		return "no trailer" + stated
	case "exempt":
		return "none recorded; exempt"
	case "unstated":
		if l.Model != "" {
			return l.Model + "; the junction states no model"
		}
	}
	return l.Model
}

// kindsText reads the kinds of a line's events, with the mark of a proposal
// after them.
func kindsText(l list.Line) string {
	var kinds []string
	for _, e := range l.Events {
		kinds = append(kinds, e.Kind)
	}
	s := strings.Join(kinds, ", ")
	if l.Proposal {
		s += " (proposal)"
	}
	return s
}

// recordedOf is the first status a line's events record, or nil. A
// reaffirmed event records none.
func recordedOf(l list.Line) *list.EventStatus {
	for _, e := range l.Events {
		if e.Kind == "status" && e.Recorded != nil {
			return e.Recorded
		}
	}
	return nil
}

// last is the cell that says what a line records: the first of the status
// phrase, the gate of a review, the move of a pin, and the appearance of a
// task file.
func (w words) last(l list.Line) string {
	if r := recordedOf(l); r != nil {
		return w.phrase(list.Status{Gate: r.Gate, State: r.State, Reason: r.Reason})
	}
	for _, e := range l.Events {
		if e.Kind == "reviewed" && e.Gate != "" {
			return w.gate(e.Gate)
		}
	}
	for _, e := range l.Events {
		if e.Kind == "pin" && e.Pin != nil {
			return pin(e.Pin)
		}
	}
	for _, e := range l.Events {
		if e.Kind == "task" {
			return "the task file appears"
		}
	}
	return ""
}

// lineGate is the gate a line names: of its status or of its review.
func lineGate(l list.Line) string {
	if r := recordedOf(l); r != nil {
		return r.Gate
	}
	for _, e := range l.Events {
		if e.Kind == "reviewed" {
			return e.Gate
		}
	}
	return ""
}

// lineTask is the task cell of a line: the task where it is not the subject,
// and for a line taken from a subproject the subproject and its task, by id.
func lineTask(l list.Line, subject string) string {
	t := ""
	if l.Task.ID != subject {
		t = task(l.Task)
	}
	if s := l.Taken(); s != nil {
		t = join(", ", t, "taken from "+s.URL+" "+s.Task)
	}
	return t
}

// historySubject is the title's subject: the subject tablo always states,
// the request's task, or the root where the data states neither.
func historySubject(d list.History) string {
	switch {
	case d.Subject.ID != "":
		return d.Subject.ID
	case d.Params.Task != "":
		return d.Params.Task
	}
	return d.Project.Root
}

// nest finds, for each line taken from a subproject, the project's own line
// whose pin of that subproject follows it next, as tabloio's historyNest
// does: the pane draws the taken line beneath that pin (8ed1 decision 2). A
// taken line with no such pin after it stands in its own place.
func nest(lines []list.Line) map[int][]int {
	under := map[int][]int{}
	for i, l := range lines {
		s := l.Taken()
		if s == nil {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if lines[j].Taken() != nil {
				continue
			}
			pinned := false
			for _, e := range lines[j].Events {
				pinned = pinned || e.Kind == "pin" && e.Pin != nil && e.Pin.URL == s.URL
			}
			if pinned {
				under[j] = append(under[j], i)
				break
			}
		}
	}
	return under
}

// historyLines builds the history at a level. The glance lists the project's
// own lines alone, as tablo gives them there; from detail on a line taken
// from a subproject stands beneath the pin it follows, with the events and
// the commits of those lines counted here.
func historyLines(d list.History, lvl list.Level) (subject, counts string, lines []Line) {
	w := words{d.Legend}
	subject = historySubject(d)
	counts = count(d.Events, "event", "events")
	if len(d.Lines) == 0 {
		return subject, counts, []Line{{Text: sentenceNoEvent, Item: -1}}
	}
	under := nest(d.Lines)
	nested := map[int]bool{}
	for _, ks := range under {
		for _, k := range ks {
			nested[k] = true
		}
	}
	for i, l := range d.Lines {
		if lvl < list.Detail && l.Taken() != nil || lvl >= list.Detail && nested[i] {
			continue
		}
		gate := lineGate(l)
		e := l.First()
		lines = append(lines, Line{
			Text: cells(e.Date, e.By, kindsText(l), lineTask(l, d.Subject.ID), w.last(l)),
			Hang: 2,
			Head: lvl >= list.Detail,
			Item: i,
			Task: l.Task.ID,
			Gate: gate,
		})
		if lvl < list.Detail {
			continue
		}
		add := func(text string) {
			lines = append(lines, Line{Text: text, Indent: 2, Hang: 2, Item: i, Task: l.Task.ID, Gate: gate})
		}
		if l.After != nil {
			s := "Status after: " + w.status(*l.After)
			if l.Unchanged {
				s += ", unchanged"
			}
			add(s)
		}
		if r := recordedOf(l); r != nil && r.Note != "" {
			add("Note: " + r.Note)
		}
		if l.Effect != "" {
			add("Effect: " + phraseOf(historyEffects, l.Effect))
		}
		if l.SubjectAfter != nil {
			add(task(d.Subject) + " after: " + w.status(*l.SubjectAfter))
		}
		if m := w.model(l); m != "" {
			add("Model: " + m)
		}
		if l.Committer != "" {
			add("Committer: " + l.Committer)
		}
		if ks := under[i]; len(ks) > 0 {
			events, commits := 0, map[string]bool{}
			for _, k := range ks {
				events += len(d.Lines[k].Events)
				for _, e := range d.Lines[k].Events {
					if e.Commit != "" {
						commits[e.Commit] = true
					}
				}
			}
			head := "Subproject events: " + count(events, "event", "events")
			if len(commits) > 0 {
				head += " in " + count(len(commits), "commit", "commits")
			}
			add(head + " of " + d.Lines[ks[0]].Taken().URL)
			for _, k := range ks {
				sl := d.Lines[k]
				se := sl.First()
				lines = append(lines, Line{
					Text:   cells(se.Date, se.By, kindsText(sl), se.Subproject.Task, w.last(sl), sl.Model),
					Indent: 4, Hang: 2, Item: i, Task: l.Task.ID, Gate: gate,
				})
			}
		}
		if lvl < list.Provenance {
			continue
		}
		if c := l.Commit; c != nil {
			add("Commit: " + c.Hash)
			add("Subject: " + c.Subject)
			add("Author: " + actor(c.Author, c.AuthorTime))
			add("Committer: " + actor(c.Committer, ""))
			if len(c.Trailers) > 0 {
				add("Trailers: " + strings.Join(c.Trailers, ", "))
			}
			var files []string
			for _, f := range c.Files {
				files = append(files, fmt.Sprintf("%s (%s)", f.Path, f.Change))
			}
			if len(files) > 0 {
				add("Files: " + strings.Join(files, ", "))
			}
		}
	}
	if lvl >= list.Provenance && len(d.Commands) > 0 {
		lines = append(lines, Line{Item: -1}, Line{Text: "Commands", Head: true, Item: len(d.Lines)})
		lines = append(lines, commandLines(d.Commands, 2, 2, len(d.Lines), "", "")...)
	}
	return subject, counts, lines
}

// actor reads an author or a committer with the time of the commit where the
// data holds it: tablo gives the author's time alone.
func actor(p list.Person, at string) string {
	who := p.Email
	if p.Name != "" {
		who = p.Name + " <" + p.Email + ">"
	}
	return join(", ", who, at)
}
