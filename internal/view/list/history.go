package list

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
