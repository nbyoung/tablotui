// Package refresh joins a watcher to a Bubble Tea frame: one load per settled
// change, never two at once, and the last good view kept when a load fails.
package refresh

import (
	"context"
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nbyoung/tablotui/internal/watch"
)

// Watcher is what the component needs of *watch.Watcher. A test scripts it.
type Watcher interface {
	Snapshot(ctx context.Context) watch.State
	Next(ctx context.Context, prev watch.State) (watch.State, error)
	Poke()
}

// Loader loads the view data for one settled state. The frame supplies it. It
// runs in a command's goroutine, and st.Err is always empty when it runs.
type Loader[T any] func(ctx context.Context, st watch.State) (T, error)

// Loaded is the one message the frame handles. It carries the result of one load.
type Loaded[T any] struct {
	State watch.State // the state the load read
	Data  T           // the view data; the zero value when Err is set
	Err   error       // why the load failed; the frame then keeps what it shows
}

// changed reports a settled state that the component loads next.
type changed struct{ state watch.State }

// Model is the component. It is a value, as a Bubbles component is.
type Model[T any] struct {
	ctx    context.Context
	w      Watcher
	load   Loader[T]
	now    func() time.Time
	status Status
}

// New returns the component. now is the clock's Now.
func New[T any](ctx context.Context, src watch.Source, w Watcher, now func() time.Time, load Loader[T]) Model[T] {
	return Model[T]{ctx: ctx, w: w, load: load, now: now, status: Status{Source: src}}
}

// Init returns the command that reads the first state.
func (m Model[T]) Init() tea.Cmd {
	return func() tea.Msg {
		st := m.w.Snapshot(m.ctx)
		if m.ctx.Err() != nil {
			return nil
		}
		return changed{st}
	}
}

// Update handles the component's own messages and ignores every other. A
// change starts one load; a finished load starts the wait for the next change,
// so exactly one command is outstanding at any time.
func (m Model[T]) Update(msg tea.Msg) (Model[T], tea.Cmd) {
	switch msg := msg.(type) {
	case changed:
		m.status.Loading = true
		return m, m.loadCmd(msg.state)
	case Loaded[T]:
		m.status.Loading = false
		if msg.Err != nil {
			if m.status.Err == nil {
				m.status.Failed = m.now()
			}
			m.status.Err = msg.Err
		} else {
			m.status.Err = nil
			m.status.Failed = time.Time{}
			m.status.Good = true
			m.status.State = msg.State
			m.status.Loaded = m.now()
		}
		return m, m.waitCmd(msg.State)
	}
	return m, nil
}

// Force reloads now, or as soon as the running load ends. It calls Poke.
func (m Model[T]) Force() { m.w.Poke() }

// Status returns the status.
func (m Model[T]) Status() Status { return m.status }

// loadCmd returns the command that loads st. A state with an error never
// reaches the Loader: the command turns the error into a failed load.
func (m Model[T]) loadCmd(st watch.State) tea.Cmd {
	return func() tea.Msg {
		if m.ctx.Err() != nil {
			return nil
		}
		if st.Err != "" {
			return Loaded[T]{State: st, Err: errors.New(st.Err)}
		}
		data, err := m.load(m.ctx, st)
		if m.ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return Loaded[T]{State: st, Err: err}
		}
		return Loaded[T]{State: st, Data: data}
	}
}

// waitCmd returns the command that waits for a change from st, the state the
// last load read, so that a failure does not reload in a loop.
func (m Model[T]) waitCmd(st watch.State) tea.Cmd {
	return func() tea.Msg {
		if m.ctx.Err() != nil {
			return nil
		}
		next, err := m.w.Next(m.ctx, st)
		if err != nil {
			return nil
		}
		return changed{next}
	}
}
