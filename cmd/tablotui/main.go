// Command tablotui is the Tableaux terminal front end.
//
//	tablotui [-C <dir>] [--ref <ref>] [--as <email>] [--settings <file>] [--fixture <dir>] [--version] [--help]
//
// It draws the tableau as a full-screen grid, and beside it the panes for the
// task, the work queue, the blockage tree and the history, which the keys 1 to
// 4 open. It starts where the person at the keyboard works: --as names that
// person, by default the email Git commits with, and R switches the role. It
// reads the project through tablo's library, from the working tree or at the
// commit --ref names, and follows the repository as it changes, which ctrl+r
// reloads at once; --fixture draws from fixture files instead. The keys r, v,
// a and u record a status, review a gate, reaffirm and authorise the selected
// task, each through the commit tabloio's action package plans.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/nbyoung/tabloio/action"

	"github.com/nbyoung/tablotui/internal/identity"
	"github.com/nbyoung/tablotui/internal/refresh"
	"github.com/nbyoung/tablotui/internal/settings"
	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/ui/act"
	"github.com/nbyoung/tablotui/internal/ui/detail"
	"github.com/nbyoung/tablotui/internal/ui/grid"
	"github.com/nbyoung/tablotui/internal/ui/role"
	"github.com/nbyoung/tablotui/internal/watch"
)

// version is the release version. GoReleaser sets it through
// -ldflags "-X main.version=...". A build from source leaves it at "dev".
var version = "dev"

// The exit codes. A repository that cannot be read at the start exits 3, as
// tablo's plumbing does (171b).
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
	exitRead    = 3
)

// runProgram runs the program until the viewer quits. The tests replace it,
// so that none starts a terminal.
var runProgram = func(ctx context.Context, m tea.Model) error {
	_, err := tea.NewProgram(m, tea.WithContext(ctx)).Run()
	return err
}

// newActions builds the four actions. The tests replace it, so that the
// actions plan through a Repo of the test.
var newActions = act.New

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses the arguments, wires the source, the panes and the frame, and
// returns the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tablotui", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("C", ".", "the project directory")
	ref := fs.String("ref", "", "read the commit this revision names, and follow the name (default: the working tree on HEAD)")
	as := fs.String("as", "", "the email of the person at the keyboard (default: the Git identity)")
	settingsPath := fs.String("settings", "", "the settings file (default: tablotui/settings.json in the user configuration directory)")
	fixtures := fs.String("fixture", "", "draw from the tableau and list files in this directory")
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := checkHyphens(args, fs); err != nil {
		_, _ = fmt.Fprintf(stderr, "tablotui: %v\n", err)
		usage(fs, stderr)
		return exitUsage
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			usage(fs, stdout)
			return exitOK
		}
		_, _ = fmt.Fprintf(stderr, "tablotui: %v\n", err)
		usage(fs, stderr)
		return exitUsage
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "tablotui: unexpected argument %q\n", fs.Arg(0))
		usage(fs, stderr)
		return exitUsage
	}
	if *showVersion {
		info, _ := debug.ReadBuildInfo()
		_, _ = fmt.Fprintln(stdout, banner(resolveVersion(version, info)))
		return exitOK
	}
	if st, err := os.Stat(*dir); err != nil || !st.IsDir() {
		_, _ = fmt.Fprintf(stderr, "tablotui: -C %s: not a directory\n", *dir)
		return exitUsage
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The viewer is read once, here: a change of the Git identity shows at the
	// next start. A malformed --as is a usage error whatever else is missing,
	// so the check stands before the one for a source.
	email, err := identity.Email(ctx, *dir, *as)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tablotui: %v\n", err)
		return exitUsage
	}
	path := *settingsPath
	if path == "" {
		// No configuration directory means no choice kept.
		path, _ = settings.DefaultPath()
	} else {
		path = inDir(*dir, path)
	}

	// The source: fixture files, or tablo's library over the snapshot that the
	// live refresh loads once per settled change and the frame stores.
	var src interface {
		source.Tableaux
		source.Lists
		source.Viewers
	}
	var live []ui.Pane
	var force []ui.Command
	// The four actions: from fixture files they have no Repo, so each key
	// says that no action runs.
	ao := act.Options{Dir: *dir, Context: ctx}
	if *fixtures != "" {
		src = source.File{Dir: inDir(*dir, *fixtures)}
	} else {
		at, err := watch.Open(ctx, *dir, *ref)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tablotui: %v\n", err)
			if errors.Is(err, watch.ErrUsage) {
				return exitUsage
			}
			return exitRead
		}
		at.Dir = projectDir(at.Dir)
		w := watch.New(at)
		tab, err := source.NewTablo(ctx, at.Dir, at.Ref, w)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tablotui: %v\n", err)
			return exitRead
		}
		src = tab
		live = append(live, refresh.NewPane(refresh.New(ctx, at, w, time.Now, tab.Load), tab.Store))
		force = append(force, refresh.Command())
		ao = act.Options{
			// Each plan reads HEAD through a fresh action.Tablo, since the
			// Repo names no Source (A2).
			Repo: &action.Repo{Dir: at.Dir},
			// Each record form reads HEAD through a fresh action.Tablo of
			// its own (518e decision 16 a).
			Facts: &formFacts{dir: at.Dir},
			// The project directory the watcher watches (A12).
			Dir: at.Dir,
			// The --ref in view: with one, nothing writes (A12, D4).
			Ref: at.Ref,
			// The person the view is drawn for, as identity.Email names
			// them; the commit stays Git's identity's (A14, D2).
			Viewer: email,
			// The end of the program ends the plans, not a commit (D12).
			Context: ctx,
			// After every commit the watcher loads at once (A12).
			Poke: w.Poke,
		}
	}

	// The one place the program's panes and commands come together: the grid is
	// the home pane, the detail panes follow it, the panes that never open come
	// last, the role pane and then the live refresh, and each task that adds a
	// pane or a command extends this list: the live refresh adds the Force key.
	// The grid has no start: the role pane starts it. The four actions follow.
	g := grid.New(grid.Options{Source: src, Settings: path})
	details := detail.New(detail.Options{Source: src})
	who := role.New(role.Options{Source: src, Viewer: email, Ref: *ref})
	actions := newActions(ao)
	opts := ui.Options{
		Panes:    append(append(append([]ui.Pane{g}, details.Panes...), who), live...),
		Commands: append(append(append(slices.Clone(details.Commands), role.Command()), force...), actions.Commands()...),
		Layout:   details.Layout,
		Styles:   ui.DefaultStyles(),
		Context:  ctx,
	}
	if err := ui.CheckKeys(opts); err != nil {
		_, _ = fmt.Fprintf(stderr, "tablotui: %v\n", err)
		return exitFailure
	}
	err = runProgram(ctx, ui.New(opts))
	// A quit never cuts a commit in two: the command waits for the one in
	// flight before it cancels its context (518e decision 12).
	if actions.Busy() {
		_, _ = fmt.Fprintln(stderr, "tablotui: a commit is in flight; waiting for git")
	}
	actions.Wait()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tablotui: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// formFacts is the Facts of the record form. A form asks Project and then
