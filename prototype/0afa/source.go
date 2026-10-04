package main

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

// View names a view of VIEWS.md as tablo emits it.
type View string

// The views the prototype loads.
const (
	ViewGlobal     View = "global-tableau"
	ViewDefinition View = "task-definition"
	ViewQueue      View = "contributor-work-queue"
	ViewBlockage   View = "work-blockage-tree"
	ViewHistory    View = "history"
)

// Params are the focusing parameters of VIEWS.md that the panes pass.
// Task focuses the definition, blockage and history views; Person focuses
// the queue; Ref is the commit in view (empty means HEAD).
type Params struct {
	Task   string
	Person string
	Ref    string
}

// ErrNoData means the source holds nothing for the view and parameters.
var ErrNoData = errors.New("no data")

// Source is the seam where tablo's library call goes. Load returns the JSON
// of one view for the parameters, or an error that wraps ErrNoData.
type Source interface {
	Load(v View, p Params) ([]byte, error)
}

// FSSource reads one view-data file per view and per task or person from a
// file system, the stand-in for tablo.
type FSSource struct {
	FS fs.FS
}

func (s FSSource) name(v View, p Params) string {
	switch v {
	case ViewGlobal:
		return "global-tableau.json"
	case ViewDefinition:
		return "task-definition-" + p.Task + ".json"
	case ViewBlockage:
		return "blockage-" + p.Task + ".json"
	case ViewHistory:
		return "history-" + p.Task + ".json"
	case ViewQueue:
		who, _, _ := strings.Cut(p.Person, "@")
		return "queue-" + who + ".json"
	}
	return string(v) + ".json"
}

// Load implements Source.
func (s FSSource) Load(v View, p Params) ([]byte, error) {
	name := s.name(v, p)
	b, err := fs.ReadFile(s.FS, name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNoData, name)
	}
	return b, err
}
