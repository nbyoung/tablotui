# Design 0afa: the declarations and the line tables

This draft holds what [the design](../0afa.md) is too long to carry: every Go declaration, the functions the golden files bind byte for byte, and the lines each pane draws at each level. A trial outside the repository compiles all of it against `main` at `21084d3` and writes the golden files beside this one.

## `internal/view/list`

The package holds the data of the four list-shaped views. Every type and tag is tabloio's: `list.go` and `task.go` copy `internal/render/view/view.go` and `task.go` of tabloio `main` at `5449b0e`, less the registry and the gate definition view, and `queue.go`, `blockage.go` and `history.go` copy the declarations of tabloio's designs `a3cc` and `9167`, less the tableau, the audit and the two fields no pane draws, `Queue.Brief` and `History.Days`.

```go
// Package list holds the data of the four list-shaped views the detail panes
// draw: the task definition, the work queue, the work-blockage tree and the
// history. The types mirror the data object tablo emits for each view, tag
// for tag as tabloio's renderers read it (tabloio designs e3ed, a3cc, 9167).
// Decoding ignores unknown fields and leaves absent ones zero.
package list

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
```

```go
// task.go
// Task is the task definition view.
type Task struct {
	Head
	Task          TaskRef        `json:"task"`
	Assignee      string         `json:"assignee"`
	Parent        *Parent        `json:"parent"` // nil for the root
	Status        Status         `json:"status"`
	Description   string         `json:"description"` // detail from here
	References    []Reference    `json:"references"`
	Path          []TaskRef      `json:"path"`     // the root first, the task last
	Siblings      int            `json:"siblings"` // children of the parent, the task included
	Children      []ChildRow     `json:"children"`
	Authorities   []Authority    `json:"authorities"` // nearest first
	Authorised    bool           `json:"authorised"`
	Requires      []Requirement  `json:"requires"`
	Dependents    []Requirement  `json:"dependents"`
	Junctions     []Junction     `json:"junctions"`     // every gate, in order
	Snapshot      *Snapshot      `json:"snapshot"`      // a recursive next junction
	StatusCommit  *Commit        `json:"status_commit"` // provenance from here
	Authorisation *Authorisation `json:"authorisation"`
	Rollup        []Fact         `json:"rollup"` // a parent: the steps of its roll-up
	Linkages      []Linkage      `json:"linkages"`
	Sources       []FieldSource  `json:"sources"`
	Reviews       []Review       `json:"reviews"`
	Models        []ModelCheck   `json:"models"`
	Events        []Event        `json:"events"` // newest first, at most five
	EventCount    int            `json:"event_count"`
	Commands      []Command      `json:"commands"`
}

// Parent is the parent of a task and the task's place among its siblings.
type Parent struct {
	TaskRef
	Order int `json:"order"`
}

// ChildRow is a child with its status.
type ChildRow struct {
	TaskRef
	Status Status `json:"status"`
}

// Authority is an assignee whose authority covers the task.
type Authority struct {
	Email string `json:"email"`
	By    string `json:"by"` // the ancestor's id
}

// Requirement is an edge between two tasks, seen from either end.
type Requirement struct {
	Task       TaskRef `json:"task"` // the other task
	From       string  `json:"from"`
	To         string  `json:"to"`
	Text       string  `json:"text"`
	Met        bool    `json:"met"`
	Due        bool    `json:"due"`
	Status     *Status `json:"status"`     // of the required task; nil for a dependent
	Subproject string  `json:"subproject"` // a cross-project entry: its url
	Commit     string  `json:"commit"`     // and the commit it reads
}

// Junction is one gate's junction of the task.
type Junction struct {
	Gate string `json:"gate"`
	Marked
	Contributor string      `json:"contributor"`
	Model       string      `json:"model"`
	Reviewer    string      `json:"reviewer"`
	ByDefault   bool        `json:"reviewer_by_default"` // the assignee reviews, since no entry states one
	Subproject  *SubRef     `json:"subproject"`
	References  []Reference `json:"references"`
	Stands      string      `json:"stands"` // passed, here, next, later, exempt
	Reviewed    bool        `json:"reviewed"`
}

// SubRef names a task of a subproject.
type SubRef struct {
	URL  string  `json:"url"`
	Task TaskRef `json:"task"`
}

// Snapshot is a subproject's task as a recursive junction reads it.
type Snapshot struct {
	SubRef
	Assignee string     `json:"assignee"`
	Status   Status     `json:"status"`
	Children []ChildRow `json:"children"`
}

// Authorisation is the commit that authorises a task and how it does.
type Authorisation struct {
	Commit Commit `json:"commit"`
	Way    string `json:"way"` // change, merge, trailer
	By     string `json:"by"`  // author, committer, both, or empty for a proposed task
}

// Fact is a name and a value in tablo's words.
type Fact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Linkage lists the facts that fix a recursive junction's subproject.
type Linkage struct {
	Gate  string `json:"gate"` // the first gate that states it
	Facts []Fact `json:"facts"`
}

// FieldSource says which task supplies one field of one junction.
type FieldSource struct {
	Gate  string `json:"gate"`
	Field string `json:"field"`
	Value string `json:"value"`
	By    string `json:"by"` // an id, or "default"
}

// Review is the commit that accepts a reviewed junction.
type Review struct {
	Gate     string  `json:"gate"`
	Reviewer string  `json:"reviewer"`
	Commit   *Commit `json:"commit"` // nil where the authorisation stands as the review
	Effect   string  `json:"effect"` // accepts, authorisation, none
}

// ModelCheck compares a junction's model with a commit's Model trailer.
type ModelCheck struct {
	Commit  string `json:"commit"`
	Gate    string `json:"gate"`
	Stated  string `json:"stated"`
	Trailer string `json:"trailer"`
	Reading string `json:"reading"` // agrees, differs, missing, exempt
}

// Event is one entry of a task's history.
type Event struct {
	Date   string   `json:"date"`
	Commit string   `json:"commit"`
	By     string   `json:"by"`
	Kinds  []string `json:"kinds"` // task, authorised, status, reaffirmed, reviewed, pin
	Status *Status  `json:"status"`
	Gate   string   `json:"gate"`
	Pin    *PinMove `json:"pin"`
}

// PinMove is a subproject pin that moves from one commit to another.
type PinMove struct {
	URL string `json:"url"`
	Old string `json:"old"`
	New string `json:"new"`
}
```

