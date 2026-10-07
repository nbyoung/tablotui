package grid

import "charm.land/bubbles/v2/key"

// keyMap holds the bindings that the grid dispatches on, one per action.
type keyMap struct {
	Up, Down, PageUp, PageDown, First, Last key.Binding
	Left, Right                             key.Binding
	Fold, Unfold, Glance                    key.Binding
	Detail, Provenance                      key.Binding
	Hide, Show, Window                      key.Binding
	Wider, Narrower, Historical             key.Binding
}

func newKeyMap() keyMap {
	b := func(keys ...string) key.Binding { return key.NewBinding(key.WithKeys(keys...)) }
	return keyMap{
		Up:         b("up", "k"),
		Down:       b("down", "j"),
		PageUp:     b("pgup"),
		PageDown:   b("pgdown"),
		First:      b("home", "g"),
		Last:       b("end", "G"),
		Left:       b("left", "h"),
		Right:      b("right", "l"),
		Fold:       b("enter", "space"),
		Unfold:     b("E"),
		Glance:     b("C"),
		Detail:     b("d"),
		Provenance: b("p"),
		Hide:       b("x"),
		Show:       b("s"),
		Window:     b("X"),
		Wider:      b("w"),
		Narrower:   b("W"),
		Historical: b("H"),
	}
}

// helpKeys returns the bindings of the help line and the help mode, in order.
// Each carries every key of the actions it names.
func helpKeys() []key.Binding {
	h := func(k, d string, keys ...string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(k, d))
	}
	return []key.Binding{
		h("↑↓", "row", "up", "k", "down", "j", "pgup", "pgdown", "home", "g", "end", "G"),
		h("←→", "column", "left", "h", "right", "l"),
		h("enter", "fold", "enter", "space"),
		h("E", "all", "E"),
		h("C", "glance", "C"),
		h("d", "detail", "d"),
		h("p", "provenance", "p"),
		h("x", "hide", "x"),
		h("s", "show", "s"),
		h("X", "window", "X"),
		h("w/W", "widen/narrow", "w", "W"),
		h("H", "historical", "H"),
	}
}
