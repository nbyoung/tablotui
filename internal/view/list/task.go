package list

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