```go
// queue.go
// Queue is the contributor work queue of one person.
type Queue struct {
	Head
	Items []QueueItem `json:"items"` // kind order, then display order; reaffirmations oldest first
}

// QueueItem is one thing the person does.
type QueueItem struct {
	Kind  string  `json:"kind"` // review, authorisation, ready, reaffirmation, waiting
	Task  TaskRef `json:"task"`
	Gate  string  `json:"gate"` // the next gate; a reaffirmation: the gate the status names
	Model string  `json:"model"`
	Since string  `json:"since"` // the date of the status
	Age   int     `json:"age"`   // reaffirmation: days from the status to the ref's commit
	Cause string  `json:"cause"` // waiting, authorisation: worded text

	Junction       *Junction     `json:"junction"` // detail from here
	Sources        []FieldSource `json:"sources"`
	TaskReferences []Reference   `json:"task_references"`
	Requires       []Requirement `json:"requires"`
	Unblocks       []Requirement `json:"unblocks"`
	AlsoRequiredBy []Requirement `json:"also_required_by"`
	ParentUnblocks []ParentEdge  `json:"parent_unblocks"`
	Status         *Status       `json:"status"`
	StatusCommit   *Commit       `json:"status_commit"`
}

// ParentEdge is a requirement that a parent of the item's task meets.
type ParentEdge struct {
	Parent   TaskRef     `json:"parent"`
	Gate     string      `json:"gate"`
	Requires Requirement `json:"requires"` // Task is the dependent
}
```

```go
// blockage.go
// Blockage is the work-blockage tree.
type Blockage struct {
	Head
	Causes      []Cause     `json:"causes"` // largest first, then display order
	Kinds       []KindCount `json:"kinds"`
	NotDue      []Waiting   `json:"not_due"` // detail: by task, in display order
	NotDueN     int         `json:"not_due_count"`
	NotDueUnmet int         `json:"not_due_unmet"`
	Commands    []Command   `json:"commands"` // provenance
}

// KindCount counts the items of one kind.
type KindCount struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// Cause is one root of the tree.
type Cause struct {
	Kind     string    `json:"kind"` // requirement, status, review, authorisation, snapshot
	Task     TaskRef   `json:"task"`
	Gate     string    `json:"gate"`   // requirement, review: the gate not passed
	Status   *Status   `json:"status"` // status: what the task states
	URL      string    `json:"url"`    // snapshot
	Pin      string    `json:"pin"`    // snapshot
	Resolver string    `json:"resolver"`
	Mark     string    `json:"mark"`    // the resolver's mark key, or empty
	Act      string    `json:"act"`     // contributes, reviews, authorises, records, advances
	Holds    int       `json:"holds"`   // distinct tasks beneath, at every depth
	Action   string    `json:"action"`  // detail: worded text
	Held     []Held    `json:"held"`    // detail
	Facts    []Fact    `json:"facts"`   // provenance
	Resolve  []Command `json:"resolve"` // provenance
}

// Held is a task a cause holds, and what it holds in turn.
type Held struct {
	Task     TaskRef      `json:"task"`
	Gate     string       `json:"gate"`
	Parent   bool         `json:"parent"`
	Via      string       `json:"via"`      // self, parent, requirement
	Child    string       `json:"child"`    // parent: the child that holds it
	Requires *Requirement `json:"requires"` // requirement: the entry
	Also     []int        `json:"also"`     // the other causes it stands under, from 1
	Held     []Held       `json:"held"`
}

// Waiting is a task with requirements not yet due.
type Waiting struct {
	Task     TaskRef       `json:"task"`
	Status   Status        `json:"status"`
	Next     string        `json:"next"`
	Requires []Requirement `json:"requires"`
}
```

