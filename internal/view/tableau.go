// Package view holds the tableau as the grid reads it, and its decoder. The
// types mirror the envelope tablo emits for the commands tableau and context
// (tablo's view.Tableau and its Envelope), tag for tag where they carry the
// fact; a front end draws them and derives nothing from them. Decoding
// ignores unknown fields and leaves absent ones zero.
package view

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Tableau is one tableau, global or contextual, as tablo's envelope carries
// it. The head comes from the envelope: Command, Source, Project, Viewer and
// Params. The rest is the envelope's data. The grid draws it and derives
// nothing from it.
type Tableau struct {
	Command string  `json:"-"` // the envelope's command: "tableau" or "context"
	Source  Source  `json:"-"`
	Project Project `json:"-"`
	Viewer  Viewer  `json:"-"`
	Params  Params  `json:"-"`

	Legend  Legend   `json:"legend"`
	Subject *TaskRef `json:"subject"` // context with a task: that task; else nil
	Columns []Column `json:"columns"` // every gate once, in order, alone or in a folded run
	Rows    []Row    `json:"rows"`    // the rows of the level, in display order
}

// Source is what the tableau reads: the envelope's source.
type Source struct {
	Ref    string `json:"ref"`    // the name given; for the working tree, the branch HEAD names
	Commit string `json:"commit"` // in full; "" where it is null
	Date   string `json:"date"`   // the commit's author date, YYYY-MM-DD; "" where it is null
}

// Project names the project: the envelope's project; zero when it is null.
type Project struct {
	Root  string `json:"root"`  // the root task's id
	Title string `json:"title"` // the root task's title
}

// Params holds the parameters tablo resolves for the tableau: the
// envelope's params.
type Params struct {
	Task       string `json:"task"`   // the contextual tableau's task; "" when unset
	Person     string `json:"person"` // "" when unset
	Window     *int   `json:"window"` // nil where columns stand in its place
	Historical bool   `json:"historical"`
	Role       string `json:"role"`  // the role whose level the view opens at; "" when unset
	Level      string `json:"level"` // glance, detail or provenance
}

// Legend is gates.yaml and the method's marks, in file order.
type Legend struct {
	Gates   []Gate   `json:"gates"`
	States  []State  `json:"states"`
	Reasons []Reason `json:"reasons"`
	Marks   []Mark   `json:"marks"` // person, agent, reviewer, subproject, exempt
}

// Gate is one gate of gates.yaml.
type Gate struct {
	Key    string `json:"key"`
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
}

// State is one state of gates.yaml.
type State struct {
	Key    string `json:"key"`
	Symbol string `json:"symbol"`
}

// Reason is one reason of gates.yaml.
type Reason struct {
	Key    string `json:"key"`
	Symbol string `json:"symbol"`
}

// Mark is a junction mark of the method.
type Mark struct {
	Key    string `json:"key"` // person, agent, reviewer, subproject or exempt
	Symbol string `json:"symbol"`
}

// Column is one column of the window: a gate, or the run of gates a fold
// stands for. The window and the fold say which gates show; a row holds one
// cell per gate whatever they say. The counts hold on every column, folded
// or not.
type Column struct {
	Gates  []string `json:"gates"` // one gate, or the run a folded column stands for
	Folded bool     `json:"folded"`
	Count  int      `json:"count"`   // the rows of the detail level whose status names a gate of the column
	ByGate []int    `json:"by_gate"` // the same count per gate of the column
}

// TaskRef names a task.
type TaskRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Row is one task of a tableau. Label is "", "spine" or "sibling". Cells
// holds one cell per gate of the legend, in its order, whatever the columns.
// The fields after Cells arrive at provenance: the task definition's facts
// for this row's task.
type Row struct {
	TaskRef
	Depth  int    `json:"depth"`  // the number of its ancestors that the data holds a row for
	Parent bool   `json:"parent"` // the task has children
	Label  string `json:"label"`  // context: "", spine or sibling; tableau: ""
	Status Status `json:"status"`
	Leaf   *Leaf  `json:"leaf"` // a roll-up or a snapshot: the task whose file states the status; else nil
	Cells  []Cell `json:"cells"`

	StatusCommit *Commit    `json:"status_commit"` // the deciding commit of the last status of the chain
	OwnCommit    *Commit    `json:"own_commit"`    // a snapshot: the deciding commit of the task's own file
	Chain        []Step     `json:"chain"`         // the statuses the status comes through, the task's own first
	Junctions    []Junction `json:"junctions"`     // every gate after undefined, in order
	Sources      []Supplier `json:"sources"`       // of those junctions, by gate, then field
	Linkages     []Linkage  `json:"linkages"`      // one per subproject a recursive junction reads
}

// Status is a task's status as the tableau shows it.
type Status struct {
	Kind     string  `json:"kind"` // recorded, absent, rollup or snapshot
	Gate     string  `json:"gate"`
	State    string  `json:"state"`
	Reason   string  `json:"reason"`
	Note     string  `json:"note"`
	Date     string  `json:"date"`     // YYYY-MM-DD; "" when undetermined
	Recorder string  `json:"recorder"` // an email; "" for an absent status and a roll-up
	From     *Origin `json:"from"`     // nil for a recorded and an absent status
}

// Origin says where a rolled-up or a snapshot status comes from.
type Origin struct {
	Kind string  `json:"kind"` // rollup or snapshot
	Task TaskRef `json:"task"` // rollup: the child; snapshot: the task read in the subproject
	URL  string  `json:"url"`  // snapshot: the subproject
	Pin  string  `json:"pin"`  // snapshot: the commit read, in full
	Gate string  `json:"gate"` // snapshot: the gate the task read there stands at
}

