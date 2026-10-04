package main

import (
	"sort"
	"strings"
)

// Role is the part the person plays at the keyboard.
type Role string

// The roles the starting state distinguishes.
const (
	Owner       Role = "owner"
	Contributor Role = "contributor"
	Observer    Role = "observer"
)

// Next is a gate a person contributes at next.
type Next struct{ Task, Gate string }

// Person is what the views say about one email.
type Person struct {
	Email       string
	Owner       bool
	Known       bool
	Assigned    []string
	ContribNext []Next
}

// Roles lists the roles the person holds, from the view data: the owner
// holds owner, and contributor too with assigned or contributed tasks; a
// contributor holds contributor; a person who appears nowhere (or has no
// task) holds observer alone.
func (p Person) Roles() []Role {
	hasTasks := len(p.Assigned) > 0 || len(p.ContribNext) > 0
	switch {
	case p.Owner && hasTasks:
		return []Role{Owner, Contributor}
	case p.Owner:
		return []Role{Owner}
	case hasTasks:
		return []Role{Contributor}
	}
	return []Role{Observer}
}

// Role is the role the person starts in: the owner, else a contributor with
// tasks, else an observer.
func (p Person) Role() Role {
	switch {
	case p.Owner:
		return Owner
	case len(p.Assigned) > 0 || len(p.ContribNext) > 0:
		return Contributor
	}
	return Observer
}

// Classify reads the person's facts from the authority delegation (the
// root's assignee) and the task assignment (assigned tasks, next junctions).
func Classify(v Views, email string) Person {
	p := Person{Email: email}
	for _, r := range v.Authority.Glance.Rows {
		if r.Depth == 0 && strings.EqualFold(r.Assignee, email) {
			p.Owner = true
		}
	}
	p.Known = p.Owner
	for _, e := range v.Assignment.Detail.People {
		if !strings.EqualFold(e.Email, email) {
			continue
		}
		p.Known = true
		for _, a := range e.Assigned {
			p.Assigned = append(p.Assigned, a.ID)
		}
		for _, c := range e.Contributes {
			if c.Next {
				p.ContribNext = append(p.ContribNext, Next{c.Task, c.Gate})
			}
		}
	}
	return p
}

// Start is a starting state: which parents are expanded and the gate window.
type Start struct {
	Mode     Role
	Expanded map[string]bool
	First    string
	Last     string
}

// parents maps each task id to its parent id, from depth and row order.
func parents(g Global) map[string]string {
	out := map[string]string{}
	var stack []string
	for _, r := range g.Rows {
		if r.Depth < len(stack) {
			stack = stack[:r.Depth]
		}
		if r.Depth > 0 && len(stack) > 0 {
			out[r.ID] = stack[len(stack)-1]
		}
		stack = append(stack, r.ID)
	}
	return out
}

func gateIndex(v Views) map[string]int {
	idx := map[string]int{}
	for i, g := range v.Gates.Glance.Gates {
		idx[g.Key] = i
	}
	return idx
}

// window spans one column either side of the given gates, clipped to the
// gate list; it spans every gate when none is given.
func window(v Views, gates []string) (string, string) {
	all := v.Gates.Glance.Gates
	if len(all) == 0 {
		return "", ""
	}
	idx := gateIndex(v)
	lo, hi := len(all), -1
	for _, g := range gates {
		i, ok := idx[g]
		if !ok {
			continue
		}
		lo, hi = min(lo, i), max(hi, i)
	}
	if hi < 0 {
		return all[0].Key, all[len(all)-1].Key
	}
	lo, hi = max(lo-1, 0), min(hi+1, len(all)-1)
	return all[lo].Key, all[hi].Key
}

// OwnerStart is the owner's state: the tree collapsed to the root, and the
// window around the next gates of every task, since the owner views the whole
// project.
func OwnerStart(v Views) Start {
	var gates []string
	for _, r := range v.Global.Rows {
		if !r.Parent && r.NextGate != "" {
			gates = append(gates, r.NextGate)
		}
	}
	first, last := window(v, gates)
	return Start{Mode: Owner, Expanded: map[string]bool{}, First: first, Last: last}
}

// ContributorStart expands the tree to the ancestors of the person's assigned
// tasks and of the tasks they contribute at next, and windows the next gates
// of those tasks that do work (leaves). A person with no task gets the
// owner's state under the observer label.
func ContributorStart(v Views, p Person) Start {
	tasks := append([]string{}, p.Assigned...)
	for _, n := range p.ContribNext {
		tasks = append(tasks, n.Task)
	}
	if len(tasks) == 0 {
		s := OwnerStart(v)
		s.Mode = Observer
		return s
	}
	par := parents(v.Global)
	rows := map[string]Row{}
	for _, r := range v.Global.Rows {
		rows[r.ID] = r
	}
	expanded := map[string]bool{}
	var gates []string
	for _, id := range tasks {
		for a, ok := par[id]; ok; a, ok = par[a] {
			expanded[a] = true
		}
		if r, ok := rows[id]; ok && !r.Parent && r.NextGate != "" {
			gates = append(gates, r.NextGate)
		}
	}
	for _, n := range p.ContribNext {
		gates = append(gates, n.Gate)
	}
	sort.Strings(gates)
	first, last := window(v, gates)
	return Start{Mode: Contributor, Expanded: expanded, First: first, Last: last}
}

// StartFor derives the state for a role the person holds. The observer's
// layout is the owner's, under the observer label.
func StartFor(v Views, p Person, role Role) Start {
	switch role {
	case Owner:
		return OwnerStart(v)
	case Contributor:
		return ContributorStart(v, p)
	}
	s := OwnerStart(v)
	s.Mode = Observer
	return s
}
