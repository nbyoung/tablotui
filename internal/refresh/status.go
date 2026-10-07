package refresh

import (
	"strings"
	"time"

	"github.com/nbyoung/tablotui/internal/watch"
)

// timeLayout prints a time as the status lines show it.
const timeLayout = "15:04:05"

// Status is what the frame says about the source and the last load.
type Status struct {
	Source  watch.Source
	State   watch.State // the state of the last good load
	Good    bool        // a load has succeeded
	Loaded  time.Time   // when the last good load finished
	Loading bool        // a load runs now
	Err     error       // the latest load's failure; nil after a good load
	Failed  time.Time   // when the present run of failures began
}

// Where names the source in one phrase, in the mockups' form "main at 3cdae52".
func (s Status) Where() string {
	if !s.Good {
		if s.Source.Live() {
			return "working tree"
		}
		return s.Source.Ref
	}
	at := short(s.State.Commit)
	switch {
	case !s.Source.Live():
		return s.Source.Ref + " at " + at
	case s.State.Branch == "":
		return "working tree at " + at + ", detached"
	default:
		return "working tree on " + s.State.Branch + " at " + at
	}
}

// Notice is the line the frame adds while there is no view or a stale one; it
// is empty when the view is current.
func (s Status) Notice() string {
	switch {
	case !s.Good && s.Err == nil:
		return "Loading"
	case !s.Good:
		return "No view yet: " + firstLine(s.Err.Error())
	case s.Err != nil:
		return "Reload failed at " + s.Failed.Local().Format(timeLayout) +
			", showing the view of " + s.Loaded.Local().Format(timeLayout) +
			": " + firstLine(s.Err.Error())
	default:
		return ""
	}
}

// short returns the first seven digits of a commit id.
func short(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// firstLine returns msg up to its first line break.
func firstLine(msg string) string {
	first, _, _ := strings.Cut(msg, "\n")
	return first
}
