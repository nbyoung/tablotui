package grid

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view"
)

const person = "nbyoung@nbyoung.com"

func contextStart() Start {
	return Start{
		Request: source.Request{View: "context", Person: person, Window: 1, Level: "detail"},
		Level:   Detail,
		Mine:    true,
	}
}

func globalStart() Start {
	return Start{
		Request: source.Request{View: "tableau", Person: person, Window: 1, Level: "detail"},
		Mine:    true,
	}
}

func (r *rig) lastSel() string { return r.selected[len(r.selected)-1].Task }

var idRow = regexp.MustCompile(`(?m)^[0-9a-f]{4} `)

// rowsIn counts the task rows the screen draws above the status line.
func rowsIn(r *rig) int {
	n := 0
	for _, l := range strings.Split(r.screen(), "\n") {
		if strings.Trim(l, "─") == "" && l != "" {
			break
		}
		if idRow.MatchString(l) && !strings.Contains(l, " · rows ") {
			n++
		}
	}
	return n
}

// T12: Mine.
func TestMineInAContextualTableau(t *testing.T) {
	r := newRig(t, fixtures(), "", contextStart(), 100, 24)
	if got := r.lastSel(); got != "437e" {
		t.Errorf("selection = %q, want 437e", got)
	}
	s := r.screen()
	for _, want := range []string{"▾ Tableaux tooling", "▾ Method", "▸ Views (0)", "Agent identity", "tablo: backend library"} {
		if !strings.Contains(s, want) {
			t.Errorf("screen lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "▾ Views") {
		t.Errorf("Views stands unfolded:\n%s", s)
	}
}

func TestMineInAGlobalTableau(t *testing.T) {
	r := newRig(t, fixtures(), "", globalStart(), 100, 60)
	if got := r.lastSel(); got != "c2ad" {
		t.Errorf("selection = %q, want c2ad", got)
	}
	if n := rowsIn(r); n != 37 {
		t.Errorf("rows in view = %d, want 37", n)
	}
}

func TestMineWithNoRowOfTheirsFallsBackToOpen(t *testing.T) {
	src := &recorder{fn: func(q source.Request) (view.Tableau, error) {
		tb := load(t, "tableau-person.json")
		tb.Rows = append([]view.Row(nil), tb.Rows...)
		for i := range tb.Rows {
			cells := append([]view.Cell(nil), tb.Rows[i].Cells...)
			for j := range cells {
				cells[j].Acts = false
			}
			tb.Rows[i].Cells = cells
		}
		return tb, nil
	}}
	r := newRig(t, src, "", globalStart(), 100, 24)
	if got := r.lastSel(); got != "437e" {
		t.Errorf("selection = %q, want the first row", got)
	}
	if sc := r.screen(); !strings.Contains(sc, "▾ Tableaux tooling") || !strings.Contains(sc, "▸ Method") {
		t.Errorf("screen shows more than the root and its children:\n%s", sc)
	}
	s := globalStart()
	s.Open = []string{"437e", "bc63"}
	r = newRig(t, src, "", s, 100, 24)
	if sc := r.screen(); !strings.Contains(sc, "▾ Method") {
		t.Errorf("screen ignores Open:\n%s", sc)
	}
}

func TestMineYieldsToSelect(t *testing.T) {
	s := contextStart()
	s.Select = "7166"
	r := newRig(t, fixtures(), "", s, 100, 24)
	if got := r.lastSel(); got != "7166" {
		t.Errorf("selection = %q, want 7166", got)
	}
	s.Select = "nowhere"
	r = newRig(t, fixtures(), "", s, 100, 24)
	if got := r.lastSel(); got != "437e" {
		t.Errorf("selection = %q, want 437e", got)
	}
}

// T13: a start that waits.
func TestAStartThatWaits(t *testing.T) {
	src := &recorder{fn: func(q source.Request) (view.Tableau, error) {
		return source.File{Dir: "testdata"}.Tableau(context.Background(), q)
	}}
	r := newRig(t, src, "", Start{}, 100, 14)
	if len(src.reqs) != 0 {
		t.Fatalf("a waiting grid asked %+v", src.reqs)
	}
	if !strings.Contains(r.screen(), "loading…") {
		t.Errorf("screen:\n%s", r.screen())
	}
	r.keys("down C E x w H")
	r.send(ui.ReloadMsg{})
	if len(src.reqs) != 0 || len(r.notices) != 0 {
		t.Errorf("keys and a reload asked %+v, noticed %+v", src.reqs, r.notices)
	}
	if !strings.Contains(r.screen(), "loading…") || r.status() != "" {
		t.Errorf("screen:\n%s\nstatus %q", r.screen(), r.status())
	}
	r.send(StartMsg{Start: contextStart()})
	if len(src.reqs) != 1 || src.reqs[0].View != "context" {
		t.Errorf("the start asked %+v", src.reqs)
	}
	if strings.Contains(r.screen(), "loading…") || !strings.Contains(r.screen(), "Tableaux tooling") {
		t.Errorf("screen:\n%s", r.screen())
	}
}
