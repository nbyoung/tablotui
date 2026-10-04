package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Content is what a pane shows: logical lines, wrapped to the pane at render.
type Content []string

func noData(view View, p Params, err error) Content {
	if errors.Is(err, ErrNoData) {
		what := p.Task
		if view == ViewQueue {
			what = p.Person
		}
		return Content{
			"No data.",
			"",
			fmt.Sprintf("The source holds no %s view for %s.", view, what),
		}
	}
	return Content{"Error: " + err.Error()}
}

func padRight(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

type junction struct {
	Contributor string   `json:"contributor"`
	Gate        string   `json:"gate"`
	Kind        string   `json:"kind"`
	Marks       []string `json:"marks"`
	Model       string   `json:"model"`
	Reviewer    string   `json:"reviewer"`
	Symbol      string   `json:"symbol"`
	Subproject  *struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	} `json:"subproject"`
}

type requirement struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	From      string `json:"from"`
	To        string `json:"to"`
	Text      string `json:"text"`
	Met       bool   `json:"met"`
	Due       bool   `json:"due"`
	Condition string `json:"condition"`
}

type definition struct {
	Glance struct {
		Assignee string `json:"assignee"`
		ID       string `json:"id"`
		Title    string `json:"title"`
		Parent   *struct {
			ID    string `json:"id"`
			Order int    `json:"order"`
			Title string `json:"title"`
		} `json:"parent"`
		Status struct {
			Gate     string `json:"gate"`
			State    string `json:"state"`
			Reason   string `json:"reason"`
			Note     string `json:"note"`
			Date     string `json:"date"`
			Recorder string `json:"recorder"`
		} `json:"status"`
	} `json:"glance"`
	Detail struct {
		Authorisation struct {
			State string `json:"state"`
			Way   string `json:"way"`
		} `json:"authorisation"`
		Authorities []string      `json:"authorities"`
		Children    []string      `json:"children"`
		Dependents  []requirement `json:"dependents"`
		Requires    []requirement `json:"requires"`
		Description string        `json:"description"`
		Junctions   []junction    `json:"junctions"`
		References  []struct {
			URL  string `json:"url"`
			Text string `json:"text"`
		} `json:"references"`
	} `json:"detail"`
}

func requirementLine(r requirement) string {
	cond := r.Condition
	if r.Due {
		cond += ", due"
	}
	return fmt.Sprintf("  %s %s, %s to %s, %s: %s", r.ID, r.Title, r.From, r.To, cond, r.Text)
}

func definitionContent(b []byte) (Content, error) {
	var d definition
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	g, det := d.Glance, d.Detail
	symbol := map[string]string{}
	for _, j := range det.Junctions {
		symbol[j.Gate] = j.Symbol
	}
	c := Content{g.ID + " " + g.Title, "assignee " + g.Assignee}
	if g.Parent != nil {
		c = append(c, fmt.Sprintf("parent   %s %s, order %d", g.Parent.ID, g.Parent.Title, g.Parent.Order))
	}
	st := g.Status
	line := fmt.Sprintf("status   %s %s, %s", symbol[st.Gate], st.Gate, st.State)
	if st.Reason != "" {
		line += ", " + st.Reason
	}
	c = append(c, line+", "+st.Date)
	if st.Note != "" {
		c = append(c, "note     "+st.Note)
	}
	c = append(c, fmt.Sprintf("authorised %s (%s)", det.Authorisation.State, det.Authorisation.Way),
		"authorities "+strings.Join(det.Authorities, ", "), "", det.Description)
	if len(det.Requires) > 0 {
		c = append(c, "", "requires")
		for _, r := range det.Requires {
			c = append(c, requirementLine(r))
		}
	}
	if len(det.Dependents) > 0 {
		c = append(c, "", "dependents")
		for _, r := range det.Dependents {
			c = append(c, requirementLine(r))
		}
	}
	if len(det.References) > 0 {
		c = append(c, "", "references")
		for _, r := range det.References {
			c = append(c, "  "+r.Text+" "+r.URL)
		}
	}
	c = append(c, "", "junctions")
	for _, j := range det.Junctions {
		who := j.Contributor
		if j.Model != "" {
			who = j.Model
		}
		if j.Subproject != nil {
			who = "subproject " + j.Subproject.URL
		}
		if j.Reviewer != "" {
			who += ", reviewer " + j.Reviewer
		}
		c = append(c, fmt.Sprintf("  %s %s %s", padRight(j.Symbol, 2), padRight(j.Gate, 14), strings.Join(j.Marks, "")+" "+who))
	}
	return c, nil
}

