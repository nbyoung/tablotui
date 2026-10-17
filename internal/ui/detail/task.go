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
	switch {
	case d.Parent == nil:
		field(0, "parent", "none", "", "")
	case d.Parent.Order == nil:
		field(0, "parent", task(d.Parent.TaskRef), d.Parent.ID, "")
	default:
		field(0, "parent", fmt.Sprintf("%s, order %d", task(d.Parent.TaskRef), *d.Parent.Order), d.Parent.ID, "")
	}
	field(0, "status", w.status(d.Status), "", "")
	field(0, "note", d.Status.Note, "", "")
	field(0, "recorded", recorded(d.Status), "", "")
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
	if p := d.Parent; p != nil {
		order := fmt.Sprintf("no order, among %d under %s", d.Siblings, p.ID)
		if p.Order != nil {
			order = fmt.Sprintf("%d of %d under %s", *p.Order, d.Siblings, p.ID)
		}
		field(2, "order", order, p.ID, "")
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
	if a := d.Authorisation; lvl >= list.Provenance && a != nil && a.Commit != nil {
		section(2, "Authorisation commit")
		lines = append(lines, commitLines(a.Commit, 2, "")...)
		for _, f := range [][2]string{{"accepts by", plain(acceptsBy[a.Way])}, {"the authority is", authorityIs[a.By]}} {
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
		if st := s.Status; st != nil {
			field(6, "status", w.status(*st), "", "")
			field(6, "note", st.Note, "", "")
		}
		for _, c := range s.Children {
			add(Line{Text: cells(task(c.TaskRef), w.status(c.Status)), Indent: 2, Hang: 2, Item: 6})
		}
	}
	if lvl < list.Provenance {
		return id, "", lines
	}

	if len(d.Considered) > 0 {
		section(7, "Roll-up")
		add(Line{Text: "Children considered: " + strings.Join(d.Considered, " "), Indent: 2, Hang: 2, Item: 7})
		for _, st := range d.Chain {
			name := task(st.Task)
			if st.URL != "" {
				name += " in " + st.URL
			}
			add(Line{Text: name + ": " + join("; ", w.status(st.Status), recorded(st.Status)), Indent: 2, Hang: 2, Item: 7, Task: st.Task.ID})
		}
	}
	if len(d.Linkages) > 0 {
		section(8, "Linkages")
		for _, l := range d.Linkages {
			var gate string
			if len(l.Gates) > 0 {
				gate = w.gate(l.Gates[0])
				if n := len(l.Gates) - 1; n > 0 {
					gate += " and " + count(n, "more", "more")
				}
				add(Line{Text: gate, Indent: 2, Item: 8, Task: id, Gate: l.Gates[0]})
			}
			for _, f := range linkageFacts(l) {
				add(Line{Text: f[0] + ": " + f[1], Indent: 4, Hang: 2, Item: 8})
			}
		}
	}
	section(9, "Where each junction field comes from")
	for _, s := range d.Sources {
		by := s.By
		if w, ok := statedBy[by]; ok {
			by = w
		}
		add(Line{Text: cells(w.gate(s.Gate), s.Field, s.Value, by), Indent: 2, Hang: 2, Item: 9, Task: id, Gate: s.Gate})
	}
	section(10, counted("Reviews", len(d.Reviews)))
	for _, r := range d.Reviews {
		var commit string
		if r.Commit != nil {
			commit = join(" ", short(r.Commit.Hash), r.Commit.Subject)
		}
		add(Line{Text: cells(w.gate(r.Gate), r.Reviewer, commit, phraseOf(reviewEffects, r.Effect)), Indent: 2, Hang: 2, Item: 10, Task: id, Gate: r.Gate})
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
		add(Line{Text: cells(short(m.Commit), w.gate(m.Gate), stated, trailer, phraseOf(modelReadings, m.Reading)), Indent: 2, Hang: 2, Item: 11, Task: id, Gate: m.Gate})
	}
	section(12, "Newest events")
	add(Line{
		Text:   fmt.Sprintf("%s; the newest %d, newest first.", count(d.EventCount, "event", "events"), len(d.Events)),
		Indent: 2, Item: 12,
	})
	for _, e := range d.Events {
		add(Line{
			Text:   cells(e.Date, short(e.Commit), e.By, e.Kind, w.eventRecords(e)),
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

// acceptsBy words how a deciding commit accepts, as tabloio does.
var acceptsBy = map[string]string{
	"change":  "a change on the trunk",
	"merge":   "a merge",
	"trailer": "an `Authorised:` trailer",
}

// authorityIs words which of a commit's actors is an authority, as tabloio
// does.
var authorityIs = map[string]string{
	"author":    "the author",
	"committer": "the committer",
	"both":      "the author and the committer",
}

// reviewEffects words a review's effect, as tabloio does.
var reviewEffects = map[string]string{
	"accepts":       "The reviewer accepts",
	"authorisation": "The authorisation stands as the review",
	"none":          "None",
}

// modelReadings words a model check's reading, as tabloio does.
var modelReadings = map[string]string{
	"agrees":   "Agrees",
	"differs":  "Differs",
	"missing":  "No trailer",
	"exempt":   "Exempt",
	"unstated": "No model stated",
}

// phraseOf returns the phrase of a key, or the key where the table has none.
func phraseOf(table map[string]string, key string) string {
	if p, ok := table[key]; ok {
		return p
	}
	return key
}

// eventRecords reads what an event records: a status phrase with its note
// after a colon, a gate for a review, or "<url> <old> → <new>" for a pin;
// then, for an event taken from a subproject, the subproject and its task.
func (w words) eventRecords(e list.Event) string {
	var x string
	switch {
	case e.Kind == "status" && e.Recorded != nil:
		r := e.Recorded
		x = join(": ", w.phrase(list.Status{Gate: r.Gate, State: r.State, Reason: r.Reason}), r.Note)
	case e.Kind == "reviewed" && e.Gate != "":
		x = w.gate(e.Gate)
	case e.Kind == "pin" && e.Pin != nil:
		x = join(" ", e.Pin.URL, short(e.Pin.Old), "→", short(e.Pin.New))
	}
	if s := e.Subproject; s != nil {
		x = join("; ", x, "in the subproject "+s.URL+", task "+subTask(*s))
	}
	return x
}

// The words of a linkage's keys, as tabloio has them: one phrase per form,
// reason, trunk reading and pin reading.
var (
	linkForms = map[string]string{
		"submodule": "A path that is a Git submodule, so the submodule's pin fixes the commit",
		"directory": "A directory of the same repository, read at the commit in view",
		"url":       "A repository URL, so the entry's `commit` field fixes the commit",
		"none":      "No link",
	}
	linkWhy = map[string]string{
		"no_link": "Not read: no link reaches the subproject",
		"no_task": "Not read: the subproject has no such task",
	}
	linkOnTrunk = map[string]string{
		"yes":     "the pin is on the trunk",
		"no":      "the pin is off the trunk",
		"unknown": "whether the pin is on the trunk is unknown",
	}
	linkBehind = map[string]string{
		"yes": "behind its tip",
		"no":  "at its tip",
	}
)

// linkageFacts reads a linkage as name and value pairs, in tabloio's order,
// each where the linkage holds it.
func linkageFacts(l list.Linkage) [][2]string {
	var out [][2]string
	add := func(name, value string) {
		if value != "" {
			out = append(out, [2]string{name, plain(value)})
		}
	}
	url := l.URL
	if l.File != "" {
		url += ", stated in " + l.File
		if l.Line > 0 {
			url += fmt.Sprintf(", line %d", l.Line)
		}
	}
	add("url", url)
	add("Form of the url", linkForms[l.Form])
	if l.Commit != "" {
		add("Pin", short(l.Commit)+", in full "+l.Commit)
	}
	if l.Form == "submodule" || l.Form == "url" {
		field := "Not stated"
		if l.StatedCommit != "" {
			field = l.StatedCommit
		} else if l.Form == "submodule" {
			field = "Not stated; nothing restates the pin in the task file"
		}
		add("commit field", field)
	}
	if l.Moved {
		add("Checkout", "Stands elsewhere than the recorded pin "+short(l.Recorded))
	}
	if l.Pinned != nil {
		add("The pinned commit", commitLine(*l.Pinned))
	}
	id := l.ID
	if l.ID == "" && l.Task.ID != "" {
		id = "Not stated; the junction reads the subproject's root, " + l.Task.ID
	}
	add("id", id)
	if l.Why != "" {
		add("Task read", linkWhy[l.Why])
	}
	if l.Trunk != "" {
		trunk := l.Trunk
		if l.Tip != "" {
			trunk += ", tip " + short(l.Tip)
		}
		if on := linkOnTrunk[l.OnTrunk]; on != "" {
			trunk += "; " + on
			if b := linkBehind[l.Behind]; b != "" {
				trunk += ", " + b
			}
		}
		add("The subproject's trunk", trunk)
	}
	if l.SetBy != nil {
		add("Pin set by", commitLine(*l.SetBy))
	}
	return out
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