```go
// history.go
// History is the history view.
type History struct {
	Head
	Subject  *TaskRef    `json:"subject"` // the task parameter; nil for the whole project
	Lines    []Line      `json:"lines"`   // oldest first
	Events   int         `json:"events"`
	Commits  int         `json:"commits"`
	Kinds    []KindCount `json:"kinds"`
	Commands []Command   `json:"commands"` // provenance
}

// Line is what one commit does to one task.
type Line struct {
	Date     string   `json:"date"`
	By       string   `json:"by"`
	Kinds    []string `json:"kinds"`
	Task     TaskRef  `json:"task"`
	Proposal bool     `json:"proposal"`
	Status   *Status  `json:"status"`
	Gate     string   `json:"gate"`
	Pin      *PinMove `json:"pin"`

	After        *Status    `json:"after"` // detail from here
	Unchanged    bool       `json:"unchanged"`
	SubjectAfter *Status    `json:"subject_after"`
	Effect       string     `json:"effect"`
	Model        string     `json:"model"`
	Stated       string     `json:"stated"`
	StatedGate   string     `json:"stated_gate"`
	Reading      string     `json:"reading"`
	Committer    string     `json:"committer"`
	Sub          *SubEvents `json:"subproject"`

	Commit   *Commit   `json:"commit"` // provenance
	Commands []Command `json:"commands"`
}

// SubEvents is a subproject's history between two pins.
type SubEvents struct {
	URL     string `json:"url"`
	Events  int    `json:"events"`
	Commits int    `json:"commits"`
	Lines   []Line `json:"lines"`
}
```

## `internal/source/lists.go`

`source.File` gains the four methods, so one fixture directory serves the grid and the panes. Each method sets `View` itself and decodes the file that `ListName` names through `list.Decode`. A missing file is an error, a file that does not parse is an error that names the file, and a cancelled context returns its error, as `File.Tableau` does.

```go
// ListRequest names one list-shaped view and the parameters that focus it.
// It names no ref, no person and no role: the source holds them.
type ListRequest struct {
	View  string // "task", "queue", "blockage" or "history"
	Task  string // the task in focus; "" on the queue
	Level string // "detail" or "provenance"; "" is the level the viewer's role opens the view at
}

// Lists loads the list-shaped views. A call blocks; a pane runs it in a command.
type Lists interface {
	Task(ctx context.Context, r ListRequest) (list.Task, error)
	Queue(ctx context.Context, r ListRequest) (list.Queue, error)
	Blockage(ctx context.Context, r ListRequest) (list.Blockage, error)
	History(ctx context.Context, r ListRequest) (list.History, error)
}

// ErrNoPerson is the error Queue returns when the viewer has no email.
var ErrNoPerson = errors.New("the work queue needs a person, and the viewer states none")

// ListName returns the file name that serves r: the view, then "-<task>"
// when Task is set, then "-provenance" when Level is "provenance", then ".json".
func (f File) ListName(r ListRequest) string {
	name := r.View
	if r.Task != "" {
		name += "-" + r.Task
	}
	if r.Level == "provenance" {
		name += "-provenance"
	}
	return name + ".json"
}

func (f File) Task(ctx context.Context, r ListRequest) (list.Task, error)
func (f File) Queue(ctx context.Context, r ListRequest) (list.Queue, error)
func (f File) Blockage(ctx context.Context, r ListRequest) (list.Blockage, error)
func (f File) History(ctx context.Context, r ListRequest) (list.History, error)
```

## The addition to the frame and the grid

`internal/ui/messages.go` gains one type. `frame.go` does not change: the frame broadcasts a message it does not know to every pane.

```go
// GotoMsg asks the home pane to select a task, and a gate column when Gate
// names one. A detail pane sends it for the task its cursor line names; the
// home pane answers with a SelectionMsg, or a notice when it lacks the task.
type GotoMsg struct{ Task, Gate string }
```

`internal/ui/grid/grid.go` gains one case in `Update`, `case ui.GotoMsg: g, cmd = g.goTo(msg, c)`, beside `StartMsg`, and this method. The tail of `Update` then settles the scroll and announces the selection, as it does after a key.

