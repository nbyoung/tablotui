package role

import (
	"context"
	"errors"
	"testing"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/ui/detail"
	"github.com/nbyoung/tablotui/internal/ui/grid"
	"github.com/nbyoung/tablotui/internal/view/list"
)

// lists records the requests of the detail panes and fails them all.
type lists struct{ reqs []source.ListRequest }

var errNoLists = errors.New("no lists")

func (l *lists) Task(_ context.Context, r source.ListRequest) (list.Task, error) {
	l.reqs = append(l.reqs, r)
	return list.Task{}, errNoLists
}

func (l *lists) Queue(_ context.Context, r source.ListRequest) (list.Queue, error) {
	l.reqs = append(l.reqs, r)
	return list.Queue{}, errNoLists
}

func (l *lists) Blockage(_ context.Context, r source.ListRequest) (list.Blockage, error) {
	l.reqs = append(l.reqs, r)
	return list.Blockage{}, errNoLists
}

func (l *lists) History(_ context.Context, r source.ListRequest) (list.History, error) {
	l.reqs = append(l.reqs, r)
	return list.History{}, errNoLists
}

// A pane states the viewer and the role in force on its requests, so that a
// source opens it at the level the role gives; a switch of role asks again.
func TestThePanesFollowTheRole(t *testing.T) {
	src := &spy{File: source.File{Dir: fixtures}}
	ls := &lists{}
	set := detail.New(detail.Options{Source: ls})
	g := grid.New(grid.Options{Source: src})
	who := New(Options{Source: src, Viewer: owner})
	m := ui.New(ui.Options{
		Panes:    append([]ui.Pane{g}, append(set.Panes, who)...),
		Commands: append(set.Commands, Command()),
		Layout:   set.Layout,
		Styles:   ui.DefaultStyles(),
	})
	r := launch(t, m, src, 140, 40)
	if len(ls.reqs) != 0 {
		t.Fatalf("closed panes asked %+v", ls.reqs)
	}
	r.keys("1")
	if len(ls.reqs) != 1 {
		t.Fatalf("the task pane asked %+v", ls.reqs)
	}
	want := source.ListRequest{View: "task", Task: "437e", Viewer: owner, Role: Owner}
	if ls.reqs[0] != want {
		t.Errorf("request = %+v, want %+v", ls.reqs[0], want)
	}
	r.keys("R")
	if len(ls.reqs) != 2 || ls.reqs[1].Role != Assignee || ls.reqs[1].Viewer != owner || ls.reqs[1].Level != "" {
		t.Errorf("after R: %+v", ls.reqs)
	}
}
