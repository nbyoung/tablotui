package act

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/nbyoung/tabloio/action"

	"github.com/nbyoung/tablotui/internal/ui"
)

// The four fields of the record form, in order.
const (
	fieldGate = iota
	fieldState
	fieldReason
	fieldNote
	fields
)

// labels names each field, as the form and the status line show it.
var labels = [fields]string{"gate", "state", "reason", "note"}

// noReason is the reason choice that removes the reason.
const noReason = "none"

// form is the record form: the gate, the state, the reason and the note of
// one task, with the defaults the task's status gives them.
type form struct {
	id, title string
	project   action.Project
	task      action.Task
	old       action.Status
	file      bool // the task has a status file

	gates   []string // the gates whose junction applies, in order
	states  []string // the states to choose
	reasons []string // none, then the project's reasons
	gate    int      // an index of gates
	state   int      // an index of states
	reason  int      // an index of reasons
	note    []rune
	cursor  int // the code points before the cursor in note

	focus   int
	touched [fields]bool // the viewer changed the field
}

// newForm builds the form for a task, with the defaults its status and the
// gate under the cursor give.
func newForm(p action.Project, t action.Task, cursorGate string) *form {
	f := &form{id: t.ID, title: t.Title, project: p, task: t, reasons: []string{noReason}}
	f.reasons = append(f.reasons, p.Reasons...)
	if len(p.Gates) > 0 {
		f.old = action.Status{Gate: p.Gates[0], State: "undefined"}
	}
	if t.Status != nil {
		f.old, f.file = *t.Status, true
	}
	for i, g := range p.Gates {
		if i == 0 || f.applies(g) {
			f.gates = append(f.gates, g)
		}
	}
	for _, s := range p.States {
		if s != "undefined" && s != "complete" {
			f.states = append(f.states, s)
		}
	}
	f.gate = max(0, slices.Index(f.gates, f.old.Gate))
	if c := slices.Index(f.gates, cursorGate); c >= 0 && slices.Index(p.Gates, cursorGate) > slices.Index(p.Gates, f.old.Gate) {
		f.gate = c
	}
	f.defaults()
	return f
}

// applies reports whether the task's junction at a gate applies.
func (f *form) applies(gate string) bool {
	for _, j := range f.task.Junctions {
		if j.Gate == gate {
			return j.Kind != "not-applicable"
		}
	}
	return false
}

// atOld reports whether the form's gate is the status's.
func (f *form) atOld() bool { return f.gates[f.gate] == f.old.Gate }

// defaults gives each field the viewer has not changed its default for the
// form's gate.
func (f *form) defaults() {
	if !f.touched[fieldState] {
		f.state = 0
		if i := slices.Index(f.states, f.old.State); f.atOld() && i >= 0 {
			f.state = i
		}
	}
	if !f.touched[fieldReason] {
		f.reason = 0
		if i := slices.Index(f.reasons, f.old.Reason); f.atOld() && f.old.Reason != "" && i >= 0 {
			f.reason = i
		}
	}
	if !f.touched[fieldNote] {
		f.note = nil
		if f.atOld() {
			f.note = []rune(f.old.Note)
		}
		f.cursor = len(f.note)
	}
}

// What the gate decides of the state, in the package's order.
const (
	stateChoice    = iota // the viewer chooses the state
	stateUndefined        // the gate undefined fixes the state undefined
	stateComplete         // the last gate that applies fixes complete
	holdsGate             // the next junction is recursive: the file holds the gate alone
)

// decides returns what the form's gate decides.
func (f *form) decides() int {
	switch {
	case f.gate == 0:
		return stateUndefined
	case f.gate == len(f.gates)-1:
		return stateComplete
	}
	for _, j := range f.task.Junctions {
		if j.Gate == f.gates[f.gate+1] && j.Kind == "recursive" {
			return holdsGate
		}
	}
	return stateChoice
}

// takes reports whether a field takes input at the form's gate.
func (f *form) takes(field int) bool {
	d := f.decides()
	switch field {
	case fieldGate:
		return true
	case fieldState:
		return d == stateChoice && len(f.states) > 0
	}
	return d != holdsGate
}

// fixed returns the muted text of a field that takes no input.
func (f *form) fixed(field int) string {
	switch d := f.decides(); {
	case d == holdsGate:
		return "the subproject states it"
	case field != fieldState:
		return ""
	case d == stateUndefined:
		return "undefined, as the gate undefined requires"
	case d == stateComplete:
		return "complete, at the last gate that applies"
	}
	return "none to choose"
}

// move focuses the next or the previous field that takes input, round the
// ends.
func (f *form) move(step int) {
	for i := 1; i <= fields; i++ {
		if n := ((f.focus+step*i)%fields + fields) % fields; f.takes(n) {
			f.focus = n
			return
		}
	}
}

// choose moves the focused choice by step, round the ends.
func (f *form) choose(step int) {
	turn := func(i, n int) int { return ((i+step)%n + n) % n }
	switch f.focus {
	case fieldGate:
		f.gate = turn(f.gate, len(f.gates))
		f.defaults()
	case fieldState:
		f.state = turn(f.state, len(f.states))
		f.touched[fieldState] = true
	case fieldReason:
		f.reason = turn(f.reason, len(f.reasons))
		f.touched[fieldReason] = true
	}
}