```go
// goTo selects the task a pane names: it unfolds the task's ancestors, moves
// the row cursor, and moves the column cursor when the gate shows as a column
// of its own.
func (g Grid) goTo(msg ui.GotoMsg, c ui.Context) (Grid, tea.Cmd) {
	if g.data == nil {
		return g, nil
	}
	rows := g.data.Rows
	at := slices.IndexFunc(rows, func(r view.Row) bool { return r.ID == msg.Task })
	if at < 0 {
		return g, notice(msg.Task+" is not in the tableau in view", false)
	}
	g.open = maps.Clone(g.open)
	depth := rows[at].Depth
	for i := at - 1; i >= 0 && depth > 0; i-- {
		if rows[i].Depth < depth {
			g.open[rows[i].ID] = true
			depth = rows[i].Depth
		}
	}
	g.sel = msg.Task
	g = g.settled(c.Width, c.Height, c.Measure)
	if _, i, ok := g.gateOf(msg.Gate); ok {
		for k, col := range g.m.cols {
			if !col.folded && col.gates[0] == i {
				g.col = k + 1
			}
		}
	}
	return g, nil
}
```

## `internal/ui/detail`

| File | Holds |
|------|-------|
| `detail.go` | `Options`, `Set`, `New`, the commands, `arrangement` and its `layout` |
| `pane.go` | `Pane`, `kind`, the requests and the loads, the cursor, `View`, `Status` |
| `keys.go` | `keyMap`, `helpKeys` |
| `lines.go` | `Line`, `row`, `wrap`, `wrapLine` |
| `words.go` | `words` and its phrases, `task`, `short`, `condition`, `count`, `counted`, `join`, `cells`, `entry`, `foldRuns` |
| `task.go`, `queue.go`, `blockage.go`, `history.go` | One builder each |

### The exported surface

```go
// Package detail holds the four detail panes of the tablotui program: the
// task definition, the work queue, the work-blockage tree and the history.
// Each follows the grid's selection, draws what its source sends and derives
// nothing from it. The package also supplies the keys that open the panes and
// the layout that places them beside the grid.
package detail

// The identifiers of the four panes.
const (
	TaskID     = "task"
	QueueID    = "queue"
	BlockageID = "blockage"
	HistoryID  = "history"
)

// Options configures the panes.
type Options struct {
	Source source.Lists
}

// Set is what the command hands the frame: the panes after the home pane,
// the commands and the layout.
type Set struct {
	Panes    []ui.Pane
	Commands []ui.Command
	Layout   ui.Layout
}

// New builds the four panes, closed, with their commands and their layout.
func New(o Options) Set

// Pane is one detail pane. It implements ui.Pane.
type Pane struct{ /* below */ }
```

### The pane

```go
// kind is what tells the four panes apart.
type kind struct {
	id, name, digit string
	follows         bool       // the request names the selected task
	deepest         list.Level // the deepest level the pane draws
	tail            bool       // the cursor arrives on the last row
}

var kinds = []kind{
	{id: TaskID, name: "Task", digit: "1", follows: true, deepest: list.Provenance},
	{id: QueueID, name: "Queue", digit: "2", deepest: list.Detail},
	{id: BlockageID, name: "Blockage", digit: "3", follows: true, deepest: list.Provenance},
	{id: HistoryID, name: "History", digit: "4", follows: true, deepest: list.Provenance, tail: true},
}

// toggleMsg is what a digit command sends: the pane it names opens focused,
// takes the focus when open, and closes when it has the focus.
type toggleMsg struct{ pane string }

// Pane is one detail pane. It implements ui.Pane.
type Pane struct {
	k    kind
	src  source.Lists
	keys keyMap

	open bool
	sel  ui.SelectionMsg // the latest selection of the grid

	seq      uint64
	asked    source.ListRequest // the request in flight
	pending  bool
	have     source.ListRequest // the request the data answers
	data     any                // *list.Task and so on; nil before the first answer
	stale    bool               // a ReloadMsg arrived since the data did
	failed   bool               // the newest load failed and left no data
	noPerson bool               // it failed because the viewer has no email
	errText  string             // why it failed

	level  *list.Level // the level the viewer chose; nil draws the level the data states
	prov   bool        // the viewer has opened provenance
	unfold bool

	subject, counts string
	lines           []Line
	cur, sub        int // the cursor: a line and a row within it
	top, topSub     int // the first row drawn
}

// request derives what the pane asks for from the selection and the level.
func (p Pane) request() (source.ListRequest, bool)

// shown is the level the pane draws: the level the data states, lowered to
// the level the viewer chose and to the deepest level the pane draws.
func (p Pane) shown() list.Level

// sync asks for the request the pane now derives, unless the data in hand or
// the load in flight already answers it.
func (p Pane) sync(c ui.Context) (Pane, tea.Cmd)

// loaded takes the answer to the newest request.
func (p Pane) loaded(msg ui.LoadedMsg) (Pane, tea.Cmd)

// rebuilt builds the lines again from the data. With keep the cursor stays on
// the item it stood on; otherwise it arrives on the first line, or on the
// last for the history.
func (p Pane) rebuilt(keep bool) Pane

// body is the lines the pane draws: its document, or the one line that
// stands for it.
func (p Pane) body() (lines []Line, isErr bool)

// window is where the cursor and the first row stand among the rows.
type window struct {
	rows     []row
	cur, top int
}

// settle wraps the body for a pane of w by h cells and holds the cursor and
// the first row in range, the cursor in view. View calls it and stores
// nothing; Update stores its result.
func (p Pane) settle(w, h int, m ui.Measure) window

// stored returns the pane with the window's cursor and first row kept.
func (p Pane) stored(win window) Pane

// keyMap holds the bindings a detail pane dispatches on.
type keyMap struct {
	Up, Down, PageUp, PageDown, First, Last key.Binding
	Go, Detail, Provenance, Fold            key.Binding
}

func newKeyMap() keyMap {
	b := func(keys ...string) key.Binding { return key.NewBinding(key.WithKeys(keys...)) }
	return keyMap{
		Up: b("up", "k"), Down: b("down", "j"), PageUp: b("pgup"), PageDown: b("pgdown"),
		First: b("home", "g"), Last: b("end", "G"),
		Go: b("enter"), Detail: b("d"), Provenance: b("p"), Fold: b("f"),
	}
}
```

