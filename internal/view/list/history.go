package list

// History is the history view: tablo's view.History, what each commit does
// to each task in view, oldest first. The counts cover the project's own
// lines, and they are the same at every level. The day counts are not a
// pane's to draw (0afa, Conformance).
type History struct {
	Head
	Subject  TaskRef     `json:"subject"`  // the task whose subtree is in view: the task parameter, the root by default
	Events   int         `json:"events"`   // the events the project's own lines hold
	Commits  int         `json:"commits"`  // the distinct commits of those lines
	Kinds    []KindCount `json:"kinds"`    // the six kinds, in SYNTAX.md's order, zeros included
	Lines    []Line      `json:"lines"`    // oldest first; a subproject's lines from detail on
	Commands []Command   `json:"commands"` // provenance: the two git log commands over the subtree and the range
}

// Line is what one commit does to one task: its events, each the shared
// Event (8ed1 decision 1). The line's kinds are its events', and its date,
// author and commit the first event's. A line taken from a subproject marks
// each event with the subproject (8ed1 decision 2).
type Line struct {
	Task     TaskRef `json:"task"`     // the task of the project in view
	Proposal bool    `json:"proposal"` // off the trunk: the trunk's tip does not reach the commit
	Events   []Event `json:"events"`   // never empty; in the order of the kinds

	After        *Status     `json:"after"`         // detail from here: the task's status as the files stand at the commit
	Unchanged    bool        `json:"unchanged"`     // the files at one of the commit's parents state the same status
	SubjectAfter *Status     `json:"subject_after"` // the subject's status, on a line of another task; else nil
	Effect       string      `json:"effect"`        // what the line does to the task's acceptance or review; "" for nothing
	Model        string      `json:"model"`         // the commit's first Model: trailer
	Check        *ModelCheck `json:"check"`         // the model check of the line's status or reaffirmed event
	Committer    string      `json:"committer"`     // the committer's email where it differs from the author's

	Commit *Commit `json:"commit"` // provenance: the commit, of the project or of the subproject the line comes from
}

// First is the line's first event, which gives its date, its author and its
// commit; a line with no event gives the zero event.
func (l Line) First() Event {
	if len(l.Events) == 0 {
		return Event{}
	}
	return l.Events[0]
}

// Taken is the subproject the line comes from, or nil for the project's own.
func (l Line) Taken() *SubEvent { return l.First().Subproject }
