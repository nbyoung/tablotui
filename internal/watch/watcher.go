package watch

import (
	"context"
	"sync"
	"time"
)

// Clock is the time the watcher sleeps on. A test supplies its own.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// systemClock is the clock of a watcher made by New.
type systemClock struct{}

func (systemClock) Now() time.Time                         { return time.Now() }
func (systemClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// Watcher polls one source. The zero value is not usable; call New.
type Watcher struct {
	Source    Source
	Interval  time.Duration
	Settle    time.Duration
	MaxSettle time.Duration
	Clock     Clock

	poke chan struct{} // capacity 1: a pending poke
	run  runner        // the git runner, which a test in the package wraps to count processes

	mu   sync.Mutex
	mods []string // the submodule checkouts to poll, as SetSubmodules last set them
}

// New returns a watcher with the default timings and the system clock.
func New(src Source) *Watcher {
	return &Watcher{
		Source:    src,
		Interval:  DefaultInterval,
		Settle:    DefaultSettle,
		MaxSettle: DefaultMaxSettle,
		Clock:     systemClock{},
		poke:      make(chan struct{}, 1),
		run:       runGit,
	}
}

// SetSubmodules names the submodule checkouts that a live watcher polls: the
// ones the Loader reports it read on disk, as paths relative to Source.Dir or
// absolute. Each poll then asks "git rev-parse HEAD" in each one, so a moved
// checkout is a change. The caller sets the list after each load; it may call
// SetSubmodules while a Next waits. A source at a commit ignores the list,
// since a pin is then the gitlink in that commit's tree.
func (w *Watcher) SetSubmodules(dirs []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.mods = append([]string(nil), dirs...)
}

// submodules returns a copy of the list that SetSubmodules last set.
func (w *Watcher) submodules() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.mods...)
}

// Poke makes the Next that waits now, or the next one called, return the
// current state at once, changed or not. It never blocks; pokes do not queue.
func (w *Watcher) Poke() {
	select {
	case w.poke <- struct{}{}:
	default:
	}
}

// Next blocks until the state differs from prev and has settled, or until a
// poke, and returns it. It returns prev and the context's error when cancelled.
func (w *Watcher) Next(ctx context.Context, prev State) (State, error) {
	for {
		// 1. A poke that waits returns the current state at once.
		select {
		case <-w.poke:
			cur := w.Snapshot(ctx)
			if err := ctx.Err(); err != nil {
				return prev, err
			}
			return cur, nil
		default:
		}
		// 2. Read the state; an unchanged one goes straight to the wait.
		cur := w.Snapshot(ctx)
		if err := ctx.Err(); err != nil {
			return prev, err
		}
		if cur != prev {
			// 3. Settle: read until two reads agree or MaxSettle runs out.
			for waited := time.Duration(0); ; {
				if err := w.sleep(ctx, w.Settle); err != nil {
					return prev, err
				}
				waited += w.Settle
				next := w.Snapshot(ctx)
				if err := ctx.Err(); err != nil {
					return prev, err
				}
				agreed := next == cur
				cur = next
				if agreed || waited >= w.MaxSettle {
					break
				}
			}
			// 4. A change that reverted while it settled reports nothing.
			if cur != prev {
				return cur, nil
			}
		}
		// 5. Wait for the context's end, a poke or the next interval.
		timer := w.Clock.After(w.Interval)
		select {
		case <-ctx.Done():
			return prev, ctx.Err()
		case <-w.poke:
			// Put the poke back for step 1, which takes it.
			w.Poke()
		case <-timer:
		}
	}
}

// sleep waits d on the clock or until the context ends.
func (w *Watcher) sleep(ctx context.Context, d time.Duration) error {
	timer := w.Clock.After(d)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer:
		return nil
	}
}
