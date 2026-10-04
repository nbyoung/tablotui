// Command 679b is the functional prototype of the tablotui tableau grid.
//
//	679b [--settings FILE] [--strip-vs16] [--dump [--size WxH] [--keys k,k,...] [--chrome]] FILE [FILE]
//
// The first file is the opening window; w cycles through the rest.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// keyMsg turns a key name such as "down" or "1" into a Bubble Tea key message.
func keyMsg(k string) tea.KeyMsg {
	named := map[string]tea.KeyType{
		"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft,
		"right": tea.KeyRight, "enter": tea.KeyEnter, "pgup": tea.KeyPgUp,
		"pgdown": tea.KeyPgDown, "home": tea.KeyHome, "end": tea.KeyEnd,
		"space": tea.KeySpace,
	}
	if t, ok := named[k]; ok {
		return tea.KeyMsg{Type: t}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

// checkHyphens refuses a long option written with one hyphen.
func checkHyphens(args []string) error {
	for _, a := range args {
		if a == "--" {
			return nil
		}
		if len(a) > 2 && a[0] == '-' && a[1] != '-' {
			name, _, _ := strings.Cut(a[1:], "=")
			return fmt.Errorf("long option %s needs two hyphens: --%s", a, name)
		}
	}
	return nil
}

// usage prints each long option with its double hyphen.
func usage(fs *flag.FlagSet, w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage: 679b [options] FILE [FILE]")
	_, _ = fmt.Fprintln(w, "options:")
	fs.VisitAll(func(f *flag.Flag) {
		def := ""
		if f.DefValue != "" && f.DefValue != "false" {
			def = fmt.Sprintf(" (default %q)", f.DefValue)
		}
		_, _ = fmt.Fprintf(w, "  --%s\n    \t%s%s\n", f.Name, f.Usage, def)
	})
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("679b", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { usage(fs, stderr) }
	settings := fs.String("settings", "", "settings file for the hidden columns (empty keeps none)")
	stripVS := fs.Bool("strip-vs16", false, "remove variation selectors from symbols")
	dump := fs.Bool("dump", false, "print the view after the keys and exit")
	size := fs.String("size", "100x14", "window size WxH for --dump")
	keys := fs.String("keys", "", "comma-separated keys to send before --dump")
	chrome := fs.Bool("chrome", false, "include the chrome in --dump")
	if err := checkHyphens(args); err != nil {
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
	if fs.NArg() == 0 {
		fs.Usage()
		return 2
	}
	var variants []Data
	for _, p := range fs.Args() {
		d, err := LoadData(p)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		variants = append(variants, d)
	}
	m := NewModel(variants, *settings)
	m.StripVS16 = *stripVS
	if !*dump {
		if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	var w, h int
	if _, err := fmt.Sscanf(*size, "%dx%d", &w, &h); err != nil {
		_, _ = fmt.Fprintf(stderr, "size %q: %v\n", *size, err)
		return 2
	}
	var tm tea.Model = m
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: w, Height: h})
	for _, k := range strings.Split(*keys, ",") {
		if k == "comma" {
			k = ","
		}
		if k != "" {
			tm, _ = tm.Update(keyMsg(k))
		}
	}
	if *chrome {
		_, _ = fmt.Fprintln(stdout, tm.View())
	} else {
		_, _ = fmt.Fprintln(stdout, tm.(Model).Data())
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