// Leaf names the task at the end of a status's chain.
type Leaf struct {
	Task TaskRef `json:"task"`
	URL  string  `json:"url"` // the subproject the task is of; "" in the project in view
}

// Cell is one task at one gate. Kind is "empty", "status", "marks" or
// "exempt". A cell at the gate the status names shows the status; one before
// it is empty, or shows its junction's marks where the view shows historical
// junctions; one after it shows its junction's marks, or reads exempt.
type Cell struct {
	Kind       string   `json:"kind"`
	Historical bool     `json:"historical"` // the gate lies before the gate the status names
	State      string   `json:"state"`      // status: a key of Legend.States
	Reason     string   `json:"reason"`     // status: a key of Legend.Reasons, or ""
	Marks      []string `json:"marks"`      // marks: keys of Legend.Marks, the contributor's first
	Acts       bool     `json:"acts"`       // the person in view contributes or reviews here, and the work is to come
}

// Person is an identity as a commit records it.
type Person struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Commit is a commit fact.
type Commit struct {
	Hash      string   `json:"hash"` // in full
	Subject   string   `json:"subject"`
	Date      string   `json:"date"` // the author date, YYYY-MM-DD
	Author    Person   `json:"author"`
	Committer Person   `json:"committer"`
	Trailers  []string `json:"trailers"` // "Key: value", in order
}

// Step is one status of a chain.
type Step struct {
	Task   TaskRef `json:"task"`
	URL    string  `json:"url"` // the subproject the task is of; "" in the project in view
	Status Status  `json:"status"`
}

// Junction is a task's junction at one gate, resolved.
type Junction struct {
	Gate        string   `json:"gate"`
	Marks       []string `json:"marks"` // keys of Legend.Marks, the contributor's first
	Contributor string   `json:"contributor"`
	Model       string   `json:"model"`
	Reviewer    string   `json:"reviewer"`
}

// Supplier says what supplies one field of one junction: a task id, or
// default, own or assignee.
type Supplier struct {
	Gate  string `json:"gate"`
	Field string `json:"field"` // contributor, model, reviewer, references, subproject or applies
	Value string `json:"value"`
	By    string `json:"by"`
}

// Linkage is one subproject a recursive junction reads.
type Linkage struct {
	URL   string  `json:"url"`    // as the link resolves it
	SetBy *Commit `json:"set_by"` // the commit of the newest pin event of the link
}

// envelope is what Decode reads of tablo's envelope.
type envelope struct {
	Command string          `json:"command"`
	Source  *Source         `json:"source"`
	Project *Project        `json:"project"`
	Viewer  *Viewer         `json:"viewer"`
	Params  Params          `json:"params"`
	Data    json.RawMessage `json:"data"`
}

// Decode reads tablo's envelope of one tableau: the command, the source,
// the project, the viewer and the parameters into the head, and the data
// into the rest. It refuses an envelope of another command or with no data,
// columns that do not name every gate of the legend once and in order, a row
// whose cells do not match the gates one for one, a depth that rises by more
// than one, and an unknown kind of cell.
func Decode(r io.Reader) (Tableau, error) {
	var env envelope
	dec := json.NewDecoder(r)
	if err := dec.Decode(&env); err != nil {
		return Tableau{}, fmt.Errorf("tableau: %w", err)
	}
	if dec.More() {
		return Tableau{}, errors.New("tableau: data after the envelope")
	}
	if env.Command != "tableau" && env.Command != "context" {
		return Tableau{}, fmt.Errorf("tableau: the envelope is of the command %q", env.Command)
	}
	data := bytes.TrimSpace(env.Data)
	if len(data) == 0 || string(data) == "null" {
		return Tableau{}, errors.New("tableau: the envelope holds no data")
	}
	var t Tableau
	if err := json.Unmarshal(data, &t); err != nil {
		return Tableau{}, fmt.Errorf("tableau: %w", err)
	}
	t.Command, t.Params = env.Command, env.Params
	if env.Source != nil {
		t.Source = *env.Source
	}
	if env.Project != nil {
		t.Project = *env.Project
	}
	if env.Viewer != nil {
		t.Viewer = *env.Viewer
	}
	return t, t.check()
}

// check refuses what the grid cannot draw: see Decode.
func (t Tableau) check() error {
	gates := t.Legend.Gates
	k := 0
	for i, c := range t.Columns {
		if len(c.Gates) == 0 {
			return fmt.Errorf("tableau: column %d names no gate", i+1)
		}
		for _, g := range c.Gates {
			if k >= len(gates) || gates[k].Key != g {
				return fmt.Errorf("tableau: column %d names gate %q out of the legend's order", i+1, g)
			}
			k++
		}
	}
	if k != len(gates) {
		return fmt.Errorf("tableau: the columns name %d of %d gates", k, len(gates))
	}
	prev := -1
	for i, row := range t.Rows {
		if len(row.Cells) != len(gates) {
			return fmt.Errorf("tableau: row %d (%s) has %d cells for %d gates", i+1, row.ID, len(row.Cells), len(gates))
		}
		if row.Depth < 0 {
			return fmt.Errorf("tableau: row %d (%s) has depth %d", i+1, row.ID, row.Depth)
		}
		if i > 0 && row.Depth > prev+1 {
			return fmt.Errorf("tableau: row %d (%s) rises from depth %d to %d", i+1, row.ID, prev, row.Depth)
		}
		prev = row.Depth
		for j, c := range row.Cells {
			switch c.Kind {
			case "empty", "status", "marks", "exempt":
			default:
				return fmt.Errorf("tableau: row %d (%s) cell %d has unknown kind %q", i+1, row.ID, j+1, c.Kind)
			}
		}
	}
	return nil
}
