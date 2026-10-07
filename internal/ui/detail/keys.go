package detail

import "charm.land/bubbles/v2/key"

// keyMap holds the bindings a detail pane dispatches on.
type keyMap struct {
	Up, Down, PageUp, PageDown, First, Last key.Binding
	Go, Detail, Provenance, Fold            key.Binding
}

func newKeyMap() keyMap {
	b := func(keys ...string) key.Binding { return key.NewBinding(key.WithKeys(keys...)) }
	return keyMap{
		Up: b("up", "k"), Down: b("down", "j"), PageUp: b("pgup"), PageDown: b("pgdown"),
		First: b("home", "g"), Last: b("end", "G"),
		Go: b("enter"), Detail: b("d"), Provenance: b("p"), Fold: b("f"),
	}
}

// helpKeys returns the bindings of the help line and the help mode, in order.
// Each carries every key of the actions it names.
func helpKeys() []key.Binding {
	h := func(k, d string, keys ...string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(k, d))
	}
	return []key.Binding{
		h("↑↓", "line", "up", "k", "down", "j", "pgup", "pgdown", "home", "g", "end", "G"),
		h("enter", "select task", "enter"),
		h("d", "detail", "d"),
		h("p", "provenance", "p"),
		h("f", "fold", "f"),
	}
}
