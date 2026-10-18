package act

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/nbyoung/tabloio/action"
)

// T5: the fifteen rows of fixtures.md, from the keys to the request.
func TestFormDefaultsAndRequest(t *testing.T) {
	const n = note9f31
	cases := []struct {
		task, gate, keys string
		want             action.Recording
	}{
		{"9f31", "", "", action.Recording{Gate: "function", State: "at_risk", Reason: "blocked", Note: n}},
		{"9f31", "defined", "", action.Recording{Gate: "function", State: "at_risk", Reason: "blocked", Note: n}},
		{"9f31", "design", "", action.Recording{Gate: "design", State: "nominal", NoReason: true, NoNote: true}},
		{"9f31", "", "right", action.Recording{Gate: "design", State: "nominal", NoReason: true, NoNote: true}},
		{"9f31", "", "right left", action.Recording{Gate: "function", State: "at_risk", Reason: "blocked", Note: n}},
		{"9f31", "", "down right down left right right up up right", action.Recording{Gate: "design", State: "stalled", Reason: "review", NoNote: true}},
		{"9f31", "", "down down down ctrl+u x up up up right", action.Recording{Gate: "design", State: "nominal", NoReason: true, Note: "x"}},
		{"9f31", "", "down down left left", action.Recording{Gate: "function", State: "at_risk", NoReason: true, Note: n}},
		{"c07d", "", "", action.Recording{Gate: "design"}},
		{"c07d", "", "down", action.Recording{Gate: "design"}},
		{"c07d", "", "left", action.Recording{Gate: "function", State: "nominal", NoReason: true, NoNote: true}},
		{"b618", "", "", action.Recording{Gate: "undefined", State: "undefined", NoReason: true, NoNote: true}},
		{"b618", "", "right down", action.Recording{Gate: "defined", State: "nominal", NoReason: true, NoNote: true}},
		{"e0aa", "release", "", action.Recording{Gate: "release", State: "complete", NoReason: true, NoNote: true}},
		{"e0aa", "", strings.Repeat("left ", 8), action.Recording{Gate: "undefined", State: "undefined", NoReason: true, NoNote: true}},
	}
	for i, c := range cases {
		r := newRig(t, 80, 16, nil).selects(c.task, c.gate).keys("r " + c.keys + " enter")
		c.want.ID = c.task
		if got := r.repo.last(); got.method != "Record" || got.rec != c.want || got.o != (action.Options{}) {
			t.Errorf("row %d: %s %+v %+v, want %+v", i+1, got.method, got.rec, got.o, c.want)
		}
	}
}

// T6: the gate choices step over a gate that does not apply.
func TestFormGateChoices(t *testing.T) {
	r := newRig(t, 80, 16, nil).selects("c07d", "").keys("r left left")
	if l := r.lines()[5]; l != "> gate    < defined >" {
		t.Errorf("the gate reads %q", l)
	}
	if got := r.status(); got != "record c07d · gate · 2 of 9" {
		t.Errorf("status %q", got)
	}
}

// T7: the note editor: text, keys that are text there, a paste, each motion
// and each deletion; the cursor in the status line; the trimmed note in the
// request.
func TestNoteEditor(t *testing.T) {
	r := newRig(t, 80, 16, nil).selects("b618", "").keys("r right down down down")
	note := func() string { t.Helper(); return strings.TrimPrefix(r.lines()[8], "> note    ") }
	r.text("q?yn")
	if got := note(); got != "q?yn" || r.status() != "record b618 · note · cursor 4 of 4" {
		t.Errorf("text: %q, %q", got, r.status())
	}
	r.send(tea.PasteMsg{Content: "a\r\nb\tc\n"})
	if got := note(); got != "q?yna b c" {
		t.Errorf("paste: %q", got)
	}
	r.keys("home")
	if r.status() != "record b618 · note · cursor 0 of 10" {
		t.Errorf("home: %q", r.status())
	}
	r.keys("right ctrl+f ctrl+b")
	if r.status() != "record b618 · note · cursor 1 of 10" {
		t.Errorf("right ctrl+f ctrl+b: %q", r.status())
	}
	r.keys("delete ctrl+d")
	if got := note(); got != "qna b c" {
		t.Errorf("delete ctrl+d: %q", got)
	}
	r.keys("end backspace ctrl+h")
	if got := note(); got != "qna b" || r.status() != "record b618 · note · cursor 6 of 6" {
		t.Errorf("end backspace ctrl+h: %q, %q", got, r.status())
	}
	r.keys("ctrl+a right right ctrl+k")
	if got := note(); got != "qn" {
		t.Errorf("ctrl+k: %q", got)
	}
	r.keys("left ctrl+u")
	if got := note(); got != "n" || r.status() != "record b618 · note · cursor 0 of 1" {
		t.Errorf("ctrl+u: %q, %q", got, r.status())
	}
	r.keys("ctrl+e space").text(" x  ")
	r.keys("enter")
	if got := r.repo.last().rec; got.Note != "n  x" || got.NoNote {
		t.Errorf("the request holds the note %q", got.Note)
	}
}
