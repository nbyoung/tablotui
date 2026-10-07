// Package source names where the tableau grid gets its view data. The grid
// asks through the Tableaux interface and never learns how the data arrives:
// this build serves fixture files, and the tablo adapter joins at the
// integrate gate.
package source

import (
	"context"

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
}

// Tableaux loads a tableau. A call blocks; the frame runs it in a command.
type Tableaux interface {
	Tableau(ctx context.Context, r Request) (view.Tableau, error)
}