### The lines, the wrap and the fold

```go
// Line is one logical line of a pane's document. A builder returns lines; the
// pane wraps them to its width and draws the rows.
type Line struct {
	Label  string // drawn first and padded to Hang cells; "" for none
	Text   string // wraps at single spaces
	Indent int    // cells before the label or the text
	Hang   int    // cells before each continuation row, after Indent
	Head   bool   // a heading: draws in Styles.Header
	Marked bool   // the line names the task the grid selects: ">" stands in its indent
	Item   int    // the item of the view the line belongs to, from 0; -1 for none
	Task   string // the task enter selects in the grid; "" for none
	Gate   string // the gate enter moves the grid's column cursor to; "" for none
}

// row is one screen row of a wrapped line.
type row struct {
	text string
	line int // the index of its Line
	sub  int // its position within that line, from 0
}

// wrap breaks every line into rows of at most w cells.
func wrap(lines []Line, w int, m ui.Measure) []row {
	var out []row
	for i, l := range lines {
		for k, t := range wrapLine(l, w, m) {
			out = append(out, row{text: t, line: i, sub: k})
		}
	}
	return out
}

// wrapLine breaks one line. The indent never exceeds a quarter of the width and the
// hang a half, so every row keeps a quarter of it for its text.
func wrapLine(l Line, w int, m ui.Measure) []string {
	if w <= 0 {
		return []string{""}
	}
	indent := min(l.Indent, w/4)
	hang := min(l.Hang, w/2)
	first := strings.Repeat(" ", indent)
	room := w - indent
	if l.Label != "" {
		first += m.Fit(l.Label, hang)
		room -= hang
	}
	rest := strings.Repeat(" ", indent+hang)
	restRoom := w - indent - hang

	var rows []string
	cur, curW := "", 0
	prefix, limit := first, room
	flush := func() {
		rows = append(rows, strings.TrimRight(prefix+cur, " "))
		cur, curW = "", 0
		prefix, limit = rest, restRoom
	}
	for _, word := range strings.Fields(m.Clean(l.Text)) {
		ww := m.Method.StringWidth(word)
		if curW > 0 && curW+1+ww > limit {
			flush()
		}
		for ww > limit {
			if curW > 0 {
				flush()
			}
			head := m.Method.Truncate(word, limit, "")
			if head == "" {
				break
			}
			cur, curW = head, m.Method.StringWidth(head)
			flush()
			word = word[len(head):]
			ww = m.Method.StringWidth(word)
		}
		if curW > 0 {
			cur += " "
			curW++
		}
		cur += word
		curW += ww
	}
	if curW > 0 || len(rows) == 0 {
		flush()
	}
	if l.Marked && indent >= 2 {
		rows[0] = mark + strings.Repeat(" ", indent-1) + strings.TrimPrefix(rows[0], first)
	}
	return rows
}
```

```go
// count writes n and the word that agrees with it: "1 cause", "2 causes".
func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// counted writes a heading with its count, or "none".
func counted(name string, n int) string {
	if n == 0 {
		return name + ": none"
	}
	return fmt.Sprintf("%s: %d", name, n)
}

// join joins the non-empty parts.
func join(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// cells joins the non-empty cells of what the Markdown draws as a table row.
func cells(parts ...string) string { return join(" · ", parts...) }

// entry is one row of a list that may fold: the task and the cells that
// follow it.
type entry struct {
	task list.TaskRef
	rest string
	item int
	gate string
}

// foldRuns applies the run fold: a run of more than eight consecutive entries
// that agree in everything but the task keeps its first three, and one entry
// with an empty task id stands for the others, its title listing their ids.
func foldRuns(in []entry, unfold bool) []entry {
	if unfold {
		return in
	}
	var out []entry
	for i := 0; i < len(in); {
		j := i + 1
		for j < len(in) && in[j].rest == in[i].rest {
			j++
		}
		if j-i <= 8 {
			out = append(out, in[i:j]...)
			i = j
			continue
		}
		out = append(out, in[i:i+3]...)
		ids := make([]string, 0, j-i-3)
		for _, e := range in[i+3 : j] {
			ids = append(ids, e.task.ID)
		}
		out = append(out, entry{
			task: list.TaskRef{Title: fmt.Sprintf("… and %d more: %s", j-i-3, strings.Join(ids, " "))},
			rest: in[i].rest,
			item: in[i+3].item,
		})
		i = j
	}
	return out
}
```

