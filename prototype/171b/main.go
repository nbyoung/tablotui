// Command 171b demonstrates live refresh for tablotui: a status screen that
// reloads when the repository's HEAD or .tableaux changes, and a flag that
// views the project at a ref.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() { os.Exit(realMain(os.Args[1:], os.Stdout, os.Stderr)) }

// checkArgs refuses a single-hyphen long option such as -dump, up to a bare --.
func checkArgs(args []string) error {
	for _, a := range args {
		if a == "--" {
			return nil
		}
		if len(a) > 2 && a[0] == '-' && a[1] != '-' {
			name := strings.SplitN(a[1:], "=", 2)[0]
			return fmt.Errorf("unknown option %s: long options take two hyphens, as --%s", a, name)
		}
	}
	return nil
}

const usageText = `Usage: 171b [options]

Options:
  -C string    the repository directory (default ".")
  --at string  view the project at this ref and stop refreshing
  --follow     with --at, follow the ref as it moves (default false)
  --dump       load once, print the project data and exit (default false)
  --chrome     with --dump, print what View returns, chrome included (default false)
`

func realMain(args []string, stdout, stderr io.Writer) int {
	if err := checkArgs(args); err != nil {
		_, _ = fmt.Fprintln(stderr, "171b:", err)
		return 2
	}
	fs := flag.NewFlagSet("171b", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { _, _ = fmt.Fprint(stderr, usageText) }
	at := fs.String("at", "", "")
	follow := fs.Bool("follow", false, "")
	dir := fs.String("C", ".", "")
	dump := fs.Bool("dump", false, "")
	chrome := fs.Bool("chrome", false, "")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if err := run(*dir, *at, *follow, *dump, *chrome, stdout); err != nil {
		_, _ = fmt.Fprintln(stderr, "171b:", err)
		return 1
	}
	return 0
}

func run(dir, at string, follow, dump, chrome bool, out io.Writer) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	src, loader, watcher, err := Open(dir, at, follow)
	if err != nil {
		return err
	}
	m := NewModel(ctx, src, loader, watcher, watcher.Snapshot(ctx))
	if dump {
		next, _ := m.Update(m.loadCmd(1)())
		if chrome {
			_, err = fmt.Fprint(out, next.(Model).View())
		} else {
			_, err = fmt.Fprint(out, next.(Model).Data())
		}
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}
