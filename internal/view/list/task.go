package list

// Task is the task definition view: tablo's view.Task with its detail and
// its provenance flattened in.
type Task struct {
	Head
	Task          TaskRef        `json:"task"`
	Assignee      string         `json:"assignee"`
	Parent        *Parent        `json:"parent"` // nil for the root
	Status        Status         `json:"status"`
	Authorised    bool           `json:"authorised"`
	Description   string         `json:"description"` // detail from here
	References    []Reference    `json:"references"`
	Path          []TaskRef      `json:"path"`     // the root first, the task last
	Siblings      int            `json:"siblings"` // children of the parent, the task included; 0 for the root
	Children      []ChildRow     `json:"children"` // in display order
	Authorities   []Authority    `json:"authorities"`
	Requires      []Requirement  `json:"requires"` // in file order
	Dependents    []Requirement  `json:"dependents"`
	Junctions     []Junction     `json:"junctions"`     // every gate after undefined, in order
	Snapshot      *Snapshot      `json:"snapshot"`      // a snapshot status: the task read; else nil
	StatusCommit  *Commit        `json:"status_commit"` // provenance from here: the deciding commit of the last status of the chain
	OwnCommit     *Commit        `json:"own_commit"`    // a snapshot: the deciding commit of the task's own file
	Authorisation *Authorisation `json:"authorisation"`
	Considered    []string       `json:"considered"` // a parent: the children its roll-up considers
	Chain         []Step         `json:"chain"`      // the statuses the status comes through, the task's own first
	Linkages      []Linkage      `json:"linkages"`   // one per subproject a recursive junction reads
	Sources       []Supplier     `json:"sources"`    // by gate, then field
	Reviews       []Review       `json:"reviews"`    // the reviewed junctions the status passes
	Models        []ModelCheck   `json:"models"`     // oldest first
	Events        []Event        `json:"events"`     // the newest five, newest first
	EventCount    int            `json:"event_count"`
	Commands      []Command      `json:"commands"`
}

// Parent is the parent of a task and the task's place under it.
type Parent struct {
	TaskRef
	Order *int `json:"order"` // nil when the file states none
}

// ChildRow is a child with its status.
type ChildRow struct {
	TaskRef
	Status Status `json:"status"`
}

// Authority is an authority of a task and the ancestor that makes it one.
type Authority struct {
	Email string `json:"email"`
	By    string `json:"by"` // the ancestor's id
}

// Requirement is one requires entry with its condition, seen from either end.
type Requirement struct {
	Task       TaskRef `json:"task"` // the other task
	From       string  `json:"from"`
	To         string  `json:"to"`
	Text       string  `json:"text"`
	Met        bool    `json:"met"`
	Due        bool    `json:"due"`
	Status     *Status `json:"status"`     // of the originating task; nil for a dependent
	Subproject string  `json:"subproject"` // a cross-project entry: its url
	Commit     string  `json:"commit"`     // and the commit it reads, in full
}

// Junction is a task's junction at one gate, resolved.
type Junction struct {
	Gate string `json:"gate"`
	Marked
	Contributor string      `json:"contributor"`
	Model       string      `json:"model"`
	Reviewer    string      `json:"reviewer"`
	ByDefault   bool        `json:"reviewer_by_default"` // the default supplies the reviewer, the assignee or the nearest person, since no entry states one
	Subproject  *SubRef     `json:"subproject"`          // a recursive junction; else nil
	References  []Reference `json:"references"`
	Stands      string      `json:"stands"` // passed, here, next, later or exempt; "" on a parent
	Reviewed    bool        `json:"reviewed"`
	From        []string    `json:"from"` // where the view shows suppliers: the tasks whose entries supply it, nearest first
}

// SubRef names the task of a subproject that a recursive junction reads.
type SubRef struct {
	URL  string  `json:"url"`
	Task TaskRef `json:"task"` // zero when undetermined
}

// Snapshot is a subproject's task as a recursive junction reads it.
type Snapshot struct {
	SubRef
	Assignee string     `json:"assignee"`
	Status   *Status    `json:"status"` // nil when the task is not read
	Children []ChildRow `json:"children"`
}

