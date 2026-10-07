package ui_test

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/nbyoung/tablotui/internal/ui"
)

// T4: the symbols of SYNTAX.md's gate sets and marks measure as the design says.
func TestMeasureSymbols(t *testing.T) {
	wide := strings.Fields("❔ 📝 📌 🔧 ⚡ ⚓ 📐 🧱 📏 🔗 🌍 🚀 ⚪ 🟢 🟡 🔴 ✅ 🪫 ⛔ 👓 🤖 🧑 👀 🪆")
	narrow := strings.Fields("— … × ─ ▸ ▾ ·")
	for _, method := range []ansi.Method{ansi.WcWidth, ansi.GraphemeWidth} {
		m := ui.Measure{Method: method}
		for _, s := range wide {
			if got := m.Width(s); got != 2 {
				t.Errorf("method %d: Width(%s) = %d, want 2", method, s, got)
			}
		}
		for _, s := range narrow {
			if got := m.Width(s); got != 1 {
				t.Errorf("method %d: Width(%s) = %d, want 1", method, s, got)
			}
		}
	}
}

func TestMeasureVariationSelector(t *testing.T) {
	for _, s := range []string{"⚙️", "🛠️", "🖼️"} {
		wc, gr := ui.Measure{Method: ansi.WcWidth}, ui.Measure{Method: ansi.GraphemeWidth}
		if got := wc.Clean(s); strings.ContainsRune(got, '\uFE0F') || wc.Width(s) != 1 {
			t.Errorf("WcWidth: Clean(%q) = %q, width %d, want no U+FE0F and 1 cell", s, got, wc.Width(s))
		}
		if got := gr.Clean(s); got != s || gr.Width(s) != 2 {
			t.Errorf("GraphemeWidth: Clean(%q) = %q, width %d, want it unchanged and 2 cells", s, got, gr.Width(s))
		}
	}
	// A selector after a wide base stays: the base already takes two cells.
	wc := ui.Measure{Method: ansi.WcWidth}
	if got := wc.Clean("🔧\uFE0F"); got != "🔧\uFE0F" {
		t.Errorf("Clean(wide base + U+FE0F) = %q, want it unchanged", got)
	}
}

func TestMeasureFitCentreWrap(t *testing.T) {
	m := ui.Measure{}
	if got := m.Fit("abc", 5); got != "abc  " {
		t.Errorf("Fit pad = %q", got)
	}
	if got := m.Fit("abcdefg", 5); got != "abcd…" {
		t.Errorf("Fit cut = %q", got)
	}
	if got := m.Fit("🤖👀🤖", 5); m.Width(got) != 5 {
		t.Errorf("Fit wide = %q, width %d", got, m.Width(got))
	}
	if got := m.Fit("x", 0); got != "" {
		t.Errorf("Fit 0 = %q", got)
	}
	if got := m.Centre("ab", 5); got != " ab  " {
		t.Errorf("Centre odd = %q, want the odd cell on the right", got)
	}
	if got := m.Centre("🤖", 5); got != " 🤖  " {
		t.Errorf("Centre wide = %q", got)
	}
	if got := m.Centre("abcdef", 4); got != "abc…" {
		t.Errorf("Centre cut = %q", got)
	}
	got := m.Wrap("one two three four five", 9, 3)
	if want := []string{"one two", "three", "four five"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("Wrap = %q, want %q", got, want)
	}
	got = m.Wrap("one two three four five six", 9, 2)
	if want := []string{"one two", "three …"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("Wrap long = %q, want %q", got, want)
	}
	got = m.Wrap("short", 20, 2)
	if len(got) != 2 || got[0] != "short" || got[1] != "" {
		t.Errorf("Wrap short = %q", got)
	}
	got = m.Wrap("abcdefghijklmnop qrs", 6, 2)
	if len(got) != 2 || m.Width(got[0]) > 6 || m.Width(got[1]) > 6 {
		t.Errorf("Wrap one long word = %q", got)
	}
}

// probe is a pane that records the measure it receives.
type probe struct{ method *ansi.Method }

func (p probe) ID() string                 { return "probe" }
func (p probe) Init(c ui.Context) tea.Cmd  { return nil }
func (p probe) Keys() []key.Binding        { return nil }
func (p probe) Status(c ui.Context) string { return "" }
func (p probe) Update(msg tea.Msg, c ui.Context) (ui.Pane, tea.Cmd) {
	*p.method = c.Measure.Method
	return p, nil
}
func (p probe) View(w, h int, c ui.Context) string {
	*p.method = c.Measure.Method
	return ""
}

// T5: a mode report for 2027 selects the grapheme method.
func TestModeReportSwitchesMeasure(t *testing.T) {
	cases := []struct {
		name  string
		mode  ansi.Mode
		value ansi.ModeSetting
		want  ansi.Method
	}{
		{"set", ansi.ModeUnicodeCore, ansi.ModeSet, ansi.GraphemeWidth},
		{"reset", ansi.ModeUnicodeCore, ansi.ModeReset, ansi.GraphemeWidth},
		{"permanently set", ansi.ModeUnicodeCore, ansi.ModePermanentlySet, ansi.GraphemeWidth},
		{"not recognised", ansi.ModeUnicodeCore, ansi.ModeNotRecognized, ansi.WcWidth},
		{"permanently reset", ansi.ModeUnicodeCore, ansi.ModePermanentlyReset, ansi.WcWidth},
		{"another mode", ansi.ModeFocusEvent, ansi.ModeSet, ansi.WcWidth},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got ansi.Method = 99
			var m tea.Model = ui.New(ui.Options{Panes: []ui.Pane{probe{&got}}})
			m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m, _ = m.Update(tea.ModeReportMsg{Mode: c.mode, Value: c.value})
			m.View()
			if got != c.want {
				t.Errorf("method = %d, want %d", got, c.want)
			}
		})
	}
}
