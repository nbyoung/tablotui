package list

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
