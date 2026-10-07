// Command tablotui is the Tableaux terminal front end.
//
//	tablotui [-C <dir>] [--settings <file>] [--fixture <dir>] [--version] [--help]
//
// It draws the global tableau as a full-screen grid, and beside it the panes
// for the task, the work queue, the blockage tree and the history, which the
// keys 1 to 4 open. This build draws from fixture files; the tablo source
// joins at the integrate gate.
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
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/nbyoung/tablotui/internal/settings"
	"github.com/nbyoung/tablotui/internal/source"
	"github.com/nbyoung/tablotui/internal/ui"
	"github.com/nbyoung/tablotui/internal/ui/detail"
	"github.com/nbyoung/tablotui/internal/ui/grid"
)

// version is the release version. GoReleaser sets it through
// -ldflags "-X main.version=...". A build from source leaves it at "dev".
var version = "dev"

// The exit codes.
const (
	exitOK       = 0
	exitFailure  = 1
	exitUsage    = 2
	noSourceText = "this build has no tablo source; pass --fixture <dir>"
)

// runProgram runs the program until the viewer quits. The tests replace it,
// so that none starts a terminal.
var runProgram = func(ctx context.Context, m tea.Model) error {
	_, err := tea.NewProgram(m, tea.WithContext(ctx)).Run()
	return err
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses the arguments, wires the source, the panes and the frame, and
// returns the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tablotui", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("C", ".", "the project directory")
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
	if *fixtures == "" {
		_, _ = fmt.Fprintf(stderr, "tablotui: %s\n", noSourceText)
		return exitFailure
	}
	path := *settingsPath
	if path == "" {
		// No configuration directory means no choice kept.
		path, _ = settings.DefaultPath()
	} else {
		path = inDir(*dir, path)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The one place the program's panes and commands come together: the grid is
	// the home pane, the detail panes follow it, and each task that adds a pane
	// or a command extends this list.
	src := source.File{Dir: inDir(*dir, *fixtures)}
	g := grid.New(grid.Options{Source: src, Settings: path, Start: grid.GlanceStart()})
	details := detail.New(detail.Options{Source: src})
	opts := ui.Options{
		Panes:    append([]ui.Pane{g}, details.Panes...),
		Commands: details.Commands,
		Layout:   details.Layout,
		Styles:   ui.DefaultStyles(),
		Context:  ctx,
	}
	if err := ui.CheckKeys(opts); err != nil {
		_, _ = fmt.Fprintf(stderr, "tablotui: %v\n", err)
		return exitFailure
	}
	if err := runProgram(ctx, ui.New(opts)); err != nil {
		_, _ = fmt.Fprintf(stderr, "tablotui: %v\n", err)
		return exitFailure
	}
	return exitOK
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
	_, _ = fmt.Fprintln(w, "usage: tablotui [-C <dir>] [--settings <file>] [--fixture <dir>] [--version] [--help]")
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