### The layout

```go
// The sizes the layout keeps.
const (
	sideWidth   = 120 // from this body width the panes stand beside the grid
	stackHeight = 12  // from this body height they stand beneath it
	paneWidth   = 40  // the least width of a pane in a split
	paneHeight  = 5   // the least height of a pane in a split
	sideMax     = 80  // the most the panes' column takes
)

// arrangement is the one piece of layout state the frame does not hold:
// whether the focused pane takes the whole body. The z command and the
// layout share it.
type arrangement struct{ zoom bool }

// layout implements ui.Layout. open[0] is the home pane.
func (a *arrangement) layout(w, h int, open []string, focus string) []ui.Rect {
	if len(open) == 0 {
		return nil
	}
	home, panes := open[0], open[1:]
	whole := func(id string) []ui.Rect { return []ui.Rect{{Pane: id, W: w, H: h}} }
	focused := -1
	for i, id := range panes {
		if id == focus {
			focused = i
		}
	}
	side, stack := w >= sideWidth, h >= stackHeight
	switch {
	case len(panes) == 0:
		return whole(home)
	case a.zoom || !side && !stack:
		if focused >= 0 {
			return whole(focus)
		}
		return whole(home)
	}
	var region ui.Rect
	out := make([]ui.Rect, 0, len(open))
	if side {
		dw := min(max(w/3, paneWidth), sideMax)
		out = append(out, ui.Rect{Pane: home, W: w - dw, H: h})
		region = ui.Rect{X: w - dw, W: dw, H: h}
	} else {
		dh := h / 2
		out = append(out, ui.Rect{Pane: home, W: w, H: h - dh})
		region = ui.Rect{Y: h - dh, W: w, H: dh}
	}
	cols := max(1, region.W/paneWidth)
	rows := max(1, region.H/paneHeight)
	k := min(len(panes), cols*rows)
	start := len(panes) - k
	if focused >= 0 && focused < start {
		start = focused
	}
	shown := panes[start : start+k]
	used := (k + cols - 1) / cols
	y := region.Y
	for r := range used {
		rh := region.H / used
		if r < region.H%used {
			rh++
		}
		in := shown[r*cols : min((r+1)*cols, k)]
		x := region.X
		for i, id := range in {
			pw := region.W / len(in)
			if i < region.W%len(in) {
				pw++
			}
			out = append(out, ui.Rect{Pane: id, X: x, Y: y, W: pw, H: rh})
			x += pw
		}
		y += rh
	}
	return out
}
```

## The lines each pane draws

A builder turns one view's data and a level into a title and `[]Line`. The notation:

