package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Data is the global tableau view data that tablo prototype 886d emits. Only
// the fields the grid draws are declared.
type Data struct {
	View      string   `json:"view"`
	Ref       string   `json:"ref"`
	Level     string   `json:"level"`
	Window    int      `json:"window"`
	CellRule  string   `json:"cell_rule"`
	NextGates []string `json:"next_gates_in_view"`
	Columns   []Column `json:"columns"`
	Folded    []Fold   `json:"folded"`
	Rows      []Row    `json:"rows"`
}

// Column is one gate column inside the window.
type Column struct {
	Gate   string `json:"gate"`
	Symbol string `json:"symbol"`
}

// Fold is a run of gate columns outside the window, folded to a count.
type Fold struct {
	Side   string         `json:"side"`
	From   string         `json:"from"`
	To     string         `json:"to"`
	Count  int            `json:"count"`
	ByGate map[string]int `json:"by_gate"`
}

// Row is one task. Rows come flat in display order; depth gives the tree.
type Row struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Depth    int    `json:"depth"`
	Parent   bool   `json:"parent"`
	Status   Status `json:"status"`
	NextGate string `json:"next_gate"`
	Cells    []Cell `json:"cells"`
}

// Status is the row's current gate, state and the child a parent rolls up from.
type Status struct {
	Gate         string `json:"gate"`
	State        string `json:"state"`
	Reason       string `json:"reason"`
	Note         string `json:"note"`
	Date         string `json:"date"`
	RolledUpFrom string `json:"rolled_up_from"`
}

// Cell is one gate cell: kind is blank, status, marks or exempt.
type Cell struct {
	Gate    string `json:"gate"`
	Kind    string `json:"kind"`
	Symbols string `json:"symbols"`
}

// LoadData reads one view data file.
func LoadData(path string) (Data, error) {
	var d Data
	b, err := os.ReadFile(path)
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return d, fmt.Errorf("%s: %w", path, err)
	}
	if d.View != "global-tableau" {
		return d, fmt.Errorf("%s: view %q, want global-tableau", path, d.View)
	}
	return d, nil
}