// Task; Project opens a fresh action.Tablo, which loads HEAD once, and Task
// reads the same one, so each form states HEAD as it stands when the form
// opens (518e decision 16 a).
type formFacts struct {
	dir string
	mu  sync.Mutex
	t   *action.Tablo
}

// Project opens a fresh action.Tablo and reads the project from it.
func (f *formFacts) Project(ctx context.Context) (action.Project, error) {
	t := &action.Tablo{Dir: f.dir}
	f.mu.Lock()
	f.t = t
	f.mu.Unlock()
	return t.Project(ctx)
}

// Task reads the task from the action.Tablo that Project opened last, or
// from a fresh one.
func (f *formFacts) Task(ctx context.Context, id string) (action.Task, error) {
	f.mu.Lock()
	t := f.t
	f.mu.Unlock()
	if t == nil {
		t = &action.Tablo{Dir: f.dir}
	}
	return t.Task(ctx, id)
}

// projectDir returns the directory that holds the project tablo reads: the
// nearest at or above dir that holds .tableaux, below the root of the
// repository, else dir itself. The watcher watches it (watch.Open).
func projectDir(dir string) string {
	for d := dir; ; {
		if st, err := os.Stat(filepath.Join(d, ".tableaux")); err == nil && st.IsDir() {
			return d
		}
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return dir
		}
		up := filepath.Dir(d)
		if up == d {
			return dir
		}
		d = up
	}
}

// inDir resolves a relative path against the project directory, as git -C does.
func inDir(dir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

// checkHyphens refuses a long option written with one hyphen, and names the
// two-hyphen form. The one short option is -C.
func checkHyphens(args []string, fs *flag.FlagSet) error {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return nil
		case strings.HasPrefix(a, "--"):
			name, _, hasValue := strings.Cut(a[2:], "=")
			if f := fs.Lookup(name); f != nil && !hasValue && !isBool(f) {
				i++ // the next argument is this option's value
			}
		case a == "-C":
			i++
		case len(a) > 2 && a[0] == '-' && a[1] != 'C':
			name, _, _ := strings.Cut(a[1:], "=")
			return fmt.Errorf("long option %s needs two hyphens: --%s", a, name)
		}
	}
	return nil
}

func isBool(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

// usage prints each long option with its double hyphen.
func usage(fs *flag.FlagSet, w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage: tablotui [-C <dir>] [--ref <ref>] [--as <email>] [--settings <file>] [--fixture <dir>] [--version] [--help]")
	_, _ = fmt.Fprintln(w, "options:")
	fs.VisitAll(func(f *flag.Flag) {
		name := "--" + f.Name
		if f.Name == "C" {
			name = "-C <dir>"
		} else if !isBool(f) {
			name += " <value>"
		}
		_, _ = fmt.Fprintf(w, "  %s\n    \t%s\n", name, f.Usage)
	})
	_, _ = fmt.Fprintln(w, "  --help\n    \tprint this text and exit")
}

// resolveVersion picks the version to report. A version set by the linker
// wins; otherwise the module version that go install recorded; otherwise
// "dev".
func resolveVersion(linked string, info *debug.BuildInfo) string {
	if linked != "" && linked != "dev" {
		return linked
	}
	if info != nil && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// banner is the one line --version prints.
func banner(v string) string {
	return "tablotui " + v
}
