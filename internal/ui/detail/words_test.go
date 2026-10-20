package detail

import (
	"slices"
	"strings"
	"testing"

	"github.com/nbyoung/tablotui/internal/view/list"
)

// texts returns the text of each line.
func texts(lines []Line) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.Text
	}
	return out
}

// TestModelCoAuthor: a model check names the agent a person's commit
// co-authors after its reading, as tabloio's task view does (cdb0 decision
// 3), and one with no co-author reads as before.
func TestModelCoAuthor(t *testing.T) {
	d := list.Task{
		Task: list.TaskRef{ID: "e9c6", Title: "Abstract views"},
		Models: []list.ModelCheck{
			{Commit: "abcdef1234", Gate: "design", Stated: "m", Reading: "missing", CoAuthor: "bot@example.org"},
			{Commit: "1234abcdef", Gate: "design", Stated: "m", Trailer: "m", Reading: "agrees"},
		},
	}
	_, _, lines := taskLines(d, list.Provenance, false)
	got := texts(lines)
	for _, want := range []string{
		"abcdef1 · design · states m · No trailer; co-author bot@example.org",
		"1234abc · design · states m · trailer m · Agrees",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("the models lack %q:\n%s", want, strings.Join(got, "\n"))
		}
	}
}

// TestHistoryCoAuthor: a history line at detail names the agent a person's
// commit co-authors after the model and before the committer, as tabloio's
// history does (cdb0 decision 3), and a line with no co-author names none.
func TestHistoryCoAuthor(t *testing.T) {
	line := func(coAuthor string) list.Line {
		return list.Line{
			Task:      list.TaskRef{ID: "e9c6", Title: "Abstract views"},
			Events:    []list.Event{{Date: "2026-09-01", Commit: "abcdef1234", By: "ada@example.org", Kind: "status"}},
			Model:     "m",
			CoAuthor:  coAuthor,
			Committer: "c@example.org",
		}
	}
	d := list.History{Subject: list.TaskRef{ID: "e9c6"}, Lines: []list.Line{line("bot@example.org")}}
	_, _, lines := historyLines(d, list.Detail)
	got := strings.Join(texts(lines), "\n")
	if want := "\nModel: m\nCo-author: bot@example.org\nCommitter: c@example.org"; !strings.Contains(got, want) {
		t.Errorf("the line lacks %q:\n%s", want, got)
	}
	d.Lines = []list.Line{line("")}
	_, _, lines = historyLines(d, list.Detail)
	got = strings.Join(texts(lines), "\n")
	if strings.Contains(got, "Co-author:") || !strings.Contains(got, "\nModel: m\nCommitter: c@example.org") {
		t.Errorf("a line with no co-author draws one:\n%s", got)
	}
}

// TestDefaultReviewer: where the default supplies a reviewer, each pane says
// who supplies it, in tabloio's words (cdb0 decision 2): the task pane by
// whether the reviewer is the task's assignee, the queue by the reviewer's
// supplier, and the blockage, which cannot tell, "by default". A waived
// reviewer is "", and no pane draws it.
func TestDefaultReviewer(t *testing.T) {
	d := list.Task{
		Task:     list.TaskRef{ID: "b2c9", Title: "Leaf"},
		Assignee: "ada@example.org",
		Junctions: []list.Junction{
			{Gate: "design", Contributor: "ada@example.org", Reviewer: "ada@example.org", ByDefault: true},
			{Gate: "implementation", Contributor: "bot@example.org", Reviewer: "olive@example.org", ByDefault: true},
			{Gate: "unit", Contributor: "bot@example.org"},
		},
	}
	_, _, lines := taskLines(d, list.Detail, false)
	got := texts(lines)
	for _, want := range []string{
		"design · ada@example.org · reviewer the assignee, ada@example.org",
		"implementation · bot@example.org · reviewer the nearest person, olive@example.org",
		"unit · bot@example.org",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("the junctions lack %q:\n%s", want, strings.Join(got, "\n"))
		}
	}

	w := words{}
	for _, c := range []struct {
		by        string
		byDefault bool
		want      string
	}{
		{"assignee", true, "Reviewer: olive@example.org, the assignee"},
		{"", true, "Reviewer: olive@example.org, the assignee"},
		{"a1c0", true, "Reviewer: olive@example.org, the nearest person"},
		{"a1c0", false, "Reviewer: olive@example.org, from a1c0"},
	} {
		it := list.QueueItem{
			Kind:     "ready",
			Task:     list.TaskRef{ID: "b2c9"},
			Gate:     "design",
			Junction: &list.Junction{Gate: "design", Reviewer: "olive@example.org", ByDefault: c.byDefault},
			Sources:  []list.Supplier{{Gate: "design", Field: "reviewer", Value: "olive@example.org", By: c.by}},
		}
		if got := texts(w.queueDetail(it, 0)); !slices.Contains(got, c.want) {
			t.Errorf("supplier %q: the queue lacks %q:\n%s", c.by, c.want, strings.Join(got, "\n"))
		}
	}
	waived := list.QueueItem{Kind: "ready", Gate: "unit", Junction: &list.Junction{Gate: "unit", Contributor: "bot@example.org"}}
	if got := strings.Join(texts(w.queueDetail(waived, 0)), "\n"); strings.Contains(got, "Reviewer:") {
		t.Errorf("the queue draws a waived reviewer:\n%s", got)
	}

	if got, want := w.junctionFields(list.Junction{Gate: "design", Reviewer: "olive@example.org", ByDefault: true}),
		"design: reviewer olive@example.org, by default"; got != want {
		t.Errorf("blockage = %q, want %q", got, want)
	}
	if got, want := w.junctionFields(list.Junction{Gate: "unit", Contributor: "bot@example.org"}),
		"unit: contributor bot@example.org"; got != want {
		t.Errorf("blockage, waived = %q, want %q", got, want)
	}
}
