// Command 0afa demonstrates the detail panes of tablotui: a task list
// beside panes for the task definition, the blockage tree, the current
// person's work queue and the history, which follow the list's selection.
package main

import (
	"embed"
	"flag"
	"fmt"
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
	as := flag.String("as", "ada@example.org", "the current person, whose work queue shows")
	script := flag.String("script", "", "print View() after these keys (space separated) instead of running")
	size := flag.String("size", "100x30", "with -script: WIDTHxHEIGHT")
	strip := flag.Bool("strip-vs16", false, "drop the emoji variation selector from content")
	flag.Parse()

	m, err := NewModel(source(), *as, []string{"ada@example.org", "ben@example.org", "dan@example.org"}, *strip)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *script != "" {
		var w, h int
		if _, err := fmt.Sscanf(*size, "%dx%d", &w, &h); err != nil {
			fmt.Fprintln(os.Stderr, "bad -size:", err)
			os.Exit(2)
		}
		var tm tea.Model = m
		tm, _ = tm.Update(tea.WindowSizeMsg{Width: w, Height: h})
		for _, k := range strings.Fields(*script) {
			tm, _ = tm.Update(keyMsg(k))
		}
		fmt.Println(tm.View())
		return
	}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// keyMsg turns a key name into the message Bubble Tea sends for it.
func keyMsg(k string) tea.KeyMsg {
	switch k {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
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
