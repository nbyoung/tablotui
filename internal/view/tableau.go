// Package view holds the tableau as the grid reads it, and its decoder. The
// types mirror the view data that tablo derives; a front end draws them and
// derives nothing from them.
package view

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Tableau is one tableau, global or contextual, as tablo derives it. The grid
// draws it and derives nothing from it. Viewer and Role are optional (f394).
type Tableau struct {
	View       string `json:"view"`    // "tableau" or "context"
	Project    string `json:"project"` // the root task's title
	Ref        string `json:"ref"`
	Commit     string `json:"commit"`
	Date       string `json:"date"` // of the commit
	Viewer     string `json:"viewer"`
	Role       string `json:"role"`
	Level      string `json:"level"`
	Window     int    `json:"window"`
	Historical bool   `json:"historical"`
	Person     string `json:"person"`
	Gates      []Gate `json:"gates"` // every gate of the project, in order
	Rows       []Row  `json:"rows"`  // every row in view, in display order
}

// Gate is one gate column. Window says whether the window shows it; Count is
// the number of tasks in view whose current gate it is.
type Gate struct {
	Key    string `json:"key"`
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	Window bool   `json:"window"`
	Count  int    `json:"count"`
}

// Row is one task. Label is "", "spine" or "sibling". Cells holds one cell
// per gate, in gate order, whatever the window.
type Row struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Depth  int    `json:"depth"`
	Parent bool   `json:"parent"`
	Label  string `json:"label"`
	Status Status `json:"status"`
	Cells  []Cell `json:"cells"`
}

// Status is the row's status, stated or rolled up. From is nil for a leaf
// that states its own status.
type Status struct {
	Gate   string `json:"gate"`
	State  string `json:"state"`
	Reason string `json:"reason"`
	Date   string `json:"date"`
	Note   string `json:"note"`
	From   *From  `json:"from"`
}

// From names where a derived status comes from. Kind is "rollup" (ID and
// Title of the deciding leaf) or "snapshot" (ID and Title of the subproject's
// root, the Pin, and the Gate it stands at there).
type From struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Title string `json:"title"`
	Pin   string `json:"pin"`
	Gate  string `json:"gate"`
}

// Cell is one gate cell. Kind is "status", "marks", "exempt" or "historical".
// Symbols is the text to draw, empty for a historical cell while the
// historical-junctions parameter is off. Acts marks a cell where the person
// acts. Facts is the provenance, present at the provenance level.
type Cell struct {
	Kind    string `json:"kind"`
	Symbols string `json:"symbols"`
	Acts    bool   `json:"acts"`
	Facts   []Fact `json:"facts"`
}

// Fact is one line of provenance: "Deciding commit", "3d8ce0e Record …".
type Fact struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// Decode reads one tableau and refuses a row whose cells do not match the
// gates one for one, a depth that rises by more than one, and an unknown kind.
func Decode(r io.Reader) (Tableau, error) {
	var t Tableau
	dec := json.NewDecoder(r)
	if err := dec.Decode(&t); err != nil {
		return Tableau{}, fmt.Errorf("tableau: %w", err)
	}
	if dec.More() {
		return Tableau{}, errors.New("tableau: data after the tableau")
	}
	prev := -1
	for i, row := range t.Rows {
		if len(row.Cells) != len(t.Gates) {
			return Tableau{}, fmt.Errorf("tableau: row %d (%s) has %d cells for %d gates", i+1, row.ID, len(row.Cells), len(t.Gates))
		}
		if row.Depth < 0 {
			return Tableau{}, fmt.Errorf("tableau: row %d (%s) has depth %d", i+1, row.ID, row.Depth)
		}
		if i > 0 && row.Depth > prev+1 {
			return Tableau{}, fmt.Errorf("tableau: row %d (%s) rises from depth %d to %d", i+1, row.ID, prev, row.Depth)
		}
		prev = row.Depth
		for j, c := range row.Cells {
			switch c.Kind {
			case "status", "marks", "exempt", "historical":
			default:
				return Tableau{}, fmt.Errorf("tableau: row %d (%s) cell %d has unknown kind %q", i+1, row.ID, j+1, c.Kind)
			}
		}
	}
	return t, nil
}
