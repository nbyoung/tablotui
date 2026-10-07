package grid

import (
	"strings"
	"testing"

	"github.com/nbyoung/tablotui/internal/ui"
)

// T14: the grid's answer to a GotoMsg.
func TestGotoSelectsATaskAndItsColumn(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 120, 24)
	if got := r.selected[len(r.selected)-1].Task; got != "437e" {
		t.Fatalf("arrival selects %q", got)
	}
	r.send(ui.GotoMsg{Task: "e9c6", Gate: "implementation"})
	want := ui.SelectionMsg{Task: "e9c6", Gate: "implementation"}
	if got := r.selected[len(r.selected)-1]; got != want {
		t.Errorf("selection = %+v, want %+v", got, want)
	}
	// The ancestors of the task stand unfolded, so its row is in view.
	if !strings.Contains(r.screen(), "e9c6") || !strings.Contains(r.screen(), "▾ Views") {
		t.Errorf("screen:\n%s", r.screen())
	}
}

func TestGotoLeavesTheColumnOfAFoldedGate(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 120, 24)
	r.send(ui.GotoMsg{Task: "e9c6", Gate: "implementation"})
	// The undefined gate stands in a folded column: the column cursor stays.
	r.send(ui.GotoMsg{Task: "c2ad", Gate: "undefined"})
	want := ui.SelectionMsg{Task: "c2ad", Gate: "implementation"}
	if got := r.selected[len(r.selected)-1]; got != want {
		t.Errorf("selection = %+v, want %+v", got, want)
	}
	// A GotoMsg with no gate moves the row alone.
	r.send(ui.GotoMsg{Task: "e9c6"})
	want = ui.SelectionMsg{Task: "e9c6", Gate: "implementation"}
	if got := r.selected[len(r.selected)-1]; got != want {
		t.Errorf("selection = %+v, want %+v", got, want)
	}
}

func TestGotoANewTaskLeavesTheFoldsOfOthers(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 120, 24)
	r.keys("C")
	r.send(ui.GotoMsg{Task: "99f0"})
	if got := r.selected[len(r.selected)-1].Task; got != "99f0" {
		t.Errorf("selection = %q", got)
	}
	for _, id := range []string{"bc63", "2034", "bc86"} {
		if !strings.Contains(r.screen(), id) {
			t.Errorf("the ancestor %s is not in view:\n%s", id, r.screen())
		}
	}
}

func TestGotoATaskTheTableauLacks(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 120, 24)
	n := len(r.selected)
	r.send(ui.GotoMsg{Task: "zzzz"})
	if len(r.selected) != n {
		t.Errorf("a missing task moved the selection: %+v", r.selected[n:])
	}
	if got := r.notices[len(r.notices)-1]; got.Text != "zzzz is not in the tableau in view" || got.Err {
		t.Errorf("notice = %+v", got)
	}
}

func TestGotoBeforeTheFirstView(t *testing.T) {
	g := New(Options{Source: fixtures(), Start: GlanceStart()})
	p, cmd := g.Update(ui.GotoMsg{Task: "e9c6"}, ui.Context{})
	if cmd != nil || p.(Grid).sel != "" {
		t.Errorf("a grid with no data answered a GotoMsg: %v, %+v", cmd, p)
	}
}
