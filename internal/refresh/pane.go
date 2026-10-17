package refresh

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/nbyoung/tablotui/internal/ui"
)

// PaneID is the identifier of the pane that embeds the component.
const PaneID = "refresh"

// Pane embeds the component in the frame as a pane that never opens, as the
// role pane does (f394 decision 3): the frame initialises it with the other
// panes and broadcasts it every message that is not a key, so the component's
// own messages reach it. On a good load it hands the data to the store and
// sends ui.ReloadMsg, on which each pane derives its view again; on a failed
// load it changes nothing the screen shows and sends the component's notice
// as an error (679b A9, 171b A4), which the next good load clears. It
// implements ui.Pane, draws nothing and binds no key.
type Pane[T any] struct {
	m     Model[T]
	store func(T)
}

// NewPane returns the pane over m. store receives each good load's data, on
// the event loop, before the ReloadMsg that follows it.
func NewPane[T any](m Model[T], store func(T)) Pane[T] {
	return Pane[T]{m: m, store: store}
}

// ID implements ui.Pane.
func (p Pane[T]) ID() string { return PaneID }

// Init implements ui.Pane: the component reads the first state.
func (p Pane[T]) Init(ui.Context) tea.Cmd { return p.m.Init() }

// Update implements ui.Pane.
func (p Pane[T]) Update(msg tea.Msg, _ ui.Context) (ui.Pane, tea.Cmd) {
	failed := p.m.Status().Err != nil
	var cmd tea.Cmd
	p.m, cmd = p.m.Update(msg)
	ld, ok := msg.(Loaded[T])
	switch {
	case !ok:
		return p, cmd
	case ld.Err != nil:
		text := p.m.Status().Notice()
		return p, tea.Batch(cmd, func() tea.Msg { return ui.NoticeMsg{Text: text, Err: true} })
	}
	if p.store != nil {
		p.store(ld.Data)
	}
	cmds := []tea.Cmd{cmd, func() tea.Msg { return ui.ReloadMsg{} }}
	if failed {
		cmds = append(cmds, func() tea.Msg { return ui.NoticeMsg{} }) // the failure is over
	}
	return p, tea.Batch(cmds...)
}

// View implements ui.Pane: h blank lines, since the pane never opens.
func (p Pane[T]) View(w, h int, _ ui.Context) string {
	lines := make([]string, max(h, 0))
	for i := range lines {
		lines[i] = strings.Repeat(" ", max(w, 0))
	}
	return strings.Join(lines, "\n")
}

// Status implements ui.Pane.
func (p Pane[T]) Status(ui.Context) string { return "" }

// Keys implements ui.Pane: none.
func (p Pane[T]) Keys() []key.Binding { return nil }
