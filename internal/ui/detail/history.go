package detail

import (
	"fmt"
	"slices"
	"strings"

	"github.com/nbyoung/tablotui/internal/view/list"
)

// sentenceNoEvent is what a history with no line reads.
const sentenceNoEvent = "No event in view."

// effects are the words of the effect keys, as tabloio's design 9167 words them.
var effects = map[string]string{
	"authorised-commit":  "an authority commits the task file on the trunk, so the task is authorised",
	"authorised-merge":   "an authority merges the change, so the task is authorised",
	"authorised-trailer": "an authority's trailer authorises the task",
	"proposed":           "no authority accepts the change, so the task stands proposed",
	"handoff":            "the contributor hands the work to the reviewer",
	"accepts":            "the reviewer accepts the work at the gate",
	"stands":             "none; the authorisation stands as the review",
	"none":               "none",
}

// model reads a line's model by how it compares with the junction's.
func (w words) model(l list.Line) string {
	stated := "the " + w.gate(l.StatedGate) + " junction states " + l.Stated
	switch l.Reading {
	case "agrees":
		return l.Model + "; " + stated
	case "differs":
		return l.Model + "; " + stated + ", which differs"
	case "missing":
		return "no trailer; " + stated
	case "exempt":
		return "none recorded; exempt"
	}
	return l.Model
}

// kindsText reads the kinds of a line, with the mark of a proposal after them.
func kindsText(l list.Line) string {
	s := strings.Join(l.Kinds, ", ")
	if l.Proposal {
		s += " (proposal)"
	}
	return s
}

// last is the cell that says what a line records: the first of the status,
// the gate of a review, the pin, and the appearance of a task file.
func (w words) last(l list.Line) string {
	switch {
	case l.Status != nil:
		return w.status(*l.Status)
	case l.Gate != "":
		return w.gate(l.Gate)
	case l.Pin != nil:
		return pin(l.Pin)
	case slices.Contains(l.Kinds, "task"):
		return "the task file appears"
	}
	return ""
}

// lineGate is the gate a line names: of its status or of its review.
func lineGate(l list.Line) string {
	if l.Gate == "" && l.Status != nil {
		return l.Status.Gate
	}
	return l.Gate
}

// historySubject is the title's subject.
func historySubject(d list.History) string {
	switch {
	case d.Subject != nil:
		return d.Subject.ID
	case d.Params.Task != "":
		return d.Params.Task
	}
	return d.Project.ID
}

// historyLines builds the history at a level.
func historyLines(d list.History, lvl list.Level) (subject, counts string, lines []Line) {
	w := words{d.Legend}
	subject = historySubject(d)
	counts = count(d.Events, "event", "events")
	if len(d.Lines) == 0 {
		return subject, counts, []Line{{Text: sentenceNoEvent, Item: -1}}
	}
	for i, l := range d.Lines {
		gate := lineGate(l)
		cell := ""
		if d.Subject == nil || l.Task.ID != d.Subject.ID {
			cell = task(l.Task)
		}
		lines = append(lines, Line{
			Text: cells(l.Date, l.By, kindsText(l), cell, w.last(l)),
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
		if l.Status != nil && l.Status.Note != "" {
			add("Note: " + l.Status.Note)
		}
		if l.Effect != "" {
			e, ok := effects[l.Effect]
			if !ok {
				e = l.Effect
			}
			add("Effect: " + e)
		}
		if l.SubjectAfter != nil && d.Subject != nil {
			add(task(*d.Subject) + " after: " + w.status(*l.SubjectAfter))
		}
		if m := w.model(l); m != "" {
			add("Model: " + m)
		}
		if l.Committer != "" {
			add("Committer: " + l.Committer)
		}
		if s := l.Sub; s != nil {
			add(fmt.Sprintf("Subproject events: %s in %s of %s",
				count(s.Events, "event", "events"), count(s.Commits, "commit", "commits"), s.URL))
			for _, sl := range s.Lines {
				lines = append(lines, Line{
					Text:   cells(sl.Date, sl.By, kindsText(sl), task(sl.Task), w.last(sl), sl.Model),
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
			add("Committer: " + actor(c.Committer, c.CommitterTime))
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
		lines = append(lines, commandLines(l.Commands, 4, 2, i, l.Task.ID, gate)...)
	}
	if lvl >= list.Provenance && len(d.Commands) > 0 {
		lines = append(lines, Line{Item: -1}, Line{Text: "Commands", Head: true, Item: len(d.Lines)})
		lines = append(lines, commandLines(d.Commands, 2, 2, len(d.Lines), "", "")...)
	}
	return subject, counts, lines
}

// actor reads an author or a committer with the time of the commit.
func actor(p list.Person, at string) string {
	who := p.Email
	if p.Name != "" {
		who = p.Name + " <" + p.Email + ">"
	}
	return join(", ", who, at)
}
