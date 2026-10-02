package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

var persons = []string{"ada@example.org", "ben@example.org", "dan@example.org"}

func newModel(t *testing.T, w, h int, strip bool) Model {
	t.Helper()
	m, err := NewModel(source(), "ada@example.org", persons, strip)
	if err != nil {
		t.Fatal(err)
	}
	return send(m, tea.WindowSizeMsg{Width: w, Height: h})
}

func send(m Model, msgs ...tea.Msg) Model {
	for _, msg := range msgs {
		tm, _ := m.Update(msg)
		m = tm.(Model)
	}
	return m
}

func keys(m Model, ks ...string) Model {
	for _, k := range ks {
		m = send(m, keyMsg(k))
	}
	return m
}

func TestPanesFollowSelection(t *testing.T) {
	m := newModel(t, 120, 40, false)
	m = keys(m, "j", "j") // a1c0 -> 4e2b -> 9f31
	if m.selected() != "9f31" {
		t.Fatalf("selected %s", m.selected())
	}
	v := m.View()
	for _, want := range []string{"Task 9f31", "Blockage 9f31", "History 9f31", "Sensor board", "Barometer ICs", "a2393fb"} {
		if !strings.Contains(v, want) {
			t.Errorf("view at 9f31 lacks %q", want)
		}
	}
	m = keys(m, "j") // c07d
	v = m.View()
	for _, want := range []string{"Task c07d", "History c07d", "Node firmware", "pin firmware at cf5b169"} {
		if !strings.Contains(v, want) {
			t.Errorf("view at c07d lacks %q", want)
		}
	}
	for _, gone := range []string{"Task 9f31", "a2393fb"} {
		if strings.Contains(v, gone) {
			t.Errorf("view at c07d still shows %q", gone)
		}
	}
}

func TestNoData(t *testing.T) {
	m := newModel(t, 120, 40, false) // a1c0 has a definition but no history file
	v := m.View()
	if !strings.Contains(v, "History a1c0") || !strings.Contains(v, "The source holds no history view for a1c0.") {
		t.Errorf("no-data text missing:\n%s", v)
	}
	if !strings.Contains(v, "Task a1c0") || strings.Contains(v, "holds no task-definition") {
		t.Errorf("definition of a1c0 should show:\n%s", v)
	}
	m = keys(m, "j", "j", "j", "j") // 7b2e: nothing blocks it
	if !strings.Contains(m.View(), "Nothing blocks 7b2e") {
		t.Errorf("empty blockage text missing")
	}
	_, err := source().Load(ViewHistory, Params{Task: "zzzz"})
	if !errors.Is(err, ErrNoData) {
		t.Errorf("unknown task: %v", err)
	}
}

func TestQueueBelongsToPerson(t *testing.T) {
	m := newModel(t, 120, 40, false)
	m = keys(m, "j", "j")
	if !strings.Contains(m.View(), "▸ work waiting: 9f31") {
		t.Errorf("the queue does not mark the selected task")
	}
	if m.panes[paneQueue].title != "Queue ada@example.org" || !strings.Contains(m.View(), "authorisation owed: 3c5d Dashboard") {
		t.Errorf("queue of ada missing")
	}
	// The selection moves to 3c5d, ada's queue stays.
	m = keys(m, "j", "j", "j")
	if !strings.Contains(m.View(), "reaffirmation: 9f31") {
		t.Errorf("queue changed with the selection")
	}
	m = keys(m, "p") // ben: empty queue
	if m.person != "ben@example.org" || !strings.Contains(m.View(), "The queue of ben@example.org is empty.") {
		t.Errorf("queue of ben missing:\n%s", m.View())
	}
	if m.selected() != "3c5d" {
		t.Errorf("a change of person moved the selection")
	}
}

