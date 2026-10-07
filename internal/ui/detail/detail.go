// Package detail holds the four detail panes of the tablotui program: the
// task definition, the work queue, the work-blockage tree and the history.
// Each follows the grid's selection, draws what its source sends and derives
// nothing from it. The package also supplies the keys that open the panes and
// the layout that places them beside the grid.
package detail

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
)

// Options configures the panes.
type Options struct {
	Source source.Lists
}

// Set is what the command hands the frame: the panes after the home pane,
// the commands and the layout.
type Set struct {
	Panes    []ui.Pane    // task, queue, blockage, history, in that order, all closed
	Commands []ui.Command // 1, 2, 3, 4 and z
	Layout   ui.Layout
}

// New builds the four panes, closed, with their commands and their layout.
// The z command and the layout share one arrangement, so two calls of New
// share nothing.
func New(o Options) Set {
	a := &arrangement{}
	s := Set{Layout: a.layout}
	for _, k := range kinds {
		s.Panes = append(s.Panes, newPane(k, o.Source))
		id := k.id
		s.Commands = append(s.Commands, ui.Command{
			Binding: key.NewBinding(key.WithKeys(k.digit), key.WithHelp(k.digit, strings.ToLower(k.name)+" pane")),
			Run: func(ui.SelectionMsg) tea.Cmd {
				return func() tea.Msg { return toggleMsg{pane: id} }
			},
		})
	}
	s.Commands = append(s.Commands, ui.Command{
		Binding: key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "zoom")),
		Run: func(ui.SelectionMsg) tea.Cmd {
			a.zoom = !a.zoom
			if a.zoom {
				return notice("zoom on: the focused pane takes the body", false)
			}
			return notice("zoom off", false)
		},
	})
	return s
}

// The sizes the layout keeps.
const (
	sideWidth   = 120 // from this body width the panes stand beside the grid
	stackHeight = 12  // from this body height they stand beneath it
	paneWidth   = 40  // the least width of a pane in a split
	paneHeight  = 5   // the least height of a pane in a split
	sideMax     = 80  // the most the panes' column takes
)

// arrangement is the one piece of layout state the frame does not hold:
// whether the focused pane takes the whole body. The z command and the
// layout share it.
type arrangement struct{ zoom bool }

// layout implements ui.Layout. open[0] is the home pane.
func (a *arrangement) layout(w, h int, open []string, focus string) []ui.Rect {
	if len(open) == 0 {
		return nil
	}
	home, panes := open[0], open[1:]
	whole := func(id string) []ui.Rect { return []ui.Rect{{Pane: id, W: w, H: h}} }
	focused := -1
	for i, id := range panes {
		if id == focus {
			focused = i
		}
	}
	side, stack := w >= sideWidth, h >= stackHeight
	switch {
	case len(panes) == 0:
		return whole(home)
	case a.zoom || !side && !stack:
		if focused >= 0 {
			return whole(focus)
		}
		return whole(home)
	}
	var region ui.Rect
	out := make([]ui.Rect, 0, len(open))
	if side {
		dw := min(max(w/3, paneWidth), sideMax)
		out = append(out, ui.Rect{Pane: home, W: w - dw, H: h})
		region = ui.Rect{X: w - dw, W: dw, H: h}
	} else {
		dh := h / 2
		out = append(out, ui.Rect{Pane: home, W: w, H: h - dh})
		region = ui.Rect{Y: h - dh, W: w, H: dh}
	}
	cols := max(1, region.W/paneWidth)
	rows := max(1, region.H/paneHeight)
	k := min(len(panes), cols*rows)
	start := len(panes) - k
	if focused >= 0 && focused < start {
		start = focused
	}
	shown := panes[start : start+k]
	used := (k + cols - 1) / cols
	y := region.Y
	for r := range used {
		rh := region.H / used
		if r < region.H%used {
			rh++
		}
		in := shown[r*cols : min((r+1)*cols, k)]
		x := region.X
		for i, id := range in {
			pw := region.W / len(in)
			if i < region.W%len(in) {
				pw++
			}
			out = append(out, ui.Rect{Pane: id, X: x, Y: y, W: pw, H: rh})
			x += pw
		}
		y += rh
	}
	return out
}
