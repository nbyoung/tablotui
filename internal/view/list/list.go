// Package list holds the data of the four list-shaped views the detail panes
// draw: the task definition, the work queue, the work-blockage tree and the
// history. The types mirror tablo's envelope of each view (tablo's Envelope
// and its view package), tag for tag where they carry the fact, as tabloio's
// renderers read it after its reconcile (tabloio designs e3ed, a3cc, 9167):
// an embedded detail or provenance of tablo's flattens into the view's own
// struct. Decoding ignores unknown fields and leaves absent ones zero.
package list

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Level orders the three levels of disclosure; each includes the one before.
type Level int

// The levels, from the least to the most.
const (
	Glance Level = iota
	Detail
	Provenance
)

var levelNames = [...]string{"glance", "detail", "provenance"}

// String returns the level's word.
func (l Level) String() string {
	if l < 0 || int(l) >= len(levelNames) {
		return fmt.Sprintf("level(%d)", int(l))
	}
	return levelNames[l]
}

// UnmarshalText reads "glance", "detail" or "provenance".
func (l *Level) UnmarshalText(b []byte) error {
	for i, n := range levelNames {
		if string(b) == n {
			*l = Level(i)
			return nil
		}
	}
	return fmt.Errorf("list: unknown level %q", b)
}

// View is the data of one view: a pointer to Task, Queue, Blockage or
// History. Each embeds Head, which supplies the method.
type View interface{ header() *Head }

// header returns the head itself.
func (h *Head) header() *Head { return h }

// command returns the name of the command whose envelope holds v.
func command(v View) string {
	switch v.(type) {
	case *Task:
		return "task"
	case *Queue:
		return "queue"
	case *Blockage:
		return "blockage"
	case *History:
		return "history"
	}
	return ""
}

// Decode reads tablo's envelope of one view into v: the source, the project
// and the parameters into the head, the level from the parameters, and the
// data into the rest, the legend among it (tabloio e3ed decision 17). It
// refuses an envelope of another command and one with no data.
func Decode(r io.Reader, v View) error {
	var env envelope
	if err := json.NewDecoder(r).Decode(&env); err != nil {
		return err
	}
	if want := command(v); env.Command != want {
		return fmt.Errorf("the envelope is of the command %q, not %q", env.Command, want)
	}
	data := bytes.TrimSpace(env.Data)
	if len(data) == 0 || string(data) == "null" {
		return errors.New("the envelope holds no data")
	}
	if err := json.Unmarshal(data, v); err != nil {
		return err
	}
	h := v.header()
	h.Level, h.Params = env.Params.Level, env.Params.Params
	if env.Source != nil {
		h.Source = *env.Source
	}
	if env.Project != nil {
		h.Project = *env.Project
	}
	return nil
}

// envelope is what Decode reads of tablo's envelope. The viewer and the
// diagnostics are not a pane's to draw.
type envelope struct {
	Command string          `json:"command"`
	Source  *Source         `json:"source"`  // null for no view
	Project *Project        `json:"project"` // null when the files give no root task
	Params  envelopeParams  `json:"params"`
	Data    json.RawMessage `json:"data"`
}

// envelopeParams is the envelope's params: the parameters in force and the
// level.
type envelopeParams struct {
	Params
	Level Level `json:"level"`
}

// Head is what every view carries beside its own data. Decode fills all but
// the legend from the envelope; the legend is the data's.
type Head struct {
	Level   Level   `json:"-"`      // the level tablo derives the data at: the envelope's params.level
	Project Project `json:"-"`      // the envelope's project; zero when it is null
	Source  Source  `json:"-"`      // the envelope's source
	Params  Params  `json:"-"`      // the envelope's params, as tablo resolves them
	Legend  Legend  `json:"legend"` // the data's legend, whole at every level
}

// Project names the project: the envelope's project.
type Project struct {
	Root  string `json:"root"`  // the root task's id
	Title string `json:"title"` // the root task's title
}

// Source is what the view reads: the envelope's source.
type Source struct {
	Ref    string `json:"ref"`    // the name given; for the working tree, the branch HEAD names
	Commit string `json:"commit"` // in full; "" where it is null
	Date   string `json:"date"`   // its author date, YYYY-MM-DD; "" where it is null
}

// Params echoes the parameters the four views take; a zero field is not in
// force, and null reads as zero.
type Params struct {
	Task   string `json:"task"` // the task in focus; the root where tablo resolves it so
	Person string `json:"person"`
}

// Legend is gates.yaml and the method's marks, in file order.
type Legend struct {
	Gates   []Gate   `json:"gates"`
	States  []State  `json:"states"`
	Reasons []Reason `json:"reasons"`
	Marks   []Mark   `json:"marks"`
}

// Gate is one gate of gates.yaml.
type Gate struct {
	Key      string `json:"key"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Criteria string `json:"criteria"`
}

// State is one state of gates.yaml.
type State struct {
	Key      string `json:"key"`
	Symbol   string `json:"symbol"`
	Synopsis string `json:"synopsis"`
	Severity int    `json:"severity"`
}

// Reason is one reason of gates.yaml; the reserved one is the method's.
type Reason struct {
	Key      string `json:"key"`
	Symbol   string `json:"symbol"`
	Synopsis string `json:"synopsis"`
	Reserved bool   `json:"reserved"` // true for review
}

// Mark is a junction mark of the method. Its key is person, agent, reviewer,
// subproject or exempt.
type Mark struct {
	Key      string `json:"key"`
	Symbol   string `json:"symbol"`
	Meaning  string `json:"meaning"`
	Junction string `json:"junction"`
}

// TaskRef names a task.
type TaskRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Person is an actor; Name is empty where only the email is known.
type Person struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Status is a status as a view shows it: stated, rolled up, or from a snapshot.
type Status struct {
	Gate     string  `json:"gate"`
	State    string  `json:"state"`
	Reason   string  `json:"reason"`
	Note     string  `json:"note"`
	Date     string  `json:"date"`
	Recorder string  `json:"recorder"`
	From     *Origin `json:"from"` // nil for a status file
}

// Origin says where a derived status comes from.
type Origin struct {
	Kind string  `json:"kind"` // "rollup" or "snapshot"
	Task TaskRef `json:"task"` // the child, or the subproject's task
	URL  string  `json:"url"`  // snapshot: the subproject
	Pin  string  `json:"pin"`  // snapshot: the commit read
	Gate string  `json:"gate"` // snapshot: where the subproject's task stands
}

// Commit is a Git fact.
type Commit struct {
	Hash       string   `json:"hash"`
	Subject    string   `json:"subject"`
	Date       string   `json:"date"` // author date
	Author     Person   `json:"author"`
	Committer  Person   `json:"committer"`
	AuthorTime string   `json:"author_time"` // RFC 3339 with the author's offset
	Trailers   []string `json:"trailers"`
	Files      []File   `json:"files"`
}

// File is a path a commit changes; Change is added, changed, removed or pin.
type File struct {
	Path   string `json:"path"`
	Change string `json:"change"`
}

// Command is a line a reader runs to reproduce or resolve a fact.
type Command struct {
	Text    string `json:"text"`
	Comment string `json:"comment"`
}

// Marked pairs a junction's marks, in the legend's order, with whether the
// person in view acts there.
type Marked struct {
	Marks []string `json:"marks"` // keys of Legend.Marks
	Acts  bool     `json:"acts"`
}

// Reference is a link a task or a junction states.
type Reference struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}