// key handles one key in the form and reports whether it asks for the plan.
func (f *form) key(k tea.KeyPressMsg) bool {
	s := k.String()
	switch s {
	case "enter":
		return true
	case "down", "tab":
		f.move(1)
		return false
	case "up", "shift+tab":
		f.move(-1)
		return false
	}
	if f.focus != fieldNote {
		switch s {
		case "right":
			f.choose(1)
		case "left":
			f.choose(-1)
		}
		return false
	}
	switch s {
	case "left", "ctrl+b":
		f.cursor = max(0, f.cursor-1)
	case "right", "ctrl+f":
		f.cursor = min(len(f.note), f.cursor+1)
	case "home", "ctrl+a":
		f.cursor = 0
	case "end", "ctrl+e":
		f.cursor = len(f.note)
	case "backspace", "ctrl+h":
		if f.cursor > 0 {
			f.edit(f.cursor-1, f.cursor, "")
		}
	case "delete", "ctrl+d":
		if f.cursor < len(f.note) {
			f.edit(f.cursor, f.cursor+1, "")
		}
	case "ctrl+u":
		f.edit(0, f.cursor, "")
	case "ctrl+k":
		f.edit(f.cursor, len(f.note), "")
	default:
		if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt|tea.ModMeta|tea.ModSuper) == 0 {
			f.insert(k.Text)
		}
	}
	return false
}

// insert enters text at the cursor of the note. A control character or a
// line break enters as one space.
func (f *form) insert(text string) {
	if f.focus != fieldNote {
		return
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
	f.edit(f.cursor, f.cursor, text)
}

// edit replaces the code points from i to j of the note with text, and
// leaves the cursor after it.
func (f *form) edit(i, j int, text string) {
	ins := []rune(text)
	note := append(append(slices.Clone(f.note[:i]), ins...), f.note[j:]...)
	f.note, f.cursor = note, i+len(ins)
	f.touched[fieldNote] = true
}

// recording returns the request the form shows: every field explicitly.
func (f *form) recording() action.Recording {
	rec := action.Recording{ID: f.id, Gate: f.gates[f.gate]}
	switch f.decides() {
	case holdsGate:
		return rec
	case stateUndefined:
		rec.State = "undefined"
	case stateComplete:
		rec.State = "complete"
	default:
		if len(f.states) > 0 {
			rec.State = f.states[f.state]
		}
	}
	if r := f.reasons[f.reason]; r == noReason {
		rec.NoReason = true
	} else {
		rec.Reason = r
	}
	if n := strings.TrimSpace(string(f.note)); n == "" {
		rec.NoNote = true
	} else {
		rec.Note = n
	}
	return rec
}

// now returns the line that states the status as it stands.
func (f *form) now() string {
	s := "  now     " + f.old.Gate
	if f.old.State != "" {
		s += " " + f.old.State
	}
	if f.old.Reason != "" {
		s += ", " + f.old.Reason
	}
	switch {
	case !f.file:
		s += ", no status file"
	case f.old.Note != "":
		s += " · " + f.old.Note
	}
	return s
}

// line returns the line of one field, w cells wide at most.
func (f *form) line(field, w int, c ui.Context) string {
	head := "  "
	if field == f.focus {
		head = "> "
	}
	head += fmt.Sprintf("%-8s", labels[field])
	if !f.takes(field) {
		return head + c.Styles.Muted.Render(c.Measure.Fit(f.fixed(field), max(0, w-10)))
	}
	switch field {
	case fieldGate:
		return head + "< " + f.gates[f.gate] + " >"
	case fieldState:
		return head + "< " + f.states[f.state] + " >"
	case fieldReason:
		return head + "< " + f.reasons[f.reason] + " >"
	}
	if field != f.focus {
		return head + string(f.note)
	}
	return head + f.noteView(w-10, c)
}

// noteView draws the focused note in w cells: the text from the first code
// point, or from "…" and the first later one, from which the text up to the
// cursor and the cursor cell fit; the cursor as one reversed cell; the rest,
// cut with "…".
func (f *form) noteView(w int, c ui.Context) string {
	m := c.Measure
	at := " "
	if f.cursor < len(f.note) {
		at = string(f.note[f.cursor])
	}
	atW := m.Width(at)
	start := 0
	lead := ""
	for start < f.cursor && m.Width(lead+string(f.note[start:f.cursor]))+atW > w {
		start++
		lead = "…"
	}
	before := lead + string(f.note[start:f.cursor])
	rest := max(0, w-m.Width(before)-atW)
	var after string
	if f.cursor < len(f.note) {
		after = m.Clean(string(f.note[f.cursor+1:]))
		if m.Method.StringWidth(after) > rest {
			after = m.Method.Truncate(after, rest, "…")
		}
	}
	return before + c.Styles.Cursor.Render(at) + after
}

// status returns the part of the status line the focused field states.
func (f *form) status() string {
	switch f.focus {
	case fieldGate:
		return fmt.Sprintf("gate · %d of %d", f.gate+1, len(f.gates))
	case fieldState:
		return fmt.Sprintf("state · %d of %d", f.state+1, len(f.states))
	case fieldReason:
		return fmt.Sprintf("reason · %d of %d", f.reason+1, len(f.reasons))
	}
	return fmt.Sprintf("note · cursor %d of %d", f.cursor, len(f.note))
}
