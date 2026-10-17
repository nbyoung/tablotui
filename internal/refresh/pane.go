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
// load it changes nothing the screen shows (679b A9, 171b A4). As a
// ui.Reporter it names the source at the end of the status line and states
// the component's notice on a line of its own, which the next good load
// clears (171b A5). On ForceMsg it reloads (A6). It implements ui.Pane,
// draws nothing and binds no key: Command binds the Force key.
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

// ForceMsg asks the pane to reload now, or as soon as the running load ends.
type ForceMsg struct{}

// Command binds ctrl+r to the reload, which the component's Force runs: the
// key is the frame's to name (171b A6), and r is an action's (518e).
func Command() ui.Command {
	return ui.Command{
		Binding: key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "reload")),
		Run:     func(ui.SelectionMsg) tea.Cmd { return func() tea.Msg { return ForceMsg{} } },
	}
}

// Update implements ui.Pane.
func (p Pane[T]) Update(msg tea.Msg, _ ui.Context) (ui.Pane, tea.Cmd) {
	if _, ok := msg.(ForceMsg); ok {
		p.m.Force()
		return p, nil
	}
	var cmd tea.Cmd
	p.m, cmd = p.m.Update(msg)
	ld, ok := msg.(Loaded[T])
	if !ok || ld.Err != nil {
		return p, cmd
	}
	if p.store != nil {
		p.store(ld.Data)
	}
	return p, tea.Batch(cmd, func() tea.Msg { return ui.ReloadMsg{} })
}

// Where implements ui.Reporter: the source in one phrase, Status().Where().
func (p Pane[T]) Where() string { return p.m.Status().Where() }

// Notice implements ui.Reporter: Status().Notice(), an error while a load
// fails.
func (p Pane[T]) Notice() ui.NoticeMsg {
	st := p.m.Status()
	return ui.NoticeMsg{Text: st.Notice(), Err: st.Err != nil}
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
