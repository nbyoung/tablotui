package grid

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/view"
)

var fixtureRequests = map[string]source.Request{
	"tableau":            reqGlobal,
	"tableau-window0":    reqWindow0,
	"tableau-historical": reqHistorical,
	"tableau-person":     reqPerson,
	"tableau-empty":      {View: "tableau-empty", Window: 1, Level: "detail"},
}

var keyScripts = []string{
	"",
	"E d down down down",
	"E p down down right right right right right right",
	"right right right right right right right right right right right",
	"E end x left x X s",
	"E end pgup pgup d C right right right x right s",
}

// T2: over the sizes the frame draws, View has height lines, each exactly width cells.
func TestEveryLineFits(t *testing.T) {
	src := newCached()
	type size struct{ w, h int }
	var sizes []size
	for w := 40; w <= 140; w++ {
		for _, h := range []int{8, 40} {
			sizes = append(sizes, size{w, h})
		}
	}
	for h := 8; h <= 40; h++ {
		for _, w := range []int{40, 100} {
			sizes = append(sizes, size{w, h})
		}
	}
	for name, req := range fixtureRequests {
		for _, method := range []ansi.Method{ansi.WcWidth, ansi.GraphemeWidth} {
			for si, script := range keyScripts {
				t.Run(fmt.Sprintf("%s/%d/script%d", name, method, si), func(t *testing.T) {
					t.Parallel()
					start := GlanceStart()
					start.Request = req
					r := newRig(t, src, "", start, 100, 30)
					if method == ansi.GraphemeWidth {
						r.send(tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeSet})
					}
					r.keys(script)
					for _, s := range sizes {
						r.send(tea.WindowSizeMsg{Width: s.w, Height: s.h})
						lines := r.content()
						if len(lines) != s.h {
							t.Fatalf("%dx%d: %d lines", s.w, s.h, len(lines))
						}
						for i, l := range lines {
							if got := method.StringWidth(ansi.Strip(l)); got != s.w {
								t.Fatalf("%dx%d line %d is %d cells: %q", s.w, s.h, i, got, ansi.Strip(l))
							}
						}
					}
				})
			}
		}
	}
}

// The keys applied at the size, not before it, fit as well.
func TestEveryLineFitsWhenTheKeysComeAtTheSize(t *testing.T) {
	src := newCached()
	for name, req := range fixtureRequests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for w := 40; w <= 140; w += 20 {
				for h := 8; h <= 40; h += 8 {
					for si, script := range keyScripts {
						start := GlanceStart()
						start.Request = req
						r := newRig(t, src, "", start, w, h).keys(script)
						lines := r.content()
						if len(lines) != h {
							t.Fatalf("%dx%d script %d: %d lines", w, h, si, len(lines))
						}
						for i, l := range lines {
							if got := ansi.StringWidth(ansi.Strip(l)); got != w {
								t.Fatalf("%dx%d script %d line %d is %d cells: %q", w, h, si, i, got, ansi.Strip(l))
							}
						}
					}
				}
			}
		})
	}
}

// gearTableau is a tableau whose first gate has the symbol "⚙️", which
// carries U+FE0F after a base one cell wide under code-point counting.
func gearTableau() view.Tableau {
	cell := func(s string) view.Cell { return view.Cell{Kind: "marks", Symbols: s} }
	return view.Tableau{
		View: "tableau", Project: "Gear", Ref: "main", Commit: "abc1234", Date: "2026-10-06", Window: 1,
		Gates: []view.Gate{
			{Key: "build", Symbol: "⚙️", Name: "Build", Window: true, Count: 1},
			{Key: "ship", Symbol: "🚀", Name: "Ship", Window: true},
		},
		Rows: []view.Row{
			{ID: "aaaa", Title: "Engine", Cells: []view.Cell{cell("🤖"), cell("🧑")}},
			{ID: "bbbb", Title: "Frame", Cells: []view.Cell{cell("⚙️"), cell("—")}},
		},
	}
}

// T4: a gate symbol of "⚙️" draws no U+FE0F and aligns under WcWidth, and keeps
// it and aligns under GraphemeWidth.
func TestVariationSelectorSymbol(t *testing.T) {
	src := &recorder{fn: func(source.Request) (view.Tableau, error) { return gearTableau(), nil }}
	for _, c := range []struct {
		method   ansi.Method
		wantVS16 bool
	}{{ansi.WcWidth, false}, {ansi.GraphemeWidth, true}} {
		r := newRig(t, src, "", GlanceStart(), 60, 10)
		if c.method == ansi.GraphemeWidth {
			r.send(tea.ModeReportMsg{Mode: ansi.ModeUnicodeCore, Value: ansi.ModeSet})
		}
		content := r.m.(ui.Model).View().Content
		if got := strings.ContainsRune(content, '\uFE0F'); got != c.wantVS16 {
			t.Errorf("method %d: U+FE0F present = %v, want %v", c.method, got, c.wantVS16)
		}
		for i, l := range strings.Split(content, "\n") {
			if got := c.method.StringWidth(ansi.Strip(l)); got != 60 {
				t.Errorf("method %d: line %d is %d cells wide", c.method, i, got)
			}
		}
		// The header and the cells of the first gate align: each column starts where its rule does.
		lines := strings.Split(ansi.Strip(content), "\n")
		head, rule, row := lines[1], lines[2], lines[4]
		if c.method.StringWidth(head) != c.method.StringWidth(rule) || c.method.StringWidth(row) != c.method.StringWidth(rule) {
			t.Errorf("method %d: header, rule and row differ in width", c.method)
		}
		gear := "⚙"
		if c.wantVS16 {
			gear = "⚙\uFE0F"
		}
		if !strings.Contains(head, gear+" ") {
			t.Errorf("method %d: header %q lacks the gear", c.method, head)
		}
	}
}
