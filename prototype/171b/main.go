// Command 171b demonstrates live refresh for tablotui: a status screen that
// reloads when the repository's HEAD or .tableaux changes, and a flag that
// views the project at a ref.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	at := flag.String("at", "", "view the project at this ref and stop refreshing")
	follow := flag.Bool("follow", false, "with --at, follow the ref as it moves")
	dir := flag.String("C", ".", "the repository directory")
	dump := flag.Bool("dump", false, "load once, print the view and exit")
	flag.Parse()
	if err := run(*dir, *at, *follow, *dump); err != nil {
		fmt.Fprintln(os.Stderr, "171b:", err)
		os.Exit(1)
	}
}

func run(dir, at string, follow, dump bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	src, loader, watcher, err := Open(dir, at, follow)
	if err != nil {
		return err
	}
	m := NewModel(ctx, src, loader, watcher, watcher.Snapshot(ctx))
	if dump {
		msg := m.loadCmd(1)()
		next, _ := m.Update(msg)
		fmt.Print(next.(Model).View())
		return nil
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}