| Term | Reads |
|------|-------|
| `<task>` | The id, a space, the title |
| `<gate>` | The gate's symbol from the data's legend, a space, the key; the key alone when the legend holds no symbol. A state and a reason read the same way |
| `<status>` | `<gate>`, then the state and the reason where the status holds them, joined by spaces; then `, rolled up from <task>` or `, 🪆 from the subproject <url>`, the mark being the legend's `subproject` symbol |
| `<hash7>` | The first seven characters of a commit hash |
| `<condition>` | `met` or `unmet`, then `, not yet due` when the requirement is not due |
| cells(a, b, …) | The cells that are not empty, joined by ` · `: what the Markdown draws as one table row |
| `<count thing(s)>` | `count(n, one, many)`: `1 cause`, `2 causes` |
| `<Name>: <n>` | `counted(name, n)`: `Requires: 1`, `Requires: none` |
| target | The `Task` and `Gate` of the line; `—` leaves both empty |
| *i*/*h* | `Indent` *i*, `Hang` *h* |

A heading sets `Head`. A blank line is `Line{Item: -1}`. A line whose value is empty does not print, unless the table says otherwise. Worded text, which tablo writes with backticks around an id, a key or a trailer, prints as it arrives.

### The task definition

A field is `Line{Label, Text, Hang: 13}`. A commit field is `Line{Label, Text, Indent: 2, Hang: 18}`. Every section but the first opens with a blank line and a heading.

| Level | Item | Lines | Target |
|-------|------|-------|--------|
| glance | 0 | Heading `<task>` | — |
| | 0 | Fields `assignee`; `parent`: `<task>, order <order>`, or `none` for the root; `status`: `<status>`; `note`; `recorded`: `<date> by <recorder>`, for a roll-up `<date>, the oldest among its children`, for a snapshot `<date>, from the subproject`, and no line without a date | `parent`: the parent |
| provenance | 0 | `Status commit`, then the commit fields `commit`: `<hash7> <subject>`; `date`; `author` and `committer`: `<name>, <email>`; `trailers`, joined by `, ` | — |
| detail | 1 | `Description`; the description as one line; `References:`; per reference `<text>, <url>` at 2/2 | — |
| | 2 | `Place in the tree`; fields `path`: the tasks joined by ` › `; `order`: `<order> of <siblings> under <parent id>`, not for the root; `children`: `none`, or the number and then per child cells(`<task>`, `<status>`) at 2/2; `authorities`: consecutive ancestors of one email as `<email> (<id>, <id>)`, the groups joined by `; then `, or `none`; `authorised`: `yes` or `no, proposed` | A child and the gate of its status |
| provenance | 2 | `Authorisation commit`; the commit fields; `accepts by`: `a change on the trunk`, `a merge` or `an Authorised: trailer`; `the authority is`: `the author`, `the committer` or `the author and the committer` | — |
| detail | 3 | `Requires: <n>`; per entry after the run fold, at 2/2: cells(`<task>`, `<from> → <to>`, the text, the `<status>` of the required task, `<condition>`). A cross-project entry's task reads `<task> in <url> at <hash7>` | The other task and `from` |
| | 4 | `Dependents: <n>`; the same line without the status | The other task and `to` |
| | 5 | `Junctions`; per gate at 2/2: cells(`<gate>`, the marks' symbols with no space, the contributor, the model, `reviewer <email>` or `reviewer the assignee, <email>`, `<url> <task>` of a subproject, the standing). The standing reads `passed`, `next`, `later` or `does not apply`, then `, reviewed`, then `; the status stands here` for `here` | The task and the gate |
| | 6 | With a snapshot: `Subproject snapshot`; fields `task`: `<url> <task>`; `assignee`; `status`; `note`; per child cells(`<task>`, `<status>`) at 2/2 | — |
| provenance | 7 | With roll-up facts: `Roll-up`; per fact `<name>: <value>` at 2/2 | — |
| | 8 | With a linkage: `Linkages`; per linkage `<gate>` at 2/0, then its facts at 4/2 | The task and the gate, on the gate line |
| | 9 | `Where each junction field comes from`; per source at 2/2: cells(`<gate>`, the field, the value, the id or `the plain default`) | The task and the gate |
| | 10 | `Reviews: <n>`; at 2/2: cells(`<gate>`, the reviewer, `<hash7> <subject>`, the effect: `the reviewer accepts`, `the authorisation stands as the review` or `none`) | The task and the gate |
| | 11 | `Models: <n>`; at 2/2: cells(`<hash7>`, `<gate>`, `states <stated>`, `trailer <trailer>`, the reading: `agrees`, `differs`, `no trailer` or `exempt`) | The task and the gate |
| | 12 | `Newest events`; `<count event(s)>; the newest <n>, newest first.` at 2/0; per event at 2/2: cells(date, `<hash7>`, by, the kinds joined by `, `, what it records: `<status>: <note>`, the `<gate>` of a review, or the pin); `… and <m> earlier` when events remain | — |
| | 13 | With commands: `Commands`; per command `<text> # <comment>` at 2/2 | — |

A pin reads `<url> <hash7> → <hash7>`, or `<url> new <hash7>` when it first appears. The title's subject is the task's id and it carries no count.

### The work queue

The five kinds and their headings, in order: `review` Reviews owed, `authorisation` Authorisations owed, `ready` Work ready, `reaffirmation` Reaffirmations, `waiting` Work waiting. A kind with no item prints nothing; a blank line stands between two kinds. The item number is the item's index in `items`. The title's subject is `params.person` and its count `<count item(s)>`.

| Level | Lines | Target |
|-------|-------|--------|
| glance | No item at all: `The queue is empty.` and nothing else | — |
| | Per kind: heading `<Heading>: <n>`, with the item number of its first item | — |
| | Per entry after the run fold, at 2/2: cells(`<task>`, or `<task>; <cause>` where the item has one, `<gate>`, the model, the date, which for a reaffirmation reads `<date>, <count day(s)>`). `Marked` is true when the task is the grid's selection. Two entries agree, for the fold, when the cells after the first agree | The task and the gate |
| | The fold line: its task cell reads `… and <n> more: <ids>`; `Marked` when the ids hold the selection | — |
| detail | Beneath each entry that is no fold line, at 4/2, each only when the data holds it: `Gate: <gate>, <name>: <criteria>` from the legend; `Contributor: <symbol of the junction's first mark> <email>, model <model>, from <id>`, the last part reading `by default` when the source says `default`; `Reviewer: the assignee, <email>`, or `Reviewer: <symbol of the reviewer mark> <email>, from <id>`; `References:` at 4/0, then the junction's and the task's as `<text>, <url>` at 6/2; `Status: <status>, <date>, <recorder>, <hash7>: <note>`; for a waiting item `Do not start: the cause stands.` | The item's task and gate |
| | Between References and Status, each with its label at 4/0 and its entries at 6/2: `Requires:`, `Unblocks:`, `Also required by:`, `Its parent unblocks:`. An entry reads `<task>, <from> → <to>, <text>: <condition>`; a parent's entry opens `<parent task> at <gate> · ` | The entry's task, and `from` under Requires, `to` under the others |

### The work-blockage tree

The item number is the cause's index; the lines under `Next` take the number of causes, and the closing commands that number and one. The title's subject is the task of the request and its count `<count cause(s)>`. The target of a cause, its action, its facts and its commands is the cause's task with its gate, or the gate of its status.

| Level | Lines | Target |
|-------|-------|--------|
| glance | No cause: `No cause holds any task.` | — |
| | Per cause at 0/2: cells(the cause, `<symbol of its mark> <resolver> <act>`, `holds <n>`). The cause reads, by kind: `<task> has not passed <gate>`; `<task> stands at <status>: <note>`; `<task> awaits review at <gate>`; `<task> is proposed`; `<task> waits on <url> at <hash7>` | The cause |
| | When `not_due_count` is not zero: a blank line, then `Next: <count requirement(s)> not yet due, <m> unmet.` | — |
| detail | The cause line is a heading, with a blank line before every cause but the first. `Action: <action>` at 2/2 | The cause |
| | Each held task at 2/2, and what it holds two cells deeper: `<task> at <gate>: `, then `the cause holds the task itself.`, `the parent of <child>.` or `requires <id> at <gate>, <text>.`, then ` Also under cause <n>.` per number | The held task and its gate |
| | When `not_due` holds a task, in place of the glance sentence: a blank line; heading `Next: <count requirement(s)> not yet due`; per task at 2/2 `<task> stands at <status>; next <gate>.`; per requirement at 4/2 `<gate of to>: requires <task> at <gate of from>, <text>: <condition>.` | The task and its next gate; the required task and `from` |
| provenance | After each cause's tree: per fact `<name>. <value>` at 2/2; per command of `resolve` `<text> # <comment>` at 4/2. At the end, with commands: a blank line, `Commands`, the commands at 2/2 | The cause; — |

### The history

The item number is the line's index. The title's subject is the task of the request and its count `<count event(s)>` from `events`. The target of a line and of everything beneath it is the line's task with the gate of its status or of its review.

| Level | Lines |
|-------|-------|
| glance | No line: `No event in view.` |
| | Per line at 0/2: cells(the date, by, the kinds joined by `, ` with ` (proposal)` after them, `<task>` unless the task is the subject, what the line records). The last cell is the first of these the line holds: `<status>`; the `<gate>` of a review; the pin; `the task file appears` for a task event |
| detail | The line is a heading. Beneath it at 2/2, each only when the data holds it: `Status after: <status>`, with `, unchanged`; `Note: <note of the status>`; `Effect: ` and the words of the key, or the key itself when the table lacks it; `<subject task> after: <status>`; `Model: `, by the reading; `Committer: <email>`; `Subproject events: <count event(s)> in <count commit(s)> of <url>`, then per line of the subproject at 4/2 cells(date, by, kinds, `<task>`, last cell, model) |
| provenance | Beneath those, with a commit, at 2/2: `Commit: <full hash>`; `Subject: `; `Author: <name> <<email>>, <author_time>`; `Committer: ` likewise; `Trailers: ` joined by `, `; `Files: <path> (<change>)` joined by `, `. Then the line's commands at 4/2. At the end, with commands: a blank line, `Commands`, the commands at 2/2 |

The effect keys read as tabloio's design `9167` words them: `authorised-commit`, `an authority commits the task file on the trunk, so the task is authorised`; `authorised-merge`, `an authority merges the change, so the task is authorised`; `authorised-trailer`, `an authority's trailer authorises the task`; `proposed`, `no authority accepts the change, so the task stands proposed`; `handoff`, `the contributor hands the work to the reviewer`; `accepts`, `the reviewer accepts the work at the gate`; `stands`, `none; the authorisation stands as the review`; `none`, `none`.

The model reads, by `reading`: `agrees`, `<model>; the <gate> junction states <stated>`; `differs`, the same and `, which differs`; `missing`, `no trailer; the <gate> junction states <stated>`; `exempt`, `none recorded; exempt`; any other reading, the model alone, and no line when it is empty.