type blockage struct {
	Causes []struct {
		Cause    string `json:"cause"`
		Resolver string `json:"resolver"`
		Action   string `json:"action"`
		Holds    int    `json:"holds"`
		Tree     []struct {
			Task  string `json:"task"`
			Title string `json:"title"`
			Gate  string `json:"gate"`
		} `json:"tree"`
	} `json:"causes"`
	NotYetDue []requirement2 `json:"not_yet_due"`
}

type requirement2 struct {
	Task     string `json:"task"`
	Requires string `json:"requires"`
	From     string `json:"from"`
	To       string `json:"to"`
	Text     string `json:"text"`
}

func blockageContent(b []byte, task string) (Content, error) {
	var d blockage
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	if len(d.Causes) == 0 {
		return Content{"Nothing blocks " + task + " and it holds nothing up."}, nil
	}
	var c Content
	for i, k := range d.Causes {
		if i > 0 {
			c = append(c, "")
		}
		c = append(c, k.Cause,
			fmt.Sprintf("  %s resolves, holds %d", k.Resolver, k.Holds),
			"  action: "+k.Action)
		for _, t := range k.Tree {
			c = append(c, fmt.Sprintf("  - %s %s at %s", t.Task, t.Title, t.Gate))
		}
	}
	for i, n := range d.NotYetDue {
		if i == 0 {
			c = append(c, "", "not yet due")
		}
		c = append(c, fmt.Sprintf("  %s requires %s, %s to %s: %s", n.Task, n.Requires, n.From, n.To, n.Text))
	}
	return c, nil
}

type event struct {
	Date   string `json:"date"`
	Commit string `json:"commit"`
	By     string `json:"by"`
	Event  string `json:"event"`
	Gate   string `json:"gate"`
	State  string `json:"state"`
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

// historyContent lists the newest event first.
func historyContent(b []byte) (Content, error) {
	var ev []event
	if err := json.Unmarshal(b, &ev); err != nil {
		return nil, err
	}
	if len(ev) == 0 {
		return Content{"No events."}, nil
	}
	var c Content
	for i := len(ev) - 1; i >= 0; i-- {
		e := ev[i]
		what := e.Event
		for _, s := range []string{e.Gate, e.State, e.Reason} {
			if s != "" {
				what += " " + s
			}
		}
		c = append(c, fmt.Sprintf("%s %s %s", e.Date, e.Commit, what), "  by "+e.By)
		if e.Note != "" {
			c = append(c, "  "+e.Note)
		}
	}
	return c, nil
}

type queue struct {
	Person string `json:"person"`
	Items  []struct {
		Kind  string `json:"kind"`
		Task  string `json:"task"`
		Title string `json:"title"`
		Gate  string `json:"gate"`
		Cause string `json:"cause"`
		Date  string `json:"status_date"`
	} `json:"items"`
}

// queueContent marks the items that name the selected task.
func queueContent(b []byte, person, selected string) (Content, error) {
	var d queue
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	if len(d.Items) == 0 {
		return Content{"The queue of " + person + " is empty."}, nil
	}
	var c Content
	for _, it := range d.Items {
		mark := "  "
		if it.Task == selected {
			mark = "▸ "
		}
		line := mark + it.Kind + ": " + it.Task + " " + it.Title
		if it.Gate != "" {
			line += " at " + it.Gate
		}
		c = append(c, line)
		if it.Cause != "" {
			c = append(c, "    "+it.Cause)
		}
	}
	return c, nil
}

type row struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Depth int    `json:"depth"`
}

func globalRows(b []byte) ([]row, error) {
	var d struct {
		Rows []row `json:"rows"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return d.Rows, nil
}
