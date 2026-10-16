package list

// Queue is the contributor work queue of one person: tablo's view.Queue.
// The brief its provenance holds is not a pane's to draw (0afa decision 8).
type Queue struct {
	Head
	Items []QueueItem `json:"items"` // kind order, then display order; reaffirmations oldest first
}

// QueueItem is one thing the person does: tablo's view.QueueItem with its
// detail flattened in.
type QueueItem struct {
	Kind   string  `json:"kind"` // review, authorisation, ready, reaffirmation or waiting
	Task   TaskRef `json:"task"`
	Gate   string  `json:"gate"`   // the next gate; an authorisation, a reaffirmation: the gate the status names
	Model  string  `json:"model"`  // review, ready, waiting: the model the junction states
	Since  string  `json:"since"`  // the date of the status; "" when undetermined
	Age    int     `json:"age"`    // reaffirmation: days from Since to the date of the source commit
	Causes []Wait  `json:"causes"` // waiting: why, in the order requirement, reason, authorisation, review

	Junction       *Junction      `json:"junction"` // detail from here: the next junction, with From; nil for a parent and at the last gate
	Sources        []Supplier     `json:"sources"`  // of that junction, by field
	TaskReferences []Reference    `json:"task_references"`
	Requires       []Requirement  `json:"requires"`         // in file order
	Unblocks       []Requirement  `json:"unblocks"`         // the dependents that passing Gate meets
	AlsoRequiredBy []Requirement  `json:"also_required_by"` // the other dependents
	ParentUnblocks []ParentEdge   `json:"parent_unblocks"`
	Status         *Status        `json:"status"`
	StatusCommit   *Commit        `json:"status_commit"` // the deciding commit of the status; nil without one
	Authorisation  *Authorisation `json:"authorisation"` // an authorisation: the task's; else nil
}

// Wait is one reason a work item waits.
type Wait struct {
	Kind     string       `json:"kind"`     // requirement, reason, authorisation or review
	Requires *Requirement `json:"requires"` // requirement: the unmet entry
	Reason   string       `json:"reason"`   // reason: the status's reason, other than review
	By       string       `json:"by"`       // authorisation: the nearest authority; review: the reviewer
}

// ParentEdge is a requirement on an item's parent that the item's gate
// would let pass.
type ParentEdge struct {
	Parent   TaskRef     `json:"parent"`
	Requires Requirement `json:"requires"` // Task is the dependent; From is the parent's gate
}
