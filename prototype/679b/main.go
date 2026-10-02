// Command 679b is the functional prototype of the tablotui tableau grid.
//
//	679b [-settings FILE] [-strip-vs16] [-dump -size WxH -keys k,k,...] VIEW.json...
//
// The first file is the opening window; w cycles through the rest.
package main

import (
	"errors"
	"flag"
	"fmt"
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

func run() error {
	settings := flag.String("settings", "", "settings file for the hidden columns (empty keeps none)")
	stripVS := flag.Bool("strip-vs16", false, "remove variation selectors from symbols")
	dump := flag.Bool("dump", false, "print the view after the keys and exit")
	size := flag.String("size", "100x14", "window size WxH for -dump")
	keys := flag.String("keys", "", "comma-separated keys to send before -dump")
	flag.Parse()
	if flag.NArg() == 0 {
		return errors.New("usage: 679b [flags] FILE [FILE]")
	}
	var variants []Data
	for _, p := range flag.Args() {
		d, err := LoadData(p)
		if err != nil {
			return err
		}
		variants = append(variants, d)
	}
	m := NewModel(variants, *settings)
	m.StripVS16 = *stripVS
	if !*dump {
		_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
		return err
	}
	var w, h int
	if _, err := fmt.Sscanf(*size, "%dx%d", &w, &h); err != nil {
		return fmt.Errorf("size %q: %w", *size, err)
	}
	var tm tea.Model = m
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: w, Height: h})
	for _, k := range strings.Split(*keys, ",") {
		if k != "" {
			tm, _ = tm.Update(keyMsg(k))
		}
	}
	fmt.Println(tm.View())
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
