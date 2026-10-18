// Package act holds the four actions of the tablotui program: record a
// status, review a gate, reaffirm and authorise. Each is a ui.Command that
// plans one commit through tabloio's action package, shows it in a ui.Mode
// and makes it on confirmation.
package act

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/nbyoung/tabloio/action"

	"github.com/nbyoung/tablotui/internal/ui"
)

// Repo is what the actions need of *action.Repo, which satisfies it.
type Repo interface {
	Record(ctx context.Context, rec action.Recording, o action.Options) (*action.Plan, error)
	Review(ctx context.Context, ids []string, gate string, o action.Options) (*action.Plan, error)
	Reaffirm(ctx context.Context, ids []string, o action.Options) (*action.Plan, error)
	Authorise(ctx context.Context, ids []string, o action.Options) (*action.Plan, error)
	Apply(ctx context.Context, p *action.Plan) (action.Result, error)
}

// Facts is what the record form needs of an action.Source, which satisfies it.
type Facts interface {
	Project(ctx context.Context) (action.Project, error)
	Task(ctx context.Context, id string) (action.Task, error)
}

// Options configures the actions.
type Options struct {
	Repo   Repo   // nil: no action runs, and each key says so
	Facts  Facts  // the facts behind the record form; required with Repo
	Dir    string // the project directory, the one that holds .tableaux
	Ref    string // the --ref in view; when not empty nothing writes
	Viewer string // the email the view is drawn for; "" when the command names none
	// Context ends the plans when the program ends; nil is context.Background.
	Context context.Context
	// Uncommitted lists the paths under dir/.tableaux, relative to dir, that
	// differ from HEAD or that Git does not track; nil is GitUncommitted.
	Uncommitted func(ctx context.Context, dir string) ([]string, error)
	// Poke asks the watcher for a load now; the actions call it after every
	// Apply. nil does nothing.
	Poke func()
}

// Actions is the four commands and the commits they have in flight.
type Actions struct {
	o      Options
	ctx    context.Context
	wg     sync.WaitGroup
	flying atomic.Int32
}

// New builds the actions.
func New(o Options) *Actions {
	a := &Actions{o: o, ctx: o.Context}
	if a.ctx == nil {
		a.ctx = context.Background()
	}
	if a.o.Uncommitted == nil {
		a.o.Uncommitted = GitUncommitted
	}
	return a
}

// kind is one of the four actions.
type kind int

const (
	record kind = iota
	review
	reaffirm
	authorise
)

// String returns the package's command name of the action.
func (k kind) String() string {
	return [...]string{"record", "review", "reaffirm", "authorise"}[k]
}

// Commands returns the four commands in the order r, v, a, u.
func (a *Actions) Commands() []ui.Command {
	bind := func(k string, help string, act kind) ui.Command {
		return ui.Command{
			Binding: key.NewBinding(key.WithKeys(k), key.WithHelp(k, help)),
			Run:     func(sel ui.SelectionMsg) tea.Cmd { return a.start(act, sel) },
		}
	}
	return []ui.Command{
		bind("r", "record a status", record),
		bind("v", "review the gate", review),
		bind("a", "reaffirm the status", reaffirm),
		bind("u", "authorise the task", authorise),
	}
}

// Busy reports whether a commit is in flight.
func (a *Actions) Busy() bool { return a.flying.Load() > 0 }

// Wait blocks until no commit is in flight. The command calls it after the
// program ends, so that a quit never cuts a commit in two.
func (a *Actions) Wait() { a.wg.Wait() }

// The messages the plan and the commit answer with. A mode handles one only
// when to is the mode itself.
type (
	plannedMsg struct {
		to   *mode
		plan *action.Plan
		err  error
	}
	appliedMsg struct {
		to  *mode
		res action.Result
		err error
	}
)

// notice answers with one line on the status line.
func notice(text string, isErr bool) tea.Cmd {
	return func() tea.Msg { return ui.NoticeMsg{Text: text, Err: isErr} }
}

