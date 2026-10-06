package grid

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/nbyoung/tablotui/internal/ui"
)

// T13: keys and help.
func TestCheckKeysPassesForTheGrid(t *testing.T) {
	g := New(Options{Source: fixtures(), Start: GlanceStart()})
	if err := ui.CheckKeys(ui.Options{Panes: []ui.Pane{g}}); err != nil {
		t.Errorf("CheckKeys: %v", err)
	}
	// A command on a key the grid owns clashes.
	cmd := ui.Command{Binding: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "act"))}
	if err := ui.CheckKeys(ui.Options{Panes: []ui.Pane{g}, Commands: []ui.Command{cmd}}); err == nil || !strings.Contains(err.Error(), `"x"`) {
		t.Errorf("CheckKeys with a command on x: %v", err)
	}
	// So does a stub pane that reuses x beside the grid's own binding for it.
	stub := dup{g}
	if err := ui.CheckKeys(ui.Options{Panes: []ui.Pane{stub}}); err == nil {
		t.Error("CheckKeys passed a pane with two bindings on x")
	}
}

// dup is the grid with one more binding on x.
type dup struct{ Grid }

func (d dup) Keys() []key.Binding {
	return append(d.Grid.Keys(), key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "again")))
}

func TestNoBindingTakesAReservedKey(t *testing.T) {
	reserved := strings.Fields("1 2 3 4 z R r v a u")
	g := New(Options{Source: fixtures(), Start: GlanceStart()})
	var all []string
	for _, b := range g.Keys() {
		all = append(all, b.Keys()...)
	}
	km := reflect.ValueOf(newKeyMap())
	for i := 0; i < km.NumField(); i++ {
		all = append(all, km.Field(i).Interface().(key.Binding).Keys()...)
	}
	for _, k := range all {
		if slices.Contains(reserved, k) {
			t.Errorf("a binding takes the reserved key %q", k)
		}
	}
}

func TestHelpKeysCoverEveryKeyTheGridHandles(t *testing.T) {
	helped := map[string]bool{}
	for _, b := range helpKeys() {
		for _, k := range b.Keys() {
			helped[k] = true
		}
	}
	km := reflect.ValueOf(newKeyMap())
	handled := map[string]bool{}
	for i := 0; i < km.NumField(); i++ {
		for _, k := range km.Field(i).Interface().(key.Binding).Keys() {
			handled[k] = true
			if !helped[k] {
				t.Errorf("the key %q has no help", k)
			}
		}
	}
	for k := range helped {
		if !handled[k] {
			t.Errorf("the help names %q, which does nothing", k)
		}
	}
}

func TestGridHelpOrder(t *testing.T) {
	var got []string
	for _, b := range New(Options{}).Keys() {
		got = append(got, b.Help().Key+" "+b.Help().Desc)
	}
	want := []string{"↑↓ row", "←→ column", "enter fold", "E all", "C glance", "d detail", "p provenance", "x hide", "s show", "X window", "w/W widen/narrow", "H historical"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("help = %q, want %q", got, want)
	}
}

func TestHelpModeListsTheGridKeys(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 100, 40).keys("?")
	body := r.screen()
	for _, want := range []string{"row", "column", "fold", "all", "glance", "detail", "provenance", "hide", "show", "window", "widen/narrow", "historical", "quit", "next pane"} {
		if !strings.Contains(body, want) {
			t.Errorf("the help mode lacks %q:\n%s", want, body)
		}
	}
	r.keys("down q")
	if r.quit {
		t.Error("q quit under the help mode")
	}
	if strings.Contains(r.screen(), "next pane") {
		t.Error("q did not close the help mode")
	}
	// The grid does not see the keys of an open mode.
	r.keys("?")
	sel := len(r.selected)
	r.keys("j j j x")
	if len(r.selected) != sel {
		t.Error("a key reached the grid under the help mode")
	}
}

func TestQuitKeys(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 100, 20).keys("q")
	if !r.quit {
		t.Error("q did not quit")
	}
	r = newRig(t, fixtures(), "", GlanceStart(), 100, 20).keys("ctrl+c")
	if !r.quit {
		t.Error("ctrl+c did not quit")
	}
}

func TestUnknownKeysAndMessagesChangeNothing(t *testing.T) {
	r := newRig(t, fixtures(), "", GlanceStart(), 100, 20)
	before := r.screen()
	r.keys("z 1 R r v a u")
	r.send(tea.FocusMsg{})
	if r.screen() != before {
		t.Error("an unbound key changed the screen")
	}
}
