package main

import (
	"bytes"
	"context"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/nbyoung/tabloio/action"

	"github.com/nbyoung/tablotui/internal/ui/act"
)

// blockingRepo plans a reaffirmation and makes its commit only once the
// test releases it.
type blockingRepo struct {
	entered, release chan struct{}
	done             atomic.Bool
}

func (r *blockingRepo) Record(context.Context, action.Recording, action.Options) (*action.Plan, error) {
	return nil, action.Refusals{{Code: "no-change", Text: "unused"}}
}

func (r *blockingRepo) Review(context.Context, []string, string, action.Options) (*action.Plan, error) {
	return nil, action.Refusals{{Code: "no-reviewer", Text: "unused"}}
}

func (r *blockingRepo) Reaffirm(_ context.Context, ids []string, _ action.Options) (*action.Plan, error) {
	return &action.Plan{Command: "reaffirm", Tasks: ids, Branch: "main", Subject: "Reaffirm only task (" + ids[0] + ")"}, nil
}

func (r *blockingRepo) Authorise(context.Context, []string, action.Options) (*action.Plan, error) {
	return nil, action.Refusals{{Code: "off-trunk", Text: "unused"}}
}

func (r *blockingRepo) Apply(context.Context, *action.Plan) (action.Result, error) {
	close(r.entered)
	<-r.release
	r.done.Store(true)
	return action.Result{Commit: "0881b37c2f9d4e1a8b7c6d5e4f3a2b1c0d9e8f7a"}, nil
}

// releaser is standard error: the line that says the command waits
// releases the commit.
type releaser struct {
	bytes.Buffer
	repo *blockingRepo
}

func (w *releaser) Write(p []byte) (int, error) {
	if strings.Contains(string(p), "a commit is in flight; waiting for git") {
		close(w.repo.release)
	}
	return w.Buffer.Write(p)
}

// T18 of 518e: under --fixture the actions say that no action runs, and
// run waits for a commit in flight after the program returns.
func TestTheActions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	key := func(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

	h := newHarness(t)
	var status string
	h.onRun = func(m tea.Model) {
		l := lines(typed(drive(m, 80, 12), key('r')))
		status = l[len(l)-2]
	}
	if code, _, errOut := runArgs("--fixture", fixtureDir(t)); code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if status != "no action: this build draws fixtures" {
		t.Errorf("r answers %q", status)
	}

	repo := &blockingRepo{entered: make(chan struct{}), release: make(chan struct{})}
	old := newActions
	newActions = func(o act.Options) *act.Actions {
		o.Repo = repo
		o.Uncommitted = func(context.Context, string) ([]string, error) { return nil, nil }
		return act.New(o)
	}
	t.Cleanup(func() { newActions = old })
	h.onRun = func(m tea.Model) {
		m = typed(drive(m, 80, 12), key('a'))
		_, cmd := m.Update(key('y'))
		// The program runs the commit's command in a goroutine of its own
		// and ends while the commit runs.
		go func() { _ = cmd() }()
		<-repo.entered
	}
	stderr := &releaser{repo: repo}
	var out bytes.Buffer
	code := run([]string{"--fixture", fixtureDir(t)}, &out, stderr)
	if code != 0 || stderr.String() != "tablotui: a commit is in flight; waiting for git\n" {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
	if !repo.done.Load() {
		t.Error("run returned before the commit ended")
	}
}
