// Package list holds the data of the four list-shaped views the detail panes
// draw: the task definition, the work queue, the work-blockage tree and the
// history. The types mirror the data object tablo emits for each view, tag
// for tag as tabloio's renderers read it (tabloio designs e3ed, a3cc, 9167).
// Decoding ignores unknown fields and leaves absent ones zero.
package list

import (
	"encoding/json"
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

// Decode reads the data object of one view into v, a pointer to Task, Queue,
// Blockage or History.
func Decode(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

// Head is what every view's data carries; each view struct embeds it.
type Head struct {
	Level   Level   `json:"level"`   // the deepest level the data holds
	Project TaskRef `json:"project"` // the root task
	Ref     Ref     `json:"ref"`
	Params  Params  `json:"params"` // the parameters in force, as tablo resolves them
	Legend  Legend  `json:"legend"`
}

// Ref is the commit in view.
type Ref struct {
	Name    string `json:"name"`   // as given, "HEAD" by default
	From    string `json:"from"`   // the start of a range; history only
	Commit  string `json:"commit"` // the full hash
	Date    string `json:"date"`   // its author date, YYYY-MM-DD
	OnTrunk bool   `json:"on_trunk"`
}

// Params echoes the focusing parameters; a zero field is not in force.
type Params struct {
	Task       string   `json:"task"` // empty when the view covers the root
	Person     string   `json:"person"`
	Window     *int     `json:"window"`
	Columns    []string `json:"columns"`
	Historical bool     `json:"historical"`
	Proposed   bool     `json:"proposed"`
	Stale      int      `json:"stale"`
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
	Hash          string   `json:"hash"`
	Subject       string   `json:"subject"`
	Date          string   `json:"date"` // author date
	Author        Person   `json:"author"`
	Committer     Person   `json:"committer"`
	AuthorTime    string   `json:"author_time"` // RFC 3339 with offset
	CommitterTime string   `json:"committer_time"`
	Trailers      []string `json:"trailers"`
	Files         []File   `json:"files"`
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
