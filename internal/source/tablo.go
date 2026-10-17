package source

import (
	"bytes"
	"context"
	"errors"
	"sync"

	"github.com/nbyoung/tablo"

	"github.com/nbyoung/tablotui/internal/view"
	"github.com/nbyoung/tablotui/internal/view/list"
	"github.com/nbyoung/tablotui/internal/watch"
)

// Poller is what the adapter asks of the watcher: the submodule checkouts it
// polls beside the working tree. *watch.Watcher implements it.
type Poller interface{ SetSubmodules(dirs []string) }

// Tablo is the source over tablo's library, the one file of the program that
// imports it for view data; internal/identity imports it for the viewer's
// rule alone. It holds one tablo.Repository, opened once. Load is the Loader
// of the live refresh (171b): it reads one snapshot per settled change. The
// frame hands each good snapshot to Store, and every view derives from the
// stored snapshot in process, so a request names no ref. A call made before
// the first Store waits for it (f394 A9). Tablo implements Tableaux, Lists
// and Viewers, and is safe for concurrent use.
type Tablo struct {
	repo *tablo.Repository
	ref  string // the revision --ref names; "" reads the working tree
	poll Poller // nil polls nothing

	mu    sync.Mutex
	snap  *tablo.Snapshot
	ready chan struct{} // closed by the first Store
}

// NewTablo opens the project whose .tableaux lies in dir, or nearest above
// it, read at ref, or from the working tree when ref is "". After each load
// it names the checkouts the load read to poll, which may be nil. The error
// is a *tablo.ReadError.
func NewTablo(ctx context.Context, dir, ref string, poll Poller) (*Tablo, error) {
	repo, err := tablo.Open(ctx, dir, tablo.Options{})
	if err != nil {
		return nil, err
	}
	return &Tablo{repo: repo, ref: ref, poll: poll, ready: make(chan struct{})}, nil
}

// Load loads the snapshot of one settled state: the commit st names when the
// source is a ref, which it never resolves again, or the working tree. It
// fails exactly when tablo gives no view data: for a ref or a repository it
// cannot read, and for a missing or unaccepted version, an unusable
// gates.yaml or a broken tree, with the first error diagnostic. Every other
// error diagnostic leaves the snapshot good (679b decision 16).
func (t *Tablo) Load(ctx context.Context, st watch.State) (*tablo.Snapshot, error) {
	at := ""
	if t.ref != "" {
		at = st.Commit
		if at == "" {
			at = t.ref
		}
	}
	snap, err := t.repo.Load(ctx, at)
	if err != nil {
		return nil, err
	}
	if t.poll != nil {
		t.poll.SetSubmodules(snap.Checkouts())
	}
	env, err := snap.View(ctx, tablo.Gates, tablo.Params{})
	if err != nil {
		return nil, err
	}
	if env.Data == nil {
		return nil, noView(env.Diagnostics)
	}
	return snap, nil
}

// Store makes s the snapshot every later call derives from.
func (t *Tablo) Store(s *tablo.Snapshot) {
	if s == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.snap == nil {
		close(t.ready)
	}
	t.snap = s
}

