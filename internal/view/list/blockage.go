package list

// Blockage is the work-blockage tree: tablo's view.Blockage with its detail
// and its provenance flattened in.
type Blockage struct {
	Head
	Causes      []Cause     `json:"causes"`        // largest first
	Kinds       []KindCount `json:"kinds"`         // the five kinds, in VIEWS.md's order
	NotDueN     int         `json:"not_due_count"` // the requirements not yet due
	NotDueUnmet int         `json:"not_due_unmet"` // those of them not met
	NotDue      []Waiting   `json:"not_due"`       // detail: by task, in display order
	Commands    []Command   `json:"commands"`      // provenance
}

// KindCount counts the items of one kind.
type KindCount struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// Cause is one root of the tree.
type Cause struct {
	Kind     string  `json:"kind"` // requirement, status, review, authorisation or snapshot
	Task     TaskRef `json:"task"` // the task the cause stands at
	URL      string  `json:"url"`  // a cross-project requirement: the project of Task; a snapshot: the link
	Gate     string  `json:"gate"` // requirement: the gate not passed; review: the gate under review; else the gate the status names
	Status   Status  `json:"status"`
	Pin      string  `json:"pin"`      // snapshot: the commit the link reads, in full
	Tip      string  `json:"tip"`      // snapshot: the tip of the subproject's trunk, in full
	Resolver string  `json:"resolver"` // the email of the person who acts; "" where the files name none
	Mark     string  `json:"mark"`     // the resolver's mark key, or ""
	Act      string  `json:"act"`      // contributes, records, reviews, authorises or advances
	Advance  string  `json:"advance"`  // snapshot: what advances, pin or gate
	Holds    int     `json:"holds"`    // the distinct tasks beneath, at every depth
	Held     []Held  `json:"held"`     // detail

	StatusCommit  *Commit        `json:"status_commit"` // provenance from here: the deciding commit of Status
	Junction      *Junction      `json:"junction"`      // the next junction of Task, with From
	Authorisation *Authorisation `json:"authorisation"` // authorisation: the task's
	Linkage       *Linkage       `json:"linkage"`       // snapshot: the link
	Resolve       []Command      `json:"resolve"`       // review, authorisation: the commit that resolves
}

// Held is a task a cause holds, and what it holds in turn.
type Held struct {
	Task     TaskRef      `json:"task"`
	Gate     string       `json:"gate"` // the gate it waits to pass
	Parent   bool         `json:"parent"`
	Via      string       `json:"via"`      // self, parent or requirement
	Child    string       `json:"child"`    // parent: the child that holds it
	Requires *Requirement `json:"requires"` // requirement: the entry
	Also     []int        `json:"also"`     // the other causes it stands under, from 1
	Held     []Held       `json:"held"`
}

// Waiting is a task with requirements not yet due.
type Waiting struct {
	Task     TaskRef       `json:"task"`
	Status   Status        `json:"status"`
	Next     string        `json:"next"` // its next gate; "" at the last
	Requires []Requirement `json:"requires"`
}