// Authorisation is a task's authorisation and the commit that decides it.
type Authorisation struct {
	Commit *Commit `json:"commit"` // the deciding commit; nil when none decides
	Way    string  `json:"way"`    // change, merge or trailer; "" with no commit
	By     string  `json:"by"`     // author, committer or both; "" for a proposed task
	Why    string  `json:"why"`    // proposed with no deciding commit: no_history, no_trunk, off_trunk or no_commit
}

// Step is one status of a chain.
type Step struct {
	Task   TaskRef `json:"task"`
	URL    string  `json:"url"` // the subproject the task is of; "" in the project in view
	Status Status  `json:"status"`
}

// Linkage is one subproject a recursive junction reads, and what fixes the
// commit it reads.
type Linkage struct {
	Gates        []string `json:"gates"` // the gates whose junctions read it, in order
	URL          string   `json:"url"`   // as the link resolves it
	ID           string   `json:"id"`    // the id the entry states; "" when it reads the root
	Task         TaskRef  `json:"task"`  // the task read there; zero when undetermined
	Form         string   `json:"form"`  // submodule, directory, url or none
	Commit       string   `json:"commit"`
	Why          string   `json:"why"`  // no_link or no_task; "" when the task is read
	File         string   `json:"file"` // where the url stands: .tableaux/tasks/<id>.yaml
	Line         int      `json:"line"` // from 1; 0 when unknown
	StatedCommit string   `json:"stated_commit"`
	Recorded     string   `json:"recorded"` // a submodule: the gitlink of the source commit
	Moved        bool     `json:"moved"`    // a submodule: the checkout stands elsewhere
	Trunk        string   `json:"trunk"`    // the subproject's trunk; "" when undetermined
	Tip          string   `json:"tip"`      // the tip of that trunk, in full
	OnTrunk      string   `json:"on_trunk"` // unknown, no or yes
	Behind       string   `json:"behind"`   // unknown, no or yes
	Pinned       *Commit  `json:"pinned"`   // the commit read
	SetBy        *Commit  `json:"set_by"`   // the commit of the newest pin event of the link
}

// Supplier says what supplies one field of one junction: a task id, or
// default, own or assignee.
type Supplier struct {
	Gate  string `json:"gate"`
	Field string `json:"field"` // contributor, model, reviewer, references, subproject or applies
	Value string `json:"value"`
	By    string `json:"by"`
}

// Review is a reviewed junction the status passes and what accepts it.
type Review struct {
	Gate     string  `json:"gate"`
	Reviewer string  `json:"reviewer"`
	Effect   string  `json:"effect"` // accepts, authorisation or none
	Commit   *Commit `json:"commit"` // nil while nothing accepts
}

// ModelCheck compares a junction's model with a commit's Model: trailer.
type ModelCheck struct {
	Commit   string `json:"commit"`
	Gate     string `json:"gate"`
	Stated   string `json:"stated"`
	Trailer  string `json:"trailer"`
	Reading  string `json:"reading"`   // agrees, differs, missing, exempt or unstated
	CoAuthor string `json:"co_author"` // the first agent a Co-Authored-By: trailer names on a person's commit; else ""
}

// Event is one event of a task: what one commit does to it of one kind.
type Event struct {
	Date       string       `json:"date"`
	Commit     string       `json:"commit"` // in full
	By         string       `json:"by"`     // the author's email
	Kind       string       `json:"kind"`   // task, authorised, status, reaffirmed, reviewed or pin
	Recorded   *EventStatus `json:"recorded"`
	Gate       string       `json:"gate"`       // reviewed
	Pin        *PinMove     `json:"pin"`        // pin
	Subproject *SubEvent    `json:"subproject"` // an event taken from a subproject
}

// EventStatus is what a status event records.
type EventStatus struct {
	Gate   string `json:"gate"`
	State  string `json:"state"`
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

// PinMove is a subproject pin that moves from one commit to another.
type PinMove struct {
	URL string `json:"url"`
	Old string `json:"old"` // "" when the linkage first appears
	New string `json:"new"`
}

// SubEvent names the subproject task an event comes from, by its id and its
// title.
type SubEvent struct {
	URL   string `json:"url"`
	Task  string `json:"task"`
	Title string `json:"title"` // as the subproject's files at the pin state it; "" where they lack the task
}
