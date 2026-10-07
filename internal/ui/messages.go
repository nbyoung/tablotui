package ui

// LoadedMsg carries the result of one Context.Load to the pane it names. A
// pane drops a Seq that is not its newest. This message carries fresh view
// data into a pane.
type LoadedMsg struct {
	Pane  string
	Seq   uint64
	Value any
	Err   error
}

// ReloadMsg tells every pane that the data behind its view changed. The frame
// sends it after it stores the snapshot that the watcher delivers (171b), and a
// command sends it after a write (518e). Each pane answers by deriving its
// current request again.
type ReloadMsg struct{}

// SelectionMsg names the task, and the gate when the cursor stands on a gate
// cell, that the home pane selects. The frame keeps the latest for
// Command.Run and broadcasts it, so panes follow. Gate is empty off a gate cell.
type SelectionMsg struct{ Task, Gate string }

// NoticeMsg shows Text in the status line until the next key. An error reads
// "error: <text>".
type NoticeMsg struct {
	Text string
	Err  bool
}

// OpenModeMsg opens Mode over the body.
type OpenModeMsg struct{ Mode Mode }

// ShowPaneMsg opens or closes a pane. Closing the focused pane focuses the
// home pane.
type ShowPaneMsg struct {
	Pane string
	Show bool
}

// FocusMsg focuses a pane, and opens it when it is closed.
type FocusMsg struct{ Pane string }

// GotoMsg asks the home pane to select a task, and a gate column when Gate
// names one. A detail pane sends it for the task its cursor line names; the
// home pane answers with a SelectionMsg, or a notice when it lacks the task.
type GotoMsg struct{ Task, Gate string }

// ReadingMsg says whom the panes read for and as which role. The role pane
// sends it at every arrival, the frame broadcasts it, and each pane that asks
// for a list view states the two on its requests.
type ReadingMsg struct{ Viewer, Role string }
