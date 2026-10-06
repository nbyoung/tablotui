package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// section is one scope of bindings in the help mode.
type section struct {
	Scope string
	Keys  []key.Binding
}

// helpMode lists every binding by scope with its help text.
type helpMode struct {
	sections []section
	top      int
	keys     struct{ Close, Up, Down key.Binding }
}

func newHelpMode(sections []section) *helpMode {
	h := &helpMode{sections: sections}
	h.keys.Close = key.NewBinding(key.WithKeys("?", "q", "esc"), key.WithHelp("esc", "close"))
	h.keys.Up = key.NewBinding(key.WithKeys("up", "k", "pgup"), key.WithHelp("↑↓", "scroll"))
	h.keys.Down = key.NewBinding(key.WithKeys("down", "j", "pgdown"))
	return h
}

func (h *helpMode) lines() []string {
	var out []string
	width := 0
	for _, s := range h.sections {
		for _, b := range s.Keys {
			if w := len([]rune(b.Help().Key)); w > width {
				width = w
			}
		}
	}
	for i, s := range h.sections {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, s.Scope)
		for _, b := range s.Keys {
			hp := b.Help()
			if !b.Enabled() || (hp.Key == "" && hp.Desc == "") {
				continue
			}
			pad := strings.Repeat(" ", max(0, width-len([]rune(hp.Key))))
			out = append(out, "  "+hp.Key+pad+"  "+hp.Desc)
		}
	}
	return out
}

// Update implements Mode. It closes on ?, q and esc, and scrolls.
func (h *helpMode) Update(msg tea.Msg, c Context) (Mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return h, nil
	}
	switch {
	case key.Matches(k, h.keys.Close):
		return nil, nil
	case key.Matches(k, h.keys.Up):
		h.top = max(0, h.top-1)
	case key.Matches(k, h.keys.Down):
		h.top++
	}
	return h, nil
}

// View implements Mode.
func (h *helpMode) View(w, hgt int, c Context) string {
	all := h.lines()
	top := min(h.top, max(0, len(all)-hgt))
	out := make([]string, hgt)
	for i := range out {
		var s string
		if top+i < len(all) {
			s = all[top+i]
		}
		out[i] = c.Measure.Fit(s, w)
		if top+i == 0 || (top+i < len(all) && !strings.HasPrefix(all[top+i], " ") && all[top+i] != "") {
			out[i] = c.Styles.Header.Render(out[i])
		}
	}
	return strings.Join(out, "\n")
}

// Status implements Mode.
func (h *helpMode) Status(c Context) string { return "help" }

// Keys implements Mode.
func (h *helpMode) Keys() []key.Binding { return []key.Binding{h.keys.Close, h.keys.Up} }