// snapshot returns the stored snapshot, and waits for the first.
func (t *Tablo) snapshot(ctx context.Context) (*tablo.Snapshot, error) {
	select {
	case <-t.ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.snap, nil
}

// view derives one view of the stored snapshot and returns its envelope as
// JSON, with the ref --ref names in place of the commit it loaded.
func (t *Tablo) view(ctx context.Context, v tablo.View, p tablo.Params) ([]byte, []tablo.Diagnostic, error) {
	snap, err := t.snapshot(ctx)
	if err != nil {
		return nil, nil, err
	}
	env, err := snap.View(ctx, v, p.Narrow(v))
	if err != nil {
		return nil, nil, err
	}
	if env.Data == nil {
		return nil, nil, noView(env.Diagnostics)
	}
	if t.ref != "" && env.Source != nil {
		env.Source.Ref = t.ref
	}
	return env.JSON(), env.Diagnostics, nil
}

// Tableau implements Tableaux. Request maps to tablo's parameters field by
// field: Viewer, Role, Level, Person, Task, Window and Historical; View names
// the view, "tableau" when it is empty; Ref is the stored snapshot's. An
// error diagnostic of the project comes beside the view as a
// *DiagnosticError.
func (t *Tablo) Tableau(ctx context.Context, r Request) (view.Tableau, error) {
	v, p := tableauOf(r)
	b, ds, err := t.view(ctx, v, p)
	if err != nil {
		return view.Tableau{}, err
	}
	tb, err := view.Decode(bytes.NewReader(b))
	if err != nil {
		return view.Tableau{}, err
	}
	if e := diagnosed(ds); e != nil {
		return tb, e
	}
	return tb, nil
}

// tableauOf maps a request to the view and tablo's parameters.
func tableauOf(r Request) (tablo.View, tablo.Params) {
	v := tablo.Tableau
	if r.View != "" {
		v = tablo.View(r.View)
	}
	window := r.Window
	return v, tablo.Params{
		Viewer: r.Viewer, Role: tablo.Role(r.Role), Level: tablo.Level(r.Level),
		Person: r.Person, Task: r.Task, Window: &window, Historical: r.Historical,
	}
}

// list derives one list view into v. ListRequest maps to tablo's parameters
// field by field: Viewer, Role, Level and Task. The queue's person is the
// viewer, as tablo resolves it, and with no viewer the queue is ErrNoPerson.
func (t *Tablo) list(ctx context.Context, name tablo.View, r ListRequest, v list.View) error {
	if name == tablo.Queue && r.Viewer == "" {
		if _, err := t.snapshot(ctx); err != nil {
			return err
		}
		return ErrNoPerson
	}
	b, _, err := t.view(ctx, name, tablo.Params{
		Viewer: r.Viewer, Role: tablo.Role(r.Role), Level: tablo.Level(r.Level), Task: r.Task,
	})
	if err != nil {
		return err
	}
	return list.Decode(bytes.NewReader(b), v)
}

// Task implements Lists.
func (t *Tablo) Task(ctx context.Context, r ListRequest) (list.Task, error) {
	var v list.Task
	err := t.list(ctx, tablo.Task, r, &v)
	return v, err
}

// Queue implements Lists.
func (t *Tablo) Queue(ctx context.Context, r ListRequest) (list.Queue, error) {
	var v list.Queue
	err := t.list(ctx, tablo.Queue, r, &v)
	return v, err
}

// Blockage implements Lists.
func (t *Tablo) Blockage(ctx context.Context, r ListRequest) (list.Blockage, error) {
	var v list.Blockage
	err := t.list(ctx, tablo.Blockage, r, &v)
	return v, err
}

// History implements Lists.
func (t *Tablo) History(ctx context.Context, r ListRequest) (list.History, error) {
	var v list.History
	err := t.list(ctx, tablo.History, r, &v)
	return v, err
}

// Viewer implements Viewers: the roles the email holds anywhere in the
// project of the stored snapshot, whose ref the call's ref repeats.
func (t *Tablo) Viewer(ctx context.Context, _, email string) (view.Viewer, error) {
	snap, err := t.snapshot(ctx)
	if err != nil {
		return view.Viewer{}, err
	}
	tv := snap.Viewer(email)
	v := view.Viewer{}
	if tv.Email != nil {
		v.Email = *tv.Email
	}
	for _, r := range tv.Roles {
		v.Roles = append(v.Roles, string(r))
	}
	return v, nil
}

// errorsOf returns the error diagnostics of ds, in tablo's order.
func errorsOf(ds []tablo.Diagnostic) []Diagnostic {
	var out []Diagnostic
	for _, d := range ds {
		if d.Severity == "error" {
			out = append(out, Diagnostic{Severity: d.Severity, Code: d.Code, Path: d.Path, Line: d.Line, Col: d.Col, Message: d.Message})
		}
	}
	return out
}

// diagnosed returns the error diagnostics of ds as a *DiagnosticError, or
// nil when ds holds none.
func diagnosed(ds []tablo.Diagnostic) error {
	if es := errorsOf(ds); len(es) > 0 {
		return &DiagnosticError{Diagnostics: es}
	}
	return nil
}

// noView is the error of a view tablo cannot derive: the first error
// diagnostic, as "path:line:col: code message" (171b A7). It is no
// *DiagnosticError, since no view stands beside it.
func noView(ds []tablo.Diagnostic) error {
	return errors.New((&DiagnosticError{Diagnostics: errorsOf(ds)}).Error())
}
