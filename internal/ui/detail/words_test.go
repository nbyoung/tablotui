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
