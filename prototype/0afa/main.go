// Command 0afa demonstrates the detail panes of tablotui: a task list
// beside panes for the task definition, the blockage tree, the current
// person's work queue and the history, which follow the list's selection.
package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

//go:embed testdata
var testdata embed.FS

func source() Source {
	sub, err := fs.Sub(testdata, "testdata")
	if err != nil {
		panic(err)
	}
	return FSSource{FS: sub}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// checkArgs refuses a long option written with one hyphen, up to a bare "--".
func checkArgs(args []string) error {
	for _, a := range args {
		if a == "--" {
			return nil
		}
		if len(a) > 2 && a[0] == '-' && a[1] != '-' {
			name, _, _ := strings.Cut(a[1:], "=")
			return fmt.Errorf("unknown option %s: long options take two hyphens, as --%s", a, name)
		}
	}
	return nil
}

func newFlags(stderr io.Writer) (*flag.FlagSet, *options) {
	o := &options{}
	fs := flag.NewFlagSet("0afa", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.as, "as", "ada@example.org", "the current person, whose work queue shows")
	fs.BoolVar(&o.dump, "dump", false, "print the view after the keys and exit")
	fs.StringVar(&o.keys, "keys", "", "with --dump: keys to send first, comma separated (comma is the comma key)")
	fs.StringVar(&o.size, "size", "100x30", "with --dump: the window size, WIDTHxHEIGHT")
	fs.BoolVar(&o.chrome, "chrome", false, "with --dump: print exactly what the interface draws, with borders, cursor, scroll marks, key help and padding")
	fs.BoolVar(&o.strip, "strip-vs16", false, "drop the emoji variation selector from content")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "usage: 0afa [options]")
		fs.VisitAll(func(f *flag.Flag) {
			def := ""
			if f.DefValue != "" && f.DefValue != "false" {
				def = fmt.Sprintf(" (default %q)", f.DefValue)
			}
			_, _ = fmt.Fprintf(stderr, "  --%s%s\n    \t%s\n", f.Name, def, f.Usage)
		})
	}
	return fs, o
}

type options struct {
	as, keys, size      string
	dump, chrome, strip bool
}

func run(args []string, stdout, stderr io.Writer) int {
	fs, o := newFlags(stderr)
	if err := checkArgs(args); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		fs.Usage()
		return 2
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	m, err := NewModel(source(), o.as, []string{"ada@example.org", "ben@example.org", "dan@example.org"}, o.strip)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if o.dump {
		var w, h int
		if _, err := fmt.Sscanf(o.size, "%dx%d", &w, &h); err != nil {
			_, _ = fmt.Fprintln(stderr, "bad --size:", err)
			return 2
		}
		var tm tea.Model = m
		tm, _ = tm.Update(tea.WindowSizeMsg{Width: w, Height: h})
		for _, k := range strings.Split(o.keys, ",") {
			if k == "" {
				continue
			}
			tm, _ = tm.Update(keyMsg(k))
		}
		if o.chrome {
			_, _ = fmt.Fprintln(stdout, tm.View())
		} else {
			_, _ = fmt.Fprintln(stdout, tm.(Model).Dump())
		}
		return 0
	}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// keyMsg turns a key name into the message Bubble Tea sends for it.
func keyMsg(k string) tea.KeyMsg {
	switch k {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "comma":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(",")}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "home":
		return tea.KeyMsg{Type: tea.KeyHome}
	case "end":
		return tea.KeyMsg{Type: tea.KeyEnd}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}