func TestFocusAndScroll(t *testing.T) {
	m := newModel(t, 120, 20, false)
	m = keys(m, "j", "j") // 9f31
	if m.focus != 0 {
		t.Fatal("focus starts on the list")
	}
	m = keys(m, "tab")
	if m.focus != 1 {
		t.Fatalf("focus %d", m.focus)
	}
	m = keys(m, "j", "j", "j")
	if m.panes[paneDefinition].scroll != 3 || m.selected() != "9f31" {
		t.Errorf("scroll %d, selected %s", m.panes[paneDefinition].scroll, m.selected())
	}
	if m.panes[paneHistory].scroll != 0 {
		t.Error("another pane scrolled")
	}
	m = keys(m, "end")
	n := m.panes[paneDefinition].scroll
	m = keys(m, "j")
	if m.panes[paneDefinition].scroll != n || n == 0 {
		t.Errorf("end does not clamp: %d", n)
	}
	m = keys(m, "home", "k")
	if m.panes[paneDefinition].scroll != 0 {
		t.Error("home does not reach the top")
	}
	m = keys(m, "end", "tab", "tab", "tab", "tab") // past the history to the list
	if m.focus != 0 {
		t.Fatalf("tab cycle ends at %d", m.focus)
	}
	m = keys(m, "j") // moves the selection and resets the scroll
	if m.selected() != "c07d" || m.panes[paneDefinition].scroll != 0 {
		t.Errorf("selected %s scroll %d", m.selected(), m.panes[paneDefinition].scroll)
	}
	m = keys(m, "3", "esc")
	if m.focus != 0 || m.active != paneQueue {
		t.Errorf("focus %d active %d", m.focus, m.active)
	}
}

// checkExact asserts the view has exactly h lines of exactly w cells.
func checkExact(t *testing.T, m Model, w, h int, label string) {
	t.Helper()
	lines := strings.Split(m.View(), "\n")
	if len(lines) != h {
		t.Errorf("%s %dx%d: %d lines", label, w, h, len(lines))
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got != w {
			t.Errorf("%s %dx%d line %d: width %d: %q", label, w, h, i, got, ansi.Strip(l))
			return
		}
	}
}

func TestSplitLayoutExactSize(t *testing.T) {
	sizes := []struct {
		name string
		w, h int
	}{{"wide", 120, 40}, {"narrow", 50, 30}, {"short", 80, 12}}
	for _, strip := range []bool{false, true} {
		for _, s := range sizes {
			m := newModel(t, s.w, s.h, strip)
			for sel := 0; sel < len(m.rows); sel++ {
				for _, k := range [][]string{{}, {"tab"}, {"tab", "end"}, {"z"}, {"3", "z", "end"}} {
					mm := keys(m, k...)
					checkExact(t, mm, s.w, s.h, fmt.Sprintf("%s strip=%v sel=%d keys=%v", s.name, strip, sel, k))
				}
				m = keys(m, "j")
			}
		}
	}
	if m := keys(newModel(t, 120, 40, false), "j", "j"); !strings.Contains(m.View(), "⚙️") {
		t.Error("wide view lacks the variation-selector symbol")
	}
	if m := keys(newModel(t, 120, 40, true), "j", "j"); strings.Contains(m.View(), "️") {
		t.Error("strip mode keeps a variation selector")
	}
}

func TestLayoutModes(t *testing.T) {
	for _, c := range []struct {
		w, h int
		mode string
	}{{120, 40, "grid"}, {50, 30, "stack"}, {80, 12, "single"}} {
		if got := newModel(t, c.w, c.h, false).layout().mode; got != c.mode {
			t.Errorf("%dx%d: mode %s, want %s", c.w, c.h, got, c.mode)
		}
	}
}

func TestEverySize(t *testing.T) {
	m := newModel(t, 100, 30, false)
	m = keys(m, "j", "j", "j") // c07d: junction rows with wide symbols
	for w := 1; w <= 140; w++ {
		for h := 1; h <= 50; h += 7 {
			checkExact(t, send(m, tea.WindowSizeMsg{Width: w, Height: h}), w, h, "sweep")
		}
	}
	for h := 1; h <= 50; h++ {
		checkExact(t, send(m, tea.WindowSizeMsg{Width: 77, Height: h}), 77, h, "sweep")
	}
}
