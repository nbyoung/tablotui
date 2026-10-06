package ui

import (
	"errors"
	"fmt"
	"sort"

	"charm.land/bubbles/v2/key"
)

// frameKeys are the keys the frame owns, in the order the help mode lists them.
type frameKeys struct {
	Quit, Help, Next, Prev, Home key.Binding
}

func newFrameKeys() frameKeys {
	return frameKeys{
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Next: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next pane")),
		Prev: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous pane")),
		Home: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "home pane")),
	}
}

func (k frameKeys) all() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.Next, k.Prev, k.Home}
}

// CheckKeys reports a key that two bindings share among the frame's own keys,
// the commands and one pane, for each pane. It returns nil when no pane has a
// clash.
func CheckKeys(o Options) error {
	var own []key.Binding
	own = append(own, newFrameKeys().all()...)
	for _, c := range o.Commands {
		own = append(own, c.Binding)
	}
	seen := map[string]bool{}
	var errs []error
	for _, p := range o.Panes {
		owner := map[string]key.Binding{}
		for _, b := range append(append([]key.Binding{}, own...), p.Keys()...) {
			if !b.Enabled() {
				continue
			}
			keys := append([]string(nil), b.Keys()...)
			sort.Strings(keys)
			for _, k := range keys {
				if prev, ok := owner[k]; ok {
					msg := fmt.Sprintf("key %q is bound twice: %s and %s", k, describe(prev), describe(b))
					if !seen[msg] {
						seen[msg] = true
						errs = append(errs, fmt.Errorf("pane %q: %s", p.ID(), msg))
					}
					continue
				}
				owner[k] = b
			}
		}
	}
	return errors.Join(errs...)
}

func describe(b key.Binding) string {
	h := b.Help()
	if h.Key == "" && h.Desc == "" {
		return fmt.Sprintf("%v", b.Keys())
	}
	return fmt.Sprintf("%q", h.Key+" "+h.Desc)
}
