package list

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
