package detail

import (
	"testing"

	"github.com/nbyoung/tablotui/internal/source"
)

// An open pane states the viewer and the role on its requests, and asks again
// when the role changes, since the role decides the level a view opens at.
func TestAnOpenPaneStatesTheReading(t *testing.T) {
	x := pane(t, TaskID, "testdata", 80, 20)
	x.send(source.Reading{Viewer: "ada@example.org", Role: "assignee"})
	x.sel("e9c6", "design").open()
	want := source.ListRequest{View: "task", Task: "e9c6", Viewer: "ada@example.org", Role: "assignee"}
	if reqs := x.requests(); len(reqs) != 1 || reqs[0] != want {
		t.Fatalf("requests = %+v, want %+v", reqs, want)
	}
	x.send(source.Reading{Viewer: "ada@example.org", Role: "assignee"})
	if n := x.src.count(); n != 1 {
		t.Errorf("the same reading asked again: %d requests", n)
	}
	x.send(source.Reading{Viewer: "ada@example.org", Role: "reviewer"})
	reqs := x.requests()
	if len(reqs) != 2 || reqs[1].Role != "reviewer" || reqs[1].Task != "e9c6" || reqs[1].Level != "" {
		t.Errorf("requests = %+v, want a second one for the reviewer", reqs)
	}
}

// A closed pane asks nothing at a change of role and asks as the new role when
// it opens.
func TestAClosedPaneAsksAsTheNewRoleWhenItOpens(t *testing.T) {
	for _, id := range allPanes {
		x := pane(t, id, "testdata", 80, 20)
		x.send(source.Reading{Viewer: "ada@example.org", Role: "owner"}).sel("e9c6", "")
		x.send(source.Reading{Viewer: "ada@example.org", Role: "contributor"})
		if n := x.src.count(); n != 0 {
			t.Fatalf("a closed %s pane asked %d times", id, n)
		}
		x.open()
		reqs := x.requests()
		if len(reqs) != 1 || reqs[0].Viewer != "ada@example.org" || reqs[0].Role != "contributor" {
			t.Errorf("%s asked %+v", id, reqs)
		}
	}
}

// The viewer's own choice of level stays through a change of role.
func TestAChangeOfRoleKeepsTheChosenLevel(t *testing.T) {
	x := pane(t, TaskID, "testdata", 80, 20).sel("e9c6", "").open().keys("p")
	n := x.src.count()
	x.send(source.Reading{Viewer: "ada@example.org", Role: "owner"})
	got := x.src.since(n)
	if len(got) != 1 || got[0].Level != "provenance" || got[0].Role != "owner" {
		t.Errorf("requests after the change = %+v", got)
	}
}