// start is the command of a key: the checks before a plan, in order, then
// the facts of the form for record or the plan for the other three. It
// answers with exactly one message: a notice, or a mode to open.
func (a *Actions) start(k kind, sel ui.SelectionMsg) tea.Cmd {
	switch {
	case a.o.Repo == nil:
		return notice("no action: this build draws fixtures", false)
	case a.o.Ref != "":
		return notice("--ref "+a.o.Ref+" shows a commit; nothing writes there", false)
	case sel.Task == "":
		return notice("select a task first", false)
	case k == review && sel.Gate == "":
		return notice("move the cursor to the gate you review", false)
	}
	return func() tea.Msg {
		ctx := a.ctx
		id := sel.Task
		paths, err := a.o.Uncommitted(ctx, a.o.Dir)
		if err != nil {
			return ui.NoticeMsg{Text: firstLine(err.Error()), Err: true}
		}
		for _, f := range []string{"status/" + id + ".yaml", "tasks/" + id + ".yaml"} {
			if slices.Contains(paths, ".tableaux/"+f) {
				return ui.NoticeMsg{Text: f + " holds an uncommitted edit; commit or restore it"}
			}
		}
		m := &mode{a: a, kind: k, id: id, uncommitted: len(paths) > 0}
		m.name = k.String() + " " + id
		if k == review {
			m.gate = sel.Gate
			m.name += " " + sel.Gate
		}
		if k == record {
			if a.o.Facts == nil {
				return ui.NoticeMsg{Text: "no facts for the record form", Err: true}
			}
			project, err := a.o.Facts.Project(ctx)
			if err != nil {
				return ui.NoticeMsg{Text: firstLine(err.Error()), Err: true}
			}
			task, err := a.o.Facts.Task(ctx, id)
			if err != nil {
				return ui.NoticeMsg{Text: firstLine(err.Error()), Err: true}
			}
			if len(project.Gates) == 0 {
				return ui.NoticeMsg{Text: "the project states no gate", Err: true}
			}
			if len(task.Children) > 0 {
				return ui.NoticeMsg{Text: id + " is a parent; its children hold the status"}
			}
			m.form = newForm(project, task, sel.Gate)
			m.stage = stageForm
			return ui.OpenModeMsg{Mode: m}
		}
		var p *action.Plan
		switch k {
		case review:
			p, err = a.o.Repo.Review(ctx, []string{id}, sel.Gate, action.Options{})
		case reaffirm:
			p, err = a.o.Repo.Reaffirm(ctx, []string{id}, action.Options{})
		case authorise:
			p, err = a.o.Repo.Authorise(ctx, []string{id}, action.Options{})
		}
		if err != nil {
			m.stopPlan(err)
		} else {
			m.confirm(p)
		}
		return ui.OpenModeMsg{Mode: m}
	}
}

// plan is the command of enter in the form: Record with the form's request.
func (a *Actions) plan(m *mode, rec action.Recording) tea.Cmd {
	return func() tea.Msg {
		p, err := a.o.Repo.Record(a.ctx, rec, action.Options{})
		return plannedMsg{to: m, plan: p, err: err}
	}
}

// apply is the command of y: it makes the commit under a context the end of
// the program does not cancel, counts itself while it runs, pokes the
// watcher, and answers with the reload, the notice and the outcome.
func (a *Actions) apply(m *mode, p *action.Plan) tea.Cmd {
	return func() tea.Msg {
		a.wg.Add(1)
		a.flying.Add(1)
		res, err := func() (action.Result, error) {
			defer func() {
				a.flying.Add(-1)
				a.wg.Done()
			}()
			return a.o.Repo.Apply(context.WithoutCancel(a.ctx), p)
		}()
		if a.o.Poke != nil {
			a.o.Poke()
		}
		var n ui.NoticeMsg
		switch {
		case err == nil:
			n = ui.NoticeMsg{Text: "committed " + short(res.Commit) + " " + p.Subject}
		case res.Commit != "":
			n = ui.NoticeMsg{Text: "committed " + short(res.Commit) + ", and a hook altered the commit", Err: true}
		default:
			n = ui.NoticeMsg{Text: "not committed: " + errLines(err)[0], Err: true}
		}
		return tea.BatchMsg{
			func() tea.Msg { return ui.ReloadMsg{} },
			func() tea.Msg { return n },
			func() tea.Msg { return appliedMsg{to: m, res: res, err: err} },
		}
	}
}

// short returns the first seven digits of a hash.
func short(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// refused reports whether err holds a refusal of the package.
func refused(err error) bool {
	var rs action.Refusals
	var r action.Refusal
	return errors.As(err, &rs) || errors.As(err, &r)
}

// errLines returns the lines that state an error: one per refusal; the tool,
// its subcommand and its exit, then each line of its standard error, for a
// tool that fails; else the lines of its text. It returns one line at least.
func errLines(err error) []string {
	var out []string
	var rs action.Refusals
	var r action.Refusal
	var x *action.ExecError
	switch {
	case errors.As(err, &rs):
		for _, r := range rs {
			out = append(out, r.Error())
		}
	case errors.As(err, &r):
		out = append(out, r.Error())
	case errors.As(err, &x):
		head := x.Tool
		if len(x.Args) > 0 {
			head += " " + x.Args[0]
		}
		if x.Exit >= 0 {
			head += fmt.Sprintf(": exit %d", x.Exit)
		}
		out = append(out, head)
		if s := strings.TrimRight(x.Stderr, "\n"); s != "" {
			out = append(out, strings.Split(s, "\n")...)
		}
	default:
		out = strings.Split(strings.TrimRight(err.Error(), "\n"), "\n")
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}
