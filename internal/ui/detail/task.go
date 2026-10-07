package detail

import (
	"fmt"
	"strings"

	"github.com/nbyoung/tablotui/internal/view/list"
)

// fieldHang is the width of a label in the task definition.
const fieldHang = 13

// commitHang is the width of a label in a commit's fields.
const commitHang = 18

// person reads an actor: the name and the email, or the email alone.
func person(p list.Person) string { return join(", ", p.Name, p.Email) }

// commitLines draws the fields of a commit at the indent of a section.
func commitLines(c *list.Commit, item int, task string) []Line {
	if c == nil {
		return nil
	}
	var out []Line
	add := func(label, text string) {
		if text != "" {
			out = append(out, Line{Label: label, Text: text, Indent: 2, Hang: commitHang, Item: item, Task: task})
		}
	}
	add("commit", join(" ", short(c.Hash), c.Subject))
	add("date", c.Date)
	add("author", person(c.Author))
	add("committer", person(c.Committer))
	add("trailers", strings.Join(c.Trailers, ", "))
	return out
}

// commandLines draws commands as text and comment, at the indent and hang given.
func commandLines(cs []list.Command, indent, hang, item int, task, gate string) []Line {
	var out []Line
	for _, c := range cs {
		out = append(out, Line{Text: join(" # ", c.Text, c.Comment), Indent: indent, Hang: hang, Item: item, Task: task, Gate: gate})
	}
	return out
}

// requirementEntry reads one requirement of a task as an entry that opens with
// the other task. A cross-project entry names its project and commit.
func (w words) requirementEntry(r list.Requirement, item int, withStatus bool, gate string) entry {
	e := entry{task: r.Task, item: item, gate: gate}
	if r.Subproject != "" {
		e.text = task(r.Task) + " in " + r.Subproject + " at " + short(r.Commit)
	}
	var status string
	if withStatus && r.Status != nil {
		status = w.status(*r.Status)
	}
	e.rest = cells(w.edge(r), r.Text, status, condition(r))
	return e
}

// standing reads where a junction stands.
func standing(j list.Junction) string {
	s := map[string]string{
		"passed": "passed", "here": "passed", "next": "next", "later": "later", "exempt": "does not apply",
	}[j.Stands]
	if s == "" {
		s = j.Stands
	}
	if j.Reviewed {
		s += ", reviewed"
	}
	if j.Stands == "here" {
		s += "; the status stands here"
	}
	return s
}

