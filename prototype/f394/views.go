package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
)

//go:embed testdata/*.json
var testdata embed.FS

// Global is the part of the global tableau view the starting state reads.
type Global struct {
	Columns []struct {
		Gate   string `json:"gate"`
		Symbol string `json:"symbol"`
	} `json:"columns"`
	NextGatesInView []string `json:"next_gates_in_view"`
	Rows            []Row    `json:"rows"`
}

// Row is one task row of the global tableau. The row carries no parent id and
// no assignee: the parent follows from depth and order.
type Row struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Depth    int    `json:"depth"`
	Parent   bool   `json:"parent"`
	NextGate string `json:"next_gate"`
	Status   struct {
		Gate  string `json:"gate"`
		State string `json:"state"`
	} `json:"status"`
}

// Authority is the authority delegation view; its glance rows carry the
// assignee of every task.
type Authority struct {
	Glance struct {
		Rows []struct {
			ID       string `json:"id"`
			Assignee string `json:"assignee"`
			Depth    int    `json:"depth"`
			Children int    `json:"children"`
		} `json:"rows"`
	} `json:"glance"`
}

// Assignment is the task assignment view at detail: one entry per email that
// appears in the project.
type Assignment struct {
	Detail struct {
		People []struct {
			Email    string `json:"email"`
			Assigned []struct {
				ID string `json:"id"`
			} `json:"assigned"`
			Contributes []struct {
				Task string `json:"task"`
				Gate string `json:"gate"`
				Next bool   `json:"next"`
			} `json:"contributes"`
		} `json:"people"`
	} `json:"detail"`
}

// GateDef is the gate definition view, which orders the gates.
type GateDef struct {
	Glance struct {
		Gates []struct {
			Key string `json:"key"`
		} `json:"gates"`
	} `json:"glance"`
}

// Contextual is the contextual tableau for one person.
type Contextual struct {
	Person string `json:"person"`
	Rows   []struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	} `json:"rows"`
}

// Queue is the work queue for one person.
type Queue struct {
	Person string `json:"person"`
	Items  []struct {
		Kind string `json:"kind"`
		Task string `json:"task"`
		Gate string `json:"gate"`
	} `json:"items"`
}

// Views holds the view data the starting state derives from.
type Views struct {
	Global     Global
	Authority  Authority
	Assignment Assignment
	Gates      GateDef
}

func load(fsys fs.FS, name string, v any) error {
	b, err := fs.ReadFile(fsys, "testdata/"+name)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// LoadViews reads the four views from the embedded copies of tablo's output.
func LoadViews(fsys fs.FS) (Views, error) {
	var v Views
	for name, dst := range map[string]any{
		"global-tableau.json":       &v.Global,
		"authority-delegation.json": &v.Authority,
		"task-assignment.json":      &v.Assignment,
		"gate-definition.json":      &v.Gates,
	} {
		if err := load(fsys, name, dst); err != nil {
			return v, err
		}
	}
	return v, nil
}
