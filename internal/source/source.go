// Package source names where the panes get their view data. The grid asks
// through the Tableaux interface, the detail panes through Lists, and the role
// pane through Viewers; none learns how the data arrives. Two sources serve
// them: File reads fixture files, and Tablo derives each view from the
// snapshot of tablo's library that the frame stores.
package source

import (
	"context"
	"fmt"

	"github.com/nbyoung/tablotui/internal/view"
)

// Request names one tableau and the parameters that focus it (VIEWS.md, Parameters).
type Request struct {
	View       string // "tableau" or "context"
	Ref        string // "" is the checkout; 171b sets it
	Task       string // the contextual tableau's task form; f394 sets it
	Person     string // the person form, or the person a global tableau marks
	Window     int
	Historical bool
	Level      string // "detail" or "provenance"
	Viewer     string // the email of the person at the keyboard; "" is nobody
	Role       string // the role the viewer reads as; "" is the viewer's own
}

// Tableaux loads a tableau. A call blocks; the frame runs it in a command.
// It returns a view and no error, an error and no view, or a view beside a
// *DiagnosticError: the project's error diagnostics leave the view standing
// (679b decision 16).
type Tableaux interface {
	Tableau(ctx context.Context, r Request) (view.Tableau, error)
}

// Diagnostic is one finding of tablo's Loader or Validator, as its envelope
// states it. Path starts with .tableaux/; Line and Col are 0 where they do
// not apply.
type Diagnostic struct {
	Severity, Code, Path string
	Line, Col            int
	Message              string
}

// String writes the diagnostic as "path:line:col: code message", with the
// parts that do not apply left out.
func (d Diagnostic) String() string {
	at := d.Path
	if at != "" && d.Line > 0 {
		at += fmt.Sprintf(":%d", d.Line)
		if d.Col > 0 {
			at += fmt.Sprintf(":%d", d.Col)
		}
	}
	s := d.Code + " " + d.Message
	if at != "" {
		s = at + ": " + s
	}
	return s
}

// DiagnosticError holds the error diagnostics of a project. A source returns
// it beside the view those errors leave standing, and the pane draws the view;
// the refresh shows the error on the notice line.
type DiagnosticError struct{ Diagnostics []Diagnostic }

// Beside marks the error as one that stands beside data: the refresh treats a
// load that returns it as good and carries it in its status.
func (*DiagnosticError) Beside() {}

// Error names the first diagnostic and counts the rest.
func (e *DiagnosticError) Error() string {
	if len(e.Diagnostics) == 0 {
		return "the project has errors"
	}
	s := e.Diagnostics[0].String()
	if n := len(e.Diagnostics) - 1; n > 0 {
		s += fmt.Sprintf(" (and %d more)", n)
	}
	return s
}