// taskLines builds the task definition at a level.
func taskLines(d list.Task, lvl list.Level, unfold bool) (subject, counts string, lines []Line) {
	w := words{d.Legend}
	id := d.Task.ID
	out := &lines
	add := func(l Line) { *out = append(*out, l) }
	section := func(item int, heading string) {
		add(Line{Item: -1})
		add(Line{Text: heading, Head: true, Item: item})
	}
	field := func(item int, label, text, target, gate string) {
		if text != "" {
			add(Line{Label: label, Text: text, Hang: fieldHang, Item: item, Task: target, Gate: gate})
		}
	}

	add(Line{Text: task(d.Task), Head: true, Item: 0})
	field(0, "assignee", d.Assignee, "", "")
	if d.Parent == nil {
		field(0, "parent", "none", "", "")
	} else {
		field(0, "parent", fmt.Sprintf("%s, order %d", task(d.Parent.TaskRef), d.Parent.Order), d.Parent.ID, "")
	}
	field(0, "status", w.status(d.Status), "", "")
	field(0, "note", d.Status.Note, "", "")
	if d.Status.Date != "" {
		recorded := join(" by ", d.Status.Date, d.Status.Recorder)
		switch {
		case d.Status.From != nil && d.Status.From.Kind == "rollup":
			recorded = d.Status.Date + ", the oldest among its children"
		case d.Status.From != nil && d.Status.From.Kind == "snapshot":
			recorded = d.Status.Date + ", from the subproject"
		}
		field(0, "recorded", recorded, "", "")
	}
	if lvl >= list.Provenance && d.StatusCommit != nil {
		section(0, "Status commit")
		lines = append(lines, commitLines(d.StatusCommit, 0, "")...)
	}
	if lvl < list.Detail {
		return id, "", lines
	}

	section(1, "Description")
	add(Line{Text: d.Description, Item: 1})
	if len(d.References) > 0 {
		add(Line{Text: "References:", Item: 1})
		for _, r := range d.References {
			add(Line{Text: join(", ", r.Text, r.URL), Indent: 2, Hang: 2, Item: 1})
		}
	}

	section(2, "Place in the tree")
	var path []string
	for _, t := range d.Path {
		path = append(path, task(t))
	}
	field(2, "path", strings.Join(path, " › "), "", "")
	if d.Parent != nil {
		field(2, "order", fmt.Sprintf("%d of %d under %s", d.Parent.Order, d.Siblings, d.Parent.ID), d.Parent.ID, "")
	}
	if len(d.Children) == 0 {
		field(2, "children", "none", "", "")
	} else {
		field(2, "children", fmt.Sprint(len(d.Children)), "", "")
		for _, c := range d.Children {
			add(Line{Text: cells(task(c.TaskRef), w.status(c.Status)), Indent: 2, Hang: 2, Item: 2, Task: c.ID, Gate: c.Status.Gate})
		}
	}
	field(2, "authorities", authorities(d.Authorities), "", "")
	if d.Authorised {
		field(2, "authorised", "yes", "", "")
	} else {
		field(2, "authorised", "no, proposed", "", "")
	}
	if a := d.Authorisation; lvl >= list.Provenance && a != nil {
		section(2, "Authorisation commit")
		lines = append(lines, commitLines(&a.Commit, 2, "")...)
		way := map[string]string{"change": "a change on the trunk", "merge": "a merge", "trailer": "an Authorised: trailer"}[a.Way]
		by := map[string]string{"author": "the author", "committer": "the committer", "both": "the author and the committer"}[a.By]
		for _, f := range [][2]string{{"accepts by", way}, {"the authority is", by}} {
			if f[1] != "" {
				add(Line{Label: f[0], Text: f[1], Indent: 2, Hang: commitHang, Item: 2})
			}
		}
	}

	section(3, counted("Requires", len(d.Requires)))
	var es []entry
	for _, r := range d.Requires {
		es = append(es, w.requirementEntry(r, 3, true, r.From))
	}
	for _, e := range foldRuns(es, unfold) {
		add(e.line(2, 2))
	}
	section(4, counted("Dependents", len(d.Dependents)))
	es = es[:0]
	for _, r := range d.Dependents {
		es = append(es, w.requirementEntry(r, 4, false, r.To))
	}
	for _, e := range foldRuns(es, unfold) {
		add(e.line(2, 2))
	}

	section(5, "Junctions")
	for _, j := range d.Junctions {
		var reviewer, sub string
		switch {
		case j.Reviewer != "" && j.ByDefault:
			reviewer = "reviewer the assignee, " + j.Reviewer
		case j.Reviewer != "":
			reviewer = "reviewer " + j.Reviewer
		}
		if j.Subproject != nil {
			sub = join(" ", j.Subproject.URL, task(j.Subproject.Task))
		}
		add(Line{
			Text:   cells(w.gate(j.Gate), w.marks(j.Marks), j.Contributor, j.Model, reviewer, sub, standing(j)),
			Indent: 2, Hang: 2, Item: 5, Task: id, Gate: j.Gate,
		})
	}

	if s := d.Snapshot; s != nil {
		section(6, "Subproject snapshot")
		field(6, "task", join(" ", s.URL, task(s.Task)), "", "")
		field(6, "assignee", s.Assignee, "", "")
		field(6, "status", w.status(s.Status), "", "")
		field(6, "note", s.Status.Note, "", "")
		for _, c := range s.Children {
			add(Line{Text: cells(task(c.TaskRef), w.status(c.Status)), Indent: 2, Hang: 2, Item: 6})
		}
	}
	if lvl < list.Provenance {
		return id, "", lines
	}

	if len(d.Rollup) > 0 {
		section(7, "Roll-up")
		for _, f := range d.Rollup {
			add(Line{Text: f.Name + ": " + f.Value, Indent: 2, Hang: 2, Item: 7})
		}
	}
	if len(d.Linkages) > 0 {
		section(8, "Linkages")
		for _, l := range d.Linkages {
			add(Line{Text: w.gate(l.Gate), Indent: 2, Item: 8, Task: id, Gate: l.Gate})
			for _, f := range l.Facts {
				add(Line{Text: f.Name + ": " + f.Value, Indent: 4, Hang: 2, Item: 8})
			}
		}
	}
	section(9, "Where each junction field comes from")
	for _, s := range d.Sources {
		by := s.By
		if by == "default" {
			by = "the plain default"
		}
		add(Line{Text: cells(w.gate(s.Gate), s.Field, s.Value, by), Indent: 2, Hang: 2, Item: 9, Task: id, Gate: s.Gate})
	}
	section(10, counted("Reviews", len(d.Reviews)))
	for _, r := range d.Reviews {
		var commit string
		if r.Commit != nil {
			commit = join(" ", short(r.Commit.Hash), r.Commit.Subject)
		}
		effect := map[string]string{
			"accepts": "the reviewer accepts", "authorisation": "the authorisation stands as the review", "none": "none",
		}[r.Effect]
		add(Line{Text: cells(w.gate(r.Gate), r.Reviewer, commit, effect), Indent: 2, Hang: 2, Item: 10, Task: id, Gate: r.Gate})
	}
	section(11, counted("Models", len(d.Models)))
	for _, m := range d.Models {
		var stated, trailer string
		if m.Stated != "" {
			stated = "states " + m.Stated
		}
		if m.Trailer != "" {
			trailer = "trailer " + m.Trailer
		}
		reading := map[string]string{"agrees": "agrees", "differs": "differs", "missing": "no trailer", "exempt": "exempt"}[m.Reading]
		add(Line{Text: cells(short(m.Commit), w.gate(m.Gate), stated, trailer, reading), Indent: 2, Hang: 2, Item: 11, Task: id, Gate: m.Gate})
	}
	section(12, "Newest events")
	add(Line{
		Text:   fmt.Sprintf("%s; the newest %d, newest first.", count(d.EventCount, "event", "events"), len(d.Events)),
		Indent: 2, Item: 12,
	})
	for _, e := range d.Events {
		var what string
		switch {
		case e.Status != nil:
			what = join(": ", w.status(*e.Status), e.Status.Note)
		case e.Gate != "":
			what = w.gate(e.Gate)
		case e.Pin != nil:
			what = pin(e.Pin)
		}
		add(Line{
			Text:   cells(e.Date, short(e.Commit), e.By, strings.Join(e.Kinds, ", "), what),
			Indent: 2, Hang: 2, Item: 12,
		})
	}
	if more := d.EventCount - len(d.Events); more > 0 {
		add(Line{Text: fmt.Sprintf("… and %d earlier", more), Indent: 2, Item: 12})
	}
	if len(d.Commands) > 0 {
		section(13, "Commands")
		lines = append(lines, commandLines(d.Commands, 2, 2, 13, "", "")...)
	}
	return id, "", lines
}

// authorities reads the assignees whose authority covers the task: the
// consecutive ancestors of one email together, nearest first.
func authorities(as []list.Authority) string {
	var groups []string
	for i := 0; i < len(as); {
		j := i
		var ids []string
		for j < len(as) && as[j].Email == as[i].Email {
			ids = append(ids, as[j].By)
			j++
		}
		groups = append(groups, fmt.Sprintf("%s (%s)", as[i].Email, strings.Join(ids, ", ")))
		i = j
	}
	if len(groups) == 0 {
		return "none"
	}
	return strings.Join(groups, "; then ")
}
