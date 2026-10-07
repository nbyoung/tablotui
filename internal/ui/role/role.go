// Package role places the person at the keyboard: it asks which roles the
// viewer holds, arrives the grid where the first of them works, and moves it
// on when the viewer switches the role. It resolves no role itself.
package role

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/ui/grid"
	"github.com/nbyoung/tablotui/internal/view"
)

// ID is the identifier of the role pane. The pane never opens.
const ID = "role"

// The roles, as tablo names them.
const (
	Owner       = "owner"
	Authority   = "authority"
	Assignee    = "assignee"
	Contributor = "contributor"
	Agent       = "agent"
	Reviewer    = "reviewer"
	Observer    = "observer"
)

// naming is the order in which a stop takes its name and the cycle its order.
var naming = []string{Owner, Assignee, Contributor, Agent, Authority, Reviewer, Observer}

// place is where a role arrives.
type place int

const (
	whole  place = iota // the global tableau at glance
	corner              // the contextual tableau, person form
	marked              // the global tableau with the viewer's cells marked
)

func placeOf(role string) place {
	switch role {
	case Assignee, Contributor, Agent, Authority:
		return corner
	case Reviewer:
		return marked
	}
	return whole
}

// Stops returns the roles that R cycles through: of the roles held and
// observer, in the naming order, the first that leads to each place.
func Stops(roles []string) []string {
	var out []string
	seen := map[place]bool{}
	for _, r := range naming {
		if r != Observer && !slices.Contains(roles, r) {
			continue
		}
		if p := placeOf(r); !seen[p] {
			seen[p] = true
			out = append(out, r)
		}
	}
	return out
}

// Arrival is where the viewer starts in a role. An unknown role, and "",
// arrive as an observer does.
func Arrival(role, viewer, ref string) grid.Start {
	s := grid.Start{Request: source.Request{
		View: "tableau", Ref: ref, Window: 1, Level: "detail", Viewer: viewer, Role: role,
	}}
	switch placeOf(role) {
	case corner:
		s.Request.View = "context"
		s.Request.Person = viewer
		s.Level = grid.Detail
		s.Mine = true
	case marked:
		s.Request.Person = viewer
		s.Mine = true
	}
	return s
}

// SwitchMsg asks the role pane for the next stop.
type SwitchMsg struct{}

// Command binds R to the switch of role.
func Command() ui.Command {
	return ui.Command{
		Binding: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "role")),
		Run:     func(ui.SelectionMsg) tea.Cmd { return func() tea.Msg { return SwitchMsg{} } },
	}
}

// Options configures the role pane. An empty Viewer is nobody: an observer.
type Options struct {
	Source source.Viewers
	Viewer string
	Ref    string
}

// Pane holds the viewer, the stops and the stop in force. It implements
// ui.Pane, draws nothing and never opens.
type Pane struct {
	src      source.Viewers
	viewer   string
	ref      string
	seq      uint64
	stops    []string // nil until tablo names the roles
	at       int
	arrived  bool   // the grid has a start
	selected string // the task the grid selects
}

// New builds the role pane.
func New(o Options) Pane {
	p := Pane{src: o.Source, viewer: o.Viewer, ref: o.Ref, seq: 1}
	if p.viewer == "" {
		p.stops = []string{Observer}
		p.arrived = true
	}
	return p
}

// ID implements ui.Pane.
func (p Pane) ID() string { return ID }

// Keys implements ui.Pane: the pane has none.
func (p Pane) Keys() []key.Binding { return nil }

// Status implements ui.Pane.
func (p Pane) Status(ui.Context) string { return "" }

// View implements ui.Pane: h blank lines of w cells.
func (p Pane) View(w, h int, _ ui.Context) string {
	lines := make([]string, max(h, 0))
	for i := range lines {
		lines[i] = strings.Repeat(" ", max(w, 0))
	}
	return strings.Join(lines, "\n")
}

// Role returns the role in force, or "" before tablo names the roles.
func (p Pane) Role() string {
	if len(p.stops) == 0 {
		return ""
	}
	return p.stops[p.at]
}

// Init implements ui.Pane: nobody arrives as an observer at once; anyone else
// asks for the roles.
func (p Pane) Init(c ui.Context) tea.Cmd {
	if p.viewer == "" {
		return tea.Batch(
			start(Arrival(Observer, "", p.ref)),
			notice("no Git identity: reading as an observer; pass --as <email>", false),
		)
	}
	return p.ask(c)
}

func (p Pane) ask(c ui.Context) tea.Cmd {
	src, ref, email := p.src, p.ref, p.viewer
	return c.Load(ID, p.seq, func(ctx context.Context) (any, error) {
		if src == nil {
			return nil, fmt.Errorf("no source")
		}
		return src.Viewer(ctx, ref, email)
	})
}

// start arrives the grid and tells the panes whom they read for and as which
// role: the two travel together, so a pane never lags behind the grid.
func start(s grid.Start) tea.Cmd {
	reading := source.Reading{Viewer: s.Request.Viewer, Role: s.Request.Role}
	return tea.Batch(
		func() tea.Msg { return grid.StartMsg{Start: s} },
		func() tea.Msg { return reading },
	)
}

func notice(text string, isErr bool) tea.Cmd {
	return func() tea.Msg { return ui.NoticeMsg{Text: text, Err: isErr} }
}

// Update implements ui.Pane.
func (p Pane) Update(msg tea.Msg, c ui.Context) (ui.Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.SelectionMsg:
		p.selected = msg.Task
	case ui.ReloadMsg:
		if p.viewer != "" {
			p.seq++
			return p, p.ask(c)
		}
	case SwitchMsg:
		return p.next(c)
	case ui.LoadedMsg:
		if msg.Pane == ID && msg.Seq == p.seq {
			return p.loaded(msg)
		}
	}
	return p, nil
}

// next moves to the next stop.
func (p Pane) next(c ui.Context) (ui.Pane, tea.Cmd) {
	switch {
	case p.stops == nil:
		p.seq++
		return p, tea.Batch(p.ask(c), notice("the roles have not loaded; asking again", false))
	case len(p.stops) == 1:
		return p, notice("one role only: "+p.stops[0], false)
	}
	p.at = (p.at + 1) % len(p.stops)
	return p, tea.Batch(p.arrive(), notice(fmt.Sprintf("reading as %s, %d of %d", p.stops[p.at], p.at+1, len(p.stops)), false))
}

// arrive starts the grid at the stop in force and keeps the selection.
func (p Pane) arrive() tea.Cmd {
	s := Arrival(p.Role(), p.viewer, p.ref)
	s.Select = p.selected
	return start(s)
}

// loaded takes tablo's answer about the viewer.
func (p Pane) loaded(msg ui.LoadedMsg) (ui.Pane, tea.Cmd) {
	v, ok := msg.Value.(view.Viewer)
	err := msg.Err
	if err == nil && !ok {
		err = fmt.Errorf("unexpected data of type %T", msg.Value)
	}
	if err != nil {
		if p.arrived {
			return p, nil
		}
		p.arrived = true
		return p, tea.Batch(p.arrive(), notice("roles: "+err.Error(), true))
	}
	stops := Stops(v.Roles)
	if p.stops == nil {
		p.stops, p.at, p.arrived = stops, 0, true
		return p, p.arrive()
	}
	was := p.stops[p.at]
	p.stops = stops
	if i := slices.Index(stops, was); i >= 0 {
		p.at = i
		return p, nil
	}
	p.at = 0
	return p, tea.Batch(p.arrive(), notice(fmt.Sprintf("the role %s no longer holds: reading as %s", was, stops[0]), false))
}
